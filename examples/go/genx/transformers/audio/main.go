// Package main provides integration tests for TTS and ASR transformers.
//
// This test validates the streaming pipeline:
//
//	Original Text -> TTS -> ASR -> TTS -> ASR -> Final Text
//
// The final text should be similar to the original text.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

// Sample text for testing (~400 Chinese characters)
const sampleText = `在一个遥远的星球上，有一座古老的城市。这座城市被茂密的森林环绕，清澈的河流穿城而过。
城市中心矗立着一座高塔，塔顶闪烁着神秘的光芒。传说这座塔是由远古的智者建造的，里面藏着无数的秘密和宝藏。
每当夜幕降临，星星点点的灯火照亮了整个城市。人们聚集在广场上，分享着一天的故事和快乐。
孩子们在街道上奔跑嬉戏，老人们坐在树下下棋聊天。这里没有战争，没有纷争，只有和平与宁静。
然而，一个古老的预言说，有一天会有一位勇者来到这里，揭开塔中的秘密，为这个世界带来新的希望。
人们期待着那一天的到来，同时也珍惜着现在的每一刻美好时光。`

var (
	testCase = flag.String("test", "all", "Test case to run: minimax, doubao, all")
	timeout  = flag.Duration("timeout", 5*time.Minute, "Test timeout duration")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("=== Transformers Audio Integration Test ===")
	fmt.Printf("Original text (%d chars):\n%s\n\n", len([]rune(sampleText)), sampleText)

	switch *testCase {
	case "minimax":
		runMinimaxTest(ctx)
	case "doubao":
		runDoubaoTest(ctx)
	case "all":
		runMinimaxTest(ctx)
		fmt.Println("\n" + strings.Repeat("=", 60) + "\n")
		runDoubaoTest(ctx)
	default:
		log.Fatalf("Unknown test case: %s", *testCase)
	}
}

func initDoubaoClient() (*doubaospeech.Client, error) {
	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")
	if appID == "" || token == "" {
		return nil, fmt.Errorf("DOUBAO_APP_ID and DOUBAO_TOKEN must be set")
	}
	return doubaospeech.NewClient(appID, doubaospeech.WithBearerToken(token)), nil
}

func initMinimaxClient() (*minimax.Client, error) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("MINIMAX_API_KEY must be set")
	}
	return minimax.NewClient(apiKey), nil
}

func runMinimaxTest(ctx context.Context) {
	fmt.Println("--- Test: MiniMax TTS -> Codec -> Doubao ASR -> MiniMax TTS -> Codec -> Doubao ASR ---")

	// Initialize clients
	doubaoClient, err := initDoubaoClient()
	if err != nil {
		log.Printf("Failed to init Doubao client: %v", err)
		return
	}

	minimaxClient, err := initMinimaxClient()
	if err != nil {
		log.Printf("Failed to init MiniMax client: %v", err)
		return
	}

	// Create transformers
	// Use 24000 Hz sample rate - compatible with both MP3 and Opus
	tts := transformers.NewMinimaxTTS(minimaxClient, "female-shaonv",
		transformers.WithMinimaxTTSFormat("mp3"),
		transformers.WithMinimaxTTSSampleRate(24000),
	)
	codec := transformers.NewMP3ToOgg()

	// Round 1: TTS -> Codec -> ASR
	fmt.Println("[1] TTS (MiniMax)...")
	stream := textToStream(sampleText)
	stream, err = tts.Transform(ctx, "", stream)
	if err != nil {
		log.Printf("Round 1 TTS error: %v", err)
		return
	}

	fmt.Println("[2] Codec (MP3 -> OGG)...")
	stream, err = codec.Transform(ctx, "", stream)
	if err != nil {
		log.Printf("Round 1 Codec error: %v", err)
		return
	}

	// MiniMax TTS at 24000 Hz -> codec converts to OGG at same rate
	text1, err := ttsToASR(ctx, doubaoClient, stream, "Round 1", 24000)
	if err != nil {
		log.Printf("Round 1 Error: %v", err)
		return
	}
	fmt.Printf("\nRound 1 ASR result (%d chars): %s\n\n", len([]rune(text1)), truncate(text1, 100))

	if text1 == "" {
		log.Printf("Round 1 ASR returned empty text")
		return
	}

	// Round 2: TTS -> Codec -> ASR
	fmt.Println("[3] TTS (MiniMax)...")
	stream = textToStream(text1)
	stream, err = tts.Transform(ctx, "", stream)
	if err != nil {
		log.Printf("Round 2 TTS error: %v", err)
		return
	}

	fmt.Println("[4] Codec (MP3 -> OGG)...")
	stream, err = codec.Transform(ctx, "", stream)
	if err != nil {
		log.Printf("Round 2 Codec error: %v", err)
		return
	}

	text2, err := ttsToASR(ctx, doubaoClient, stream, "Round 2", 24000)
	if err != nil {
		log.Printf("Round 2 Error: %v", err)
		return
	}

	printComparison(sampleText, text2)
}

