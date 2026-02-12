// Command voiceprint extracts speaker embeddings from audio files using
// the ERes2Net model, supporting both ncnn and ONNX Runtime engines.
//
// TODO(cl/go/giztoy-cli): This command will be rewritten as a subcommand
// of the unified `giztoy` CLI. The current standalone binary is temporary.
// When rewriting, preserve: -engine flag, -model flag, embedding output format,
// and the batch comparison mode.
//
// Modes:
//
//  1. Single file — extract embedding and write to stdout/file:
//     voiceprint -engine=ncnn input.ogg
//     voiceprint -engine=onnx -model=path/to/model.onnx input.ogg
//     voiceprint -engine=ncnn -output=emb.bin input.ogg
//
//  2. Batch comparison — analyze a directory of OGG files:
//     voiceprint -batch <dir>
//
// The speaker name is extracted from filenames like:
//
//	2026_02_08_19_00_38_天尼_8b62a440.ogg
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/ogg"
	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/audio/fbank"
	"github.com/haivivi/giztoy/go/pkg/audio/resampler"
	"github.com/haivivi/giztoy/go/pkg/ncnn"
	ortpkg "github.com/haivivi/giztoy/go/pkg/onnx"
	"github.com/haivivi/giztoy/go/pkg/voiceprint"
)

// engine abstracts ncnn and ONNX Runtime inference.
type engine interface {
	// Extract computes a speaker embedding from fbank features.
	// features is [T][80] mel filterbank.
	Extract(features [][]float32) ([]float32, error)
	Close() error
}

// --------------------------------------------------------------------------
// ncnn engine
// --------------------------------------------------------------------------

type ncnnEngine struct {
	net *ncnn.Net
}

func newNCNNEngine() (*ncnnEngine, error) {
	net, err := ncnn.LoadModel(ncnn.ModelSpeakerERes2Net)
	if err != nil {
		return nil, err
	}
	return &ncnnEngine{net: net}, nil
}

func (e *ncnnEngine) Extract(features [][]float32) ([]float32, error) {
	return extractEmbedding(e.net, features)
}

func (e *ncnnEngine) Close() error {
	return e.net.Close()
}

// --------------------------------------------------------------------------
// ONNX Runtime engine
// --------------------------------------------------------------------------

type onnxEngine struct {
	env     *ortpkg.Env
	session *ortpkg.Session
}

func newONNXEngine(modelPath string) (*onnxEngine, error) {
	data, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("read onnx model: %w", err)
	}
	env, err := ortpkg.NewEnv("voiceprint")
	if err != nil {
		return nil, err
	}
	session, err := env.NewSession(data)
	if err != nil {
		env.Close()
		return nil, err
	}
	return &onnxEngine{env: env, session: session}, nil
}

func (e *onnxEngine) Extract(features [][]float32) ([]float32, error) {
	flat := fbank.Flatten(features)
	T := len(features)

	// ONNX ERes2Net input: [1, T, 80]
	input, err := ortpkg.NewTensor([]int64{1, int64(T), 80}, flat)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	outputs, err := e.session.Run(
		[]string{"x"}, []*ortpkg.Tensor{input},
		[]string{"embedding"},
	)
	if err != nil {
		return nil, err
	}
	defer outputs[0].Close()

	emb, err := outputs[0].FloatData()
	if err != nil {
		return nil, err
	}

	l2Normalize(emb)
	return emb, nil
}

func (e *onnxEngine) Close() error {
	e.session.Close()
	e.env.Close()
	return nil
}

// --------------------------------------------------------------------------
// CLI
// --------------------------------------------------------------------------

type sample struct {
	path    string
	speaker string
	emb     []float32
	hash    string
}

