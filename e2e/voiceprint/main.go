// Command e2e-voiceprint runs end-to-end voiceprint verification tests.
//
// It scans a directory of OGG files, runs the voiceprint CLI on each file
// to extract embeddings, then computes a similarity matrix and reports
// same-speaker vs different-speaker statistics.
//
// Usage:
//
//	bazel run //e2e/voiceprint -- -dir=/path/to/audio -voiceprint=$(location //go/cmd/voiceprint)
//	go run ./e2e/voiceprint -dir=~/Vibing/cursorcat/chat_history
//
// TODO(cl/go/giztoy-cli): When voiceprint becomes a giztoy subcommand,
// update -voiceprint flag to use `giztoy voiceprint` instead.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type sample struct {
	path    string
	speaker string
	emb     []float32
}

func main() {
	dirFlag := flag.String("dir", "", "directory containing OGG audio files")
	vpFlag := flag.String("voiceprint", "voiceprint", "path to voiceprint binary")
	engineFlag := flag.String("engine", "ncnn", "inference engine: ncnn or onnx")
	modelFlag := flag.String("model", "", "ONNX model path (for engine=onnx)")
	flag.Parse()

	if *dirFlag == "" {
		fmt.Fprintf(os.Stderr, "usage: e2e-voiceprint -dir=<path> [-engine=ncnn|onnx] [-model=path]\n")
		os.Exit(1)
	}

	// Find OGG files
	entries, err := os.ReadDir(*dirFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dir: %v\n", err)
		os.Exit(1)
	}

	var oggFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".ogg") {
			oggFiles = append(oggFiles, filepath.Join(*dirFlag, e.Name()))
		}
	}
	if len(oggFiles) == 0 {
		fmt.Fprintf(os.Stderr, "no OGG files found in %s\n", *dirFlag)
		os.Exit(1)
	}
	fmt.Printf("found %d OGG files in %s\n", len(oggFiles), *dirFlag)

	// Extract embeddings
	var samples []sample
	tmpDir, err := os.MkdirTemp("", "voiceprint-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktemp: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	for _, path := range oggFiles {
		speaker := parseSpeaker(filepath.Base(path))
		if speaker == "" {
			continue
		}

		embPath := filepath.Join(tmpDir, filepath.Base(path)+".emb")

		// Build command
		args := []string{
			"-engine=" + *engineFlag,
			"-output=" + embPath,
		}
		if *modelFlag != "" {
			args = append(args, "-model="+*modelFlag)
		}
		args = append(args, path)

		cmd := exec.Command(*vpFlag, args...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: voiceprint failed: %v\n", filepath.Base(path), err)
			continue
		}

		// Read embedding
		emb, err := readEmbedding(embPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: read embedding: %v\n", filepath.Base(path), err)
			continue
		}

		samples = append(samples, sample{
			path:    filepath.Base(path),
			speaker: speaker,
			emb:     emb,
		})
		fmt.Printf("  [%s] %s — %d dims\n", speaker, filepath.Base(path), len(emb))
	}

	if len(samples) < 2 {
		fmt.Fprintf(os.Stderr, "need at least 2 samples\n")
		os.Exit(1)
	}

	// Sort by speaker
	sort.Slice(samples, func(i, j int) bool {
		if samples[i].speaker != samples[j].speaker {
			return samples[i].speaker < samples[j].speaker
		}
		return samples[i].path < samples[j].path
	})

	// Analyze
	analyze(samples)
}

func parseSpeaker(name string) string {
	name = strings.TrimSuffix(name, ".ogg")
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2]
}

func readEmbedding(path string) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	n := len(data) / 4
	emb := make([]float32, n)
	for i := range emb {
		emb[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return emb, nil
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

func analyze(samples []sample) {
	speakers := map[string][]int{}
	for i, s := range samples {
		speakers[s.speaker] = append(speakers[s.speaker], i)
	}

	var intraSims, interSims []float64

	for _, idxs := range speakers {
		if len(idxs) < 2 {
			continue
		}
		for i := 0; i < len(idxs); i++ {
			for j := i + 1; j < len(idxs); j++ {
				intraSims = append(intraSims, cosineSimilarity(samples[idxs[i]].emb, samples[idxs[j]].emb))
			}
		}
	}

	speakerNames := make([]string, 0, len(speakers))
	for sp := range speakers {
		speakerNames = append(speakerNames, sp)
	}
	sort.Strings(speakerNames)
	for i := 0; i < len(speakerNames); i++ {
		for j := i + 1; j < len(speakerNames); j++ {
			for _, ii := range speakers[speakerNames[i]] {
				for _, jj := range speakers[speakerNames[j]] {
					interSims = append(interSims, cosineSimilarity(samples[ii].emb, samples[jj].emb))
				}
			}
		}
	}

	fmt.Printf("\n=== E2E Voiceprint Results (%d samples, %d speakers) ===\n\n", len(samples), len(speakers))
	fmt.Printf("Speakers: %s\n\n", strings.Join(speakerNames, ", "))

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
		fmt.Printf("GAP:           %.4f\n", gap)
		if gap > 0.3 {
			fmt.Println("\nVERDICT: EXCELLENT — model discriminates well")
		} else if gap > 0.1 {
			fmt.Println("\nVERDICT: OK — model shows speaker discrimination")
		} else {
			fmt.Println("\nVERDICT: WEAK — model has poor discrimination")
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