func runDoubaoTest(ctx context.Context) {
	fmt.Println("--- Test: Doubao TTS -> Doubao ASR -> Doubao TTS -> Doubao ASR ---")

	// Initialize client
	doubaoClient, err := initDoubaoClient()
	if err != nil {
		log.Printf("Failed to init Doubao client: %v", err)
		return
	}

	// Create TTS transformer
	// Note: seed-tts-2.0 requires *_uranus_bigtts suffix speakers
	tts := transformers.NewDoubaoTTSSeedV2(doubaoClient, "zh_female_vv_uranus_bigtts",
		transformers.WithDoubaoTTSSeedV2Format("ogg_opus"),
	)

	// Round 1: TTS -> ASR
	// Doubao TTS ogg_opus is at 24000 Hz
	fmt.Println("[1] TTS (Doubao)...")
	stream := textToStream(sampleText)
	stream, err = tts.Transform(ctx, "", stream)
	if err != nil {
		log.Printf("Round 1 TTS error: %v", err)
		return
	}

	text1, err := ttsToASR(ctx, doubaoClient, stream, "Round 1", 24000)
	if err != nil {
		log.Printf("Round 1 Error: %v", err)
		return
	}
	fmt.Printf("\nRound 1 ASR result (%d chars): %s\n\n", len([]rune(text1)), truncate(text1, 100))

	if text1 == "" {
		log.Printf("Round 1 ASR returned empty text")
		return
	}

	// Round 2: TTS -> ASR
	fmt.Println("[3] TTS (Doubao)...")
	stream = textToStream(text1)
	stream, err = tts.Transform(ctx, "", stream)
	if err != nil {
		log.Printf("Round 2 TTS error: %v", err)
		return
	}

	text2, err := ttsToASR(ctx, doubaoClient, stream, "Round 2", 24000)
	if err != nil {
		log.Printf("Round 2 Error: %v", err)
		return
	}

	printComparison(sampleText, text2)
}

// ttsToASR collects audio from TTS stream and runs ASR
// sampleRate should match the OGG audio sample rate
func ttsToASR(ctx context.Context, client *doubaospeech.Client, stream genx.Stream, label string, sampleRate int) (string, error) {
	// Collect all audio chunks into one buffer
	var audioBuffer bytes.Buffer
	chunkCount := 0
	for {
		chunk, err := stream.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("TTS error: %w", err)
		}
		if chunk == nil {
			continue
		}
		if blob, ok := chunk.Part.(*genx.Blob); ok {
			audioBuffer.Write(blob.Data)
			chunkCount++
		}
	}
	fmt.Printf("  TTS: %d chunks, %d bytes\n", chunkCount, audioBuffer.Len())

	if audioBuffer.Len() == 0 {
		return "", fmt.Errorf("no audio data from TTS")
	}

	fmt.Printf("[%s] ASR (Doubao, %dHz)...\n", label, sampleRate)

	// Create ASR session - send OGG directly
	config := &doubaospeech.ASRV2Config{
		Format:     "ogg",
		SampleRate: sampleRate,
		Channels:   1,
		Language:   "zh-CN",
		EnableITN:  true,
		EnablePunc: true,
		ResultType: "single", // only definite results
	}
	session, err := client.ASRV2.OpenStreamSession(ctx, config)
	if err != nil {
		return "", fmt.Errorf("ASR session error: %w", err)
	}
	defer session.Close()

	// Start receiving results - use utterances to avoid duplicates
	var results []string
	lastEndTime := 0
	resultsDone := make(chan error, 1)
	go func() {
		for result, err := range session.Recv() {
			if err != nil {
				resultsDone <- err
				return
			}
			// Use utterances array to get clean, non-overlapping results
			if len(result.Utterances) > 0 {
				for _, utt := range result.Utterances {
					// Only collect definite utterances that haven't been processed
					if utt.Definite && utt.EndTime > lastEndTime && utt.Text != "" {
						results = append(results, utt.Text)
						fmt.Printf("  [ASR] %s\n", truncate(utt.Text, 80))
						lastEndTime = utt.EndTime
					}
				}
			} else if result.IsFinal && result.Text != "" {
				// Fallback if no utterances
				results = append(results, result.Text)
				fmt.Printf("  [ASR] %s\n", truncate(result.Text, 80))
			}
		}
		resultsDone <- nil
	}()

	// Send OGG data directly to ASR with rate limiting
	senderDone := make(chan error, 1)
	go func() {
		audioData := audioBuffer.Bytes()
		chunkSize := 4096
		totalSent := 0

		for i := 0; i < len(audioData); i += chunkSize {
			end := i + chunkSize
			if end > len(audioData) {
				end = len(audioData)
			}
			chunk := audioData[i:end]

			if err := session.SendAudio(ctx, chunk, false); err != nil {
				senderDone <- fmt.Errorf("send audio error: %w", err)
				return
			}
			totalSent += len(chunk)

			// Rate limit: ~20ms per chunk
			time.Sleep(20 * time.Millisecond)
		}

		// Send final marker
		if err := session.SendAudio(ctx, nil, true); err != nil {
			senderDone <- fmt.Errorf("send final error: %w", err)
			return
		}
		fmt.Printf("  Sent %d bytes to ASR\n", totalSent)
		senderDone <- nil
	}()

	// Wait for goroutines
	if err := <-senderDone; err != nil {
		return "", err
	}
	if err := <-resultsDone; err != nil {
		return "", fmt.Errorf("ASR error: %w", err)
	}

	return strings.Join(results, ""), nil
}