func main() {
	engineFlag := flag.String("engine", "ncnn", "inference engine: ncnn or onnx")
	modelFlag := flag.String("model", "", "ONNX model path (required when engine=onnx)")
	outputFlag := flag.String("output", "", "output embedding to file (binary float32)")
	batchFlag := flag.Bool("batch", false, "batch mode: analyze directory of OGG files")
	denoiseFlag := flag.Bool("denoise", false, "apply spectral subtraction denoise before embedding extraction")
	hashBitsFlag := flag.Int("hash-bits", 20, "LSH hash bits (4-64, must be multiple of 4)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: voiceprint [flags] <file-or-dir>\n")
		fmt.Fprintf(os.Stderr, "\nflags:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}
	target := flag.Arg(0)

	// Create engine
	var eng engine
	var err error
	switch *engineFlag {
	case "ncnn":
		eng, err = newNCNNEngine()
	case "onnx":
		if *modelFlag == "" {
			fmt.Fprintf(os.Stderr, "error: -model is required when engine=onnx\n")
			os.Exit(1)
		}
		eng, err = newONNXEngine(*modelFlag)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown engine %q (use ncnn or onnx)\n", *engineFlag)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "load engine: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()
	fmt.Fprintf(os.Stderr, "engine: %s\n", *engineFlag)

	// Create fbank extractor
	fbankExt := fbank.New(fbank.DefaultConfig())

	if *denoiseFlag {
		fmt.Fprintf(os.Stderr, "denoise: enabled (spectral subtraction)\n")
	}

	// Hasher dimension is determined by the engine's embedding output.
	// ncnn ERes2Net = 512, but ONNX models may differ.
	embDim := 512 // default; overridden after first extraction if needed
	if *engineFlag == "ncnn" {
		embDim = 512
	}
	hasher := voiceprint.NewHasher(embDim, *hashBitsFlag, 42)

	if *batchFlag {
		runBatch(eng, fbankExt, hasher, *denoiseFlag, target)
	} else {
		runSingle(eng, fbankExt, hasher, *denoiseFlag, target, *outputFlag)
	}
}

// runSingle processes a single audio file and outputs the embedding.
func runSingle(eng engine, fbankExt *fbank.Extractor, hasher *voiceprint.Hasher, denoise bool, audioPath, outputPath string) {
	pcm16k, err := decodeOGGTo16kMono(audioPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	if denoise {
		pcm16k = spectralDenoise(pcm16k)
	}
	pcm16k = trimSilence(pcm16k, 300)
	features := fbankExt.ExtractFromInt16(pcm16k)
	if len(features) < 30 {
		fmt.Fprintf(os.Stderr, "audio too short: %d frames\n", len(features))
		os.Exit(1)
	}

	emb, err := eng.Extract(features)
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract: %v\n", err)
		os.Exit(1)
	}

	hash := hasher.Hash(emb)
	fmt.Fprintf(os.Stderr, "hash: %s (%s)\n", hash, voiceprint.VoiceLabel(hash))

	if outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create output: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := binary.Write(f, binary.LittleEndian, emb); err != nil {
			fmt.Fprintf(os.Stderr, "write output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote %d-dim embedding to %s\n", len(emb), outputPath)
	} else {
		fmt.Println(hash)
	}
}

// runBatch processes a directory of OGG files and outputs a similarity matrix.
func runBatch(eng engine, fbankExt *fbank.Extractor, hasher *voiceprint.Hasher, denoise bool, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dir: %v\n", err)
		os.Exit(1)
	}

	var oggFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".ogg") {
			oggFiles = append(oggFiles, filepath.Join(dir, e.Name()))
		}
	}
	if len(oggFiles) == 0 {
		fmt.Fprintf(os.Stderr, "no OGG files found in %s\n", dir)
		os.Exit(1)
	}
	fmt.Printf("found %d OGG files\n", len(oggFiles))

	var samples []sample
	for _, path := range oggFiles {
		speaker := parseSpeaker(filepath.Base(path))
		if speaker == "" {
			continue
		}

		pcm16k, err := decodeOGGTo16kMono(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", filepath.Base(path), err)
			continue
		}

		if denoise {
			pcm16k = spectralDenoise(pcm16k)
		}
		pcm16k = trimSilence(pcm16k, 300)
		features := fbankExt.ExtractFromInt16(pcm16k)
		if len(features) < 30 {
			fmt.Fprintf(os.Stderr, "  skip %s: too short (%d frames)\n", filepath.Base(path), len(features))
			continue
		}

		emb, err := eng.Extract(features)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", filepath.Base(path), err)
			continue
		}

		hash := hasher.Hash(emb)
		samples = append(samples, sample{
			path:    filepath.Base(path),
			speaker: speaker,
			emb:     emb,
			hash:    hash,
		})
		fmt.Printf("  [%s] %s — hash=%s\n", speaker, filepath.Base(path), hash)
	}

	if len(samples) < 2 {
		fmt.Fprintf(os.Stderr, "need at least 2 samples\n")
		os.Exit(1)
	}

	sort.Slice(samples, func(i, j int) bool {
		if samples[i].speaker != samples[j].speaker {
			return samples[i].speaker < samples[j].speaker
		}
		return samples[i].path < samples[j].path
	})

	// Similarity matrix
	fmt.Printf("\n=== Cosine Similarity Matrix (%d samples) ===\n\n", len(samples))
	fmt.Printf("%20s", "")
	for i := range samples {
		fmt.Printf(" %4d", i)
	}
	fmt.Println()
	for i, si := range samples {
		label := fmt.Sprintf("[%d] %s", i, si.speaker)
		if len(label) > 20 {
			label = label[:20]
		}
		fmt.Printf("%20s", label)
		for j, sj := range samples {
			if i == j {
				fmt.Printf(" %4s", "----")
			} else {
				fmt.Printf(" %4.2f", cosineSimilarity(si.emb, sj.emb))
			}
		}
		fmt.Println()
	}

	fmt.Println()
	printSpeakerAnalysis(samples)
	fmt.Println()
	printHashAnalysis(samples, hasher.Bits())
}

