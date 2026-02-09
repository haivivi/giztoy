// Command voiceprint analyzes voice samples using the ERes2Net speaker
// embedding model. It decodes OGG/Opus files, extracts mel filterbank
// features, and computes speaker embeddings via ncnn inference.
//
// Usage:
//
//	voiceprint <dir>
//
// The directory should contain OGG files named like:
//
//	2026_02_08_19_00_38_天尼_8b62a440.ogg
//
// The speaker name is extracted from the filename (field before the last _).
// The tool outputs a cosine similarity matrix showing how similar each
// pair of voice samples is, grouped by speaker.
package main

import (
	"bytes"
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
)

type sample struct {
	path    string
	speaker string
	emb     []float32
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: voiceprint <dir>\n")
		os.Exit(1)
	}
	dir := os.Args[1]

	// Find all OGG files
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

	// Load ERes2Net model
	net, err := ncnn.LoadModel(ncnn.ModelSpeakerERes2Net)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load model: %v\n", err)
		os.Exit(1)
	}
	defer net.Close()
	fmt.Println("loaded ERes2Net speaker model")

	// Create fbank extractor (16kHz, 80 mels)
	fbankExt := fbank.New(fbank.DefaultConfig())

	// Process each file
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

		// Trim silence from PCM before feature extraction
		pcm16k = trimSilence(pcm16k, 300) // threshold: ~300/32768 ≈ -40dB

		// fbank features
		features := fbankExt.ExtractFromInt16(pcm16k)
		if len(features) < 30 { // ~0.3s minimum
			fmt.Fprintf(os.Stderr, "  skip %s: too short after VAD (%d frames)\n", filepath.Base(path), len(features))
			continue
		}

		// Run ERes2Net inference
		emb, err := extractEmbedding(net, features)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: inference error: %v\n", filepath.Base(path), err)
			continue
		}

		samples = append(samples, sample{
			path:    filepath.Base(path),
			speaker: speaker,
			emb:     emb,
		})
		fmt.Printf("  [%s] %s — %d frames, emb dim=%d\n", speaker, filepath.Base(path), len(features), len(emb))
	}

	if len(samples) < 2 {
		fmt.Fprintf(os.Stderr, "need at least 2 samples for comparison\n")
		os.Exit(1)
	}

	// Sort by speaker name
	sort.Slice(samples, func(i, j int) bool {
		if samples[i].speaker != samples[j].speaker {
			return samples[i].speaker < samples[j].speaker
		}
		return samples[i].path < samples[j].path
	})

	// Print similarity matrix
	fmt.Printf("\n=== Cosine Similarity Matrix (%d samples) ===\n\n", len(samples))

	// Header
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
			sim := cosineSimilarity(si.emb, sj.emb)
			if i == j {
				fmt.Printf(" %4s", "----")
			} else {
				fmt.Printf(" %4.2f", sim)
			}
		}
		fmt.Println()
	}

	// Print per-speaker analysis
	fmt.Println()
	printSpeakerAnalysis(samples)
}

// parseSpeaker extracts speaker name from filename like:
// "2026_02_08_19_00_38_天尼_8b62a440.ogg" -> "天尼"
func parseSpeaker(name string) string {
	name = strings.TrimSuffix(name, ".ogg")
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return ""
	}
	// Speaker name is the second-to-last part
	return parts[len(parts)-2]
}

// decodeOGGTo16kMono reads an OGG/Opus file and returns 16kHz mono PCM (int16 LE bytes).
func decodeOGGTo16kMono(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Decode Opus packets to 48kHz mono PCM
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
			continue // skip bad frames
		}
		pcm48k.Write(pcmData)
	}

	if pcm48k.Len() == 0 {
		return nil, fmt.Errorf("no audio decoded")
	}

	// Resample 48kHz -> 16kHz mono
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

// extractEmbedding runs ERes2Net inference on fbank features.
// features is [T][80] mel filterbank. Returns [512] speaker embedding.
//
// For long audio (> segFrames), the features are split into overlapping
// segments, each segment produces an embedding, and the final embedding
// is the L2-normalized average. This significantly improves robustness.
func extractEmbedding(net *ncnn.Net, features [][]float32) ([]float32, error) {
	const segFrames = 300 // ~3 seconds
	const hopFrames = 150 // 50% overlap

	if len(features) <= segFrames {
		return extractSegment(net, features)
	}

	// Multi-segment averaging
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
	// Include the last segment only if it wasn't already covered by the loop
	if lastStart := len(features) - segFrames; lastStart > lastLoopStart {
		seg := features[lastStart:]
		emb, err := extractSegment(net, seg)
		if err == nil {
			embeddings = append(embeddings, emb)
		}
	}

	if len(embeddings) == 0 {
		return extractSegment(net, features) // fallback to full
	}

	// Average and L2-normalize
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

// extractSegment runs ERes2Net on a single feature segment.
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

// cosineSimilarity computes the cosine similarity between two vectors.
// Assumes vectors are already L2-normalized, so it's just the dot product.
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

// trimSilence removes leading and trailing silence from int16 PCM bytes.
// It uses a simple energy-based VAD: scans for the first/last frame where
// the RMS energy exceeds the threshold.
// frameSize is 320 samples (20ms at 16kHz).
func trimSilence(pcm []byte, threshold int16) []byte {
	const frameBytes = 640 // 320 samples * 2 bytes
	numFrames := len(pcm) / frameBytes
	if numFrames < 3 {
		return pcm
	}

	// Compute RMS energy per frame
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

	// Find first active frame
	first := 0
	for f := 0; f < numFrames; f++ {
		if rms(f*frameBytes) > thresh {
			first = f
			break
		}
	}

	// Find last active frame
	last := numFrames - 1
	for f := numFrames - 1; f >= first; f-- {
		if rms(f*frameBytes) > thresh {
			last = f
			break
		}
	}

	// Add 1 frame padding on each side
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

func printSpeakerAnalysis(samples []sample) {
	// Group by speaker
	type pair struct {
		i, j int
		sim  float64
	}
	speakers := map[string][]int{}
	for i, s := range samples {
		speakers[s.speaker] = append(speakers[s.speaker], i)
	}

	// Intra-speaker similarity (same speaker)
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

	// Inter-speaker similarity (different speakers)
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