// textToStream converts a text string to a genx.Stream
func textToStream(text string) genx.Stream {
	return &singleTextStream{
		text: text,
		done: false,
	}
}

type singleTextStream struct {
	text string
	done bool
}

func (s *singleTextStream) Next() (*genx.MessageChunk, error) {
	if s.done {
		return nil, io.EOF
	}
	s.done = true
	return &genx.MessageChunk{
		Role: genx.RoleUser,
		Part: genx.Text(s.text),
	}, nil
}

func (s *singleTextStream) Close() error {
	s.done = true
	return nil
}

func (s *singleTextStream) CloseWithError(err error) error {
	s.done = true
	return nil
}

// collectText reads all text chunks from a stream and concatenates them
func collectText(stream genx.Stream) (string, error) {
	var sb strings.Builder
	chunkCount := 0

	for {
		chunk, err := stream.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("stream error: %w", err)
		}

		if chunk == nil {
			continue
		}

		if text, ok := chunk.Part.(genx.Text); ok {
			sb.WriteString(string(text))
			chunkCount++
			// Print progress
			fmt.Printf("  [ASR chunk %d]: %s\n", chunkCount, truncate(string(text), 50))
		}
	}

	return sb.String(), nil
}

func printComparison(original, final string) {
	fmt.Println("\n=== Comparison ===")
	fmt.Printf("Original (%d chars):\n%s\n\n", len([]rune(original)), truncate(original, 200))
	fmt.Printf("Final (%d chars):\n%s\n\n", len([]rune(final)), truncate(final, 200))

	// Calculate similarity (simple character-based)
	similarity := calculateSimilarity(original, final)
	fmt.Printf("Similarity: %.1f%%\n", similarity*100)
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// calculateSimilarity calculates a simple similarity ratio
func calculateSimilarity(a, b string) float64 {
	// Remove whitespace and punctuation for comparison
	cleanA := cleanText(a)
	cleanB := cleanText(b)

	if len(cleanA) == 0 && len(cleanB) == 0 {
		return 1.0
	}
	if len(cleanA) == 0 || len(cleanB) == 0 {
		return 0.0
	}

	// Count matching characters
	matches := 0
	runesA := []rune(cleanA)
	runesB := []rune(cleanB)

	// Create a map of characters in B
	charMap := make(map[rune]int)
	for _, r := range runesB {
		charMap[r]++
	}

	// Count how many characters from A are in B
	for _, r := range runesA {
		if charMap[r] > 0 {
			matches++
			charMap[r]--
		}
	}

	maxLen := len(runesA)
	if len(runesB) > maxLen {
		maxLen = len(runesB)
	}

	return float64(matches) / float64(maxLen)
}

func cleanText(s string) string {
	var sb strings.Builder
	for _, r := range s {
		// Keep only Chinese characters and alphanumeric
		if r >= 0x4e00 && r <= 0x9fff || // CJK Unified Ideographs
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