// --------------------------------------------------------------------------
// Audio processing
// --------------------------------------------------------------------------

func parseSpeaker(name string) string {
	name = strings.TrimSuffix(name, ".ogg")
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2]
}

func decodeOGGTo16kMono(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec, err := opus.NewDecoder(48000, 1)
	if err != nil {
		return nil, fmt.Errorf("opus decoder: %w", err)
	}
	defer dec.Close()

	var pcm48k bytes.Buffer
	for pkt, err := range ogg.ReadOpusPackets(f) {
		if err != nil {
			return nil, fmt.Errorf("read opus: %w", err)
		}
		pcmData, err := dec.Decode(pkt.Frame)
		if err != nil {
			continue
		}
		pcm48k.Write(pcmData)
	}

	if pcm48k.Len() == 0 {
		return nil, fmt.Errorf("no audio decoded")
	}

	rs, err := resampler.New(
		&pcm48k,
		resampler.Format{SampleRate: 48000, Stereo: false},
		resampler.Format{SampleRate: 16000, Stereo: false},
	)
	if err != nil {
		return nil, fmt.Errorf("resampler: %w", err)
	}
	defer rs.Close()

	var out bytes.Buffer
	if _, err := io.Copy(&out, rs); err != nil {
		return nil, fmt.Errorf("resample: %w", err)
	}
	return out.Bytes(), nil
}

// --------------------------------------------------------------------------
// Embedding extraction (ncnn-specific, used by ncnnEngine)
// --------------------------------------------------------------------------

func extractEmbedding(net *ncnn.Net, features [][]float32) ([]float32, error) {
	const segFrames = 300
	const hopFrames = 150

	if len(features) <= segFrames {
		return extractSegment(net, features)
	}

	var embeddings [][]float32
	var lastLoopStart int
	for start := 0; start+segFrames <= len(features); start += hopFrames {
		seg := features[start : start+segFrames]
		emb, err := extractSegment(net, seg)
		if err != nil {
			continue
		}
		embeddings = append(embeddings, emb)
		lastLoopStart = start
	}
	if lastStart := len(features) - segFrames; lastStart > lastLoopStart {
		seg := features[lastStart:]
		emb, err := extractSegment(net, seg)
		if err == nil {
			embeddings = append(embeddings, emb)
		}
	}

	if len(embeddings) == 0 {
		return extractSegment(net, features)
	}

	dim := len(embeddings[0])
	avg := make([]float32, dim)
	for _, emb := range embeddings {
		for i, v := range emb {
			avg[i] += v
		}
	}
	n := float32(len(embeddings))
	for i := range avg {
		avg[i] /= n
	}
	l2Normalize(avg)
	return avg, nil
}

func extractSegment(net *ncnn.Net, features [][]float32) ([]float32, error) {
	flat := fbank.Flatten(features)
	numFrames := len(features)
	numMels := len(features[0])

	input, err := ncnn.NewMat2D(numMels, numFrames, flat)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	ex, err := net.NewExtractor()
	if err != nil {
		return nil, err
	}
	defer ex.Close()

	if err := ex.SetInput("in0", input); err != nil {
		return nil, err
	}

	output, err := ex.Extract("out0")
	if err != nil {
		return nil, err
	}
	defer output.Close()

	emb := output.FloatData()
	if len(emb) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}

	l2Normalize(emb)
	return emb, nil
}

// --------------------------------------------------------------------------
// Math utilities
// --------------------------------------------------------------------------

func l2Normalize(v []float32) {
	norm := float32(0)
	for _, x := range v {
		norm += x * x
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range v {
			v[i] /= norm
		}
	}
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	dot := float64(0)
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot
}

func trimSilence(pcm []byte, threshold int16) []byte {
	const frameBytes = 640
	numFrames := len(pcm) / frameBytes
	if numFrames < 3 {
		return pcm
	}

	rms := func(start int) float64 {
		sum := float64(0)
		for i := 0; i < 320; i++ {
			off := start + i*2
			if off+1 >= len(pcm) {
				break
			}
			s := int16(pcm[off]) | int16(pcm[off+1])<<8
			sum += float64(s) * float64(s)
		}
		return math.Sqrt(sum / 320)
	}

	thresh := float64(threshold)
	first := 0
	for f := 0; f < numFrames; f++ {
		if rms(f*frameBytes) > thresh {
			first = f
			break
		}
	}
	last := numFrames - 1
	for f := numFrames - 1; f >= first; f-- {
		if rms(f*frameBytes) > thresh {
			last = f
			break
		}
	}
	if first > 0 {
		first--
	}
	if last < numFrames-1 {
		last++
	}
	startByte := first * frameBytes
	endByte := (last + 1) * frameBytes
	if endByte > len(pcm) {
		endByte = len(pcm)
	}
	return pcm[startByte:endByte]
}

// --------------------------------------------------------------------------
// Analysis
// --------------------------------------------------------------------------

func printHashAnalysis(samples []sample, maxBits int) {
	fmt.Printf("=== Hash Recall Analysis (%d-bit max) ===\n\n", maxBits)
	speakers := map[string][]sample{}
	for _, s := range samples {
		speakers[s.speaker] = append(speakers[s.speaker], s)
	}
	speakerNames := make([]string, 0, len(speakers))
	for name := range speakers {
		speakerNames = append(speakerNames, name)
	}
	sort.Strings(speakerNames)

	fmt.Printf("%-6s %5s %7s %10s %10s %10s\n", "bits", "chars", "unique", "recall", "precision", "f1")
	for bits := 4; bits <= maxBits; bits += 4 {
		nc := bits / 4
		// recall = same-speaker pairs sharing hash / total same-speaker pairs
		sameTotal, sameMatch := 0, 0
		for _, samps := range speakers {
			for i := 0; i < len(samps); i++ {
				for j := i + 1; j < len(samps); j++ {
					sameTotal++
					if samps[i].hash[:nc] == samps[j].hash[:nc] {
						sameMatch++
					}
				}
			}
		}
		// precision = same-speaker pairs in hash group / all pairs in hash group
		groups := map[string]map[string]int{}
		for _, s := range samples {
			h := s.hash[:nc]
			if groups[h] == nil {
				groups[h] = map[string]int{}
			}
			groups[h][s.speaker]++
		}
		correctPairs, totalPairs := 0, 0
		for _, g := range groups {
			total := 0
			for _, c := range g {
				total += c
			}
			if total < 2 {
				continue
			}
			totalPairs += total * (total - 1) / 2
			for _, c := range g {
				correctPairs += c * (c - 1) / 2
			}
		}
		unique := len(groups)
		recall := float64(0)
		if sameTotal > 0 {
			recall = float64(sameMatch) / float64(sameTotal)
		}
		precision := float64(0)
		if totalPairs > 0 {
			precision = float64(correctPairs) / float64(totalPairs)
		}
		f1 := float64(0)
		if precision+recall > 0 {
			f1 = 2 * precision * recall / (precision + recall)
		}
		fmt.Printf("%-6d %5d %7d %9.1f%% %9.1f%% %9.1f%%\n",
			bits, nc, unique, recall*100, precision*100, f1*100)
	}

	// Per-speaker hash table
	fmt.Printf("\n=== Voice Hashes (%d-bit) ===\n\n", maxBits)
	for _, name := range speakerNames {
		hashes := map[string]int{}
		for _, s := range speakers[name] {
			nc := maxBits / 4
			hashes[s.hash[:nc]]++
		}
		fmt.Printf("  %-8s", name)
		for h, c := range hashes {
			if c > 1 {
				fmt.Printf(" %s(%d)", h, c)
			} else {
				fmt.Printf(" %s", h)
			}
		}
		fmt.Println()
	}
}

func printSpeakerAnalysis(samples []sample) {
	speakers := map[string][]int{}
	for i, s := range samples {
		speakers[s.speaker] = append(speakers[s.speaker], i)
	}

	var intraSims []float64
	for sp, idxs := range speakers {
		if len(idxs) < 2 {
			continue
		}
		for i := 0; i < len(idxs); i++ {
			for j := i + 1; j < len(idxs); j++ {
				sim := cosineSimilarity(samples[idxs[i]].emb, samples[idxs[j]].emb)
				intraSims = append(intraSims, sim)
				fmt.Printf("  same [%s]: %.4f  (%s vs %s)\n", sp, sim,
					samples[idxs[i]].path, samples[idxs[j]].path)
			}
		}
	}

	var interSims []float64
	speakerNames := make([]string, 0, len(speakers))
	for sp := range speakers {
		speakerNames = append(speakerNames, sp)
	}
	sort.Strings(speakerNames)
	for i := 0; i < len(speakerNames); i++ {
		for j := i + 1; j < len(speakerNames); j++ {
			for _, ii := range speakers[speakerNames[i]] {
				for _, jj := range speakers[speakerNames[j]] {
					sim := cosineSimilarity(samples[ii].emb, samples[jj].emb)
					interSims = append(interSims, sim)
				}
			}
		}
	}

	fmt.Println()
	if len(intraSims) > 0 {
		avg, min, max := stats(intraSims)
		fmt.Printf("SAME speaker:  avg=%.4f  min=%.4f  max=%.4f  (n=%d)\n", avg, min, max, len(intraSims))
	}
	if len(interSims) > 0 {
		avg, min, max := stats(interSims)
		fmt.Printf("DIFF speaker:  avg=%.4f  min=%.4f  max=%.4f  (n=%d)\n", avg, min, max, len(interSims))
	}
	if len(intraSims) > 0 && len(interSims) > 0 {
		intraAvg, _, _ := stats(intraSims)
		interAvg, _, _ := stats(interSims)
		gap := intraAvg - interAvg
		fmt.Printf("GAP (intra-inter): %.4f\n", gap)
		if gap > 0.3 {
			fmt.Println("-> Model discriminates well between speakers!")
		} else if gap > 0.1 {
			fmt.Println("-> Model shows some speaker discrimination")
		} else {
			fmt.Println("-> Model has weak speaker discrimination")
		}
	}
}

func stats(vals []float64) (avg, min, max float64) {
	if len(vals) == 0 {
		return
	}
	min, max = vals[0], vals[0]
	sum := 0.0
	for _, v := range vals {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	avg = sum / float64(len(vals))
	return
}
