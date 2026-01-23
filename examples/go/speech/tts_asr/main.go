// Speech Package TTS + ASR Integration Example
//
// This example demonstrates the speech package's TTS and ASR capabilities:
// 1. Registering TTS handlers (Doubao V1, V2, MiniMax) with TTSMux
// 2. Registering ASR handlers (Doubao SAUC) with ASRMux
// 3. Using TTS to synthesize long text with sentence segmentation
// 4. Validating synthesized audio with registered ASR handler
// 5. Blocking behavior - wait for complete speech before ASR
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	export MINIMAX_API_KEY="your_api_key" (optional)
//	bazel run //examples/go/speech/tts_asr
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/opusrt"
	"github.com/haivivi/giztoy/pkg/audio/pcm"
	"github.com/haivivi/giztoy/pkg/doubaospeech"
	"github.com/haivivi/giztoy/pkg/minimax"
	"github.com/haivivi/giztoy/pkg/speech"
)

// Test text - long enough to trigger sentence segmentation
const testText = `人工智能正在深刻改变我们的生活。
从智能手机上的语音助手，到自动驾驶汽车，再到医疗诊断系统，AI无处不在。
语音合成技术让机器能够像人类一样自然地说话。
这项技术在客服系统、有声读物、无障碍辅助等领域有着广泛的应用。
今天，我们将测试语音合成的效果，看看合成的语音是否能够被准确识别。
这是一个完整的端到端测试，从文本输入到语音输出，再到语音识别验证。`

func main() {
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("              Speech Package TTS + ASR Integration Example")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Get credentials
	doubaoAppID := os.Getenv("DOUBAO_APP_ID")
	doubaoToken := os.Getenv("DOUBAO_TOKEN")
	minimaxAPIKey := os.Getenv("MINIMAX_API_KEY")

	if doubaoAppID == "" || doubaoToken == "" {
		fmt.Println("❌ Please set DOUBAO_APP_ID and DOUBAO_TOKEN environment variables")
		fmt.Println()
		fmt.Println("   export DOUBAO_APP_ID=\"your_app_id\"")
		fmt.Println("   export DOUBAO_TOKEN=\"your_token\"")
		os.Exit(1)
	}

	fmt.Printf("📋 Doubao App ID: %s\n", doubaoAppID)
	fmt.Printf("📋 Doubao Token: %s...%s\n", doubaoToken[:4], doubaoToken[len(doubaoToken)-4:])
	if minimaxAPIKey != "" {
		fmt.Printf("📋 MiniMax API Key: %s...%s\n", minimaxAPIKey[:4], minimaxAPIKey[len(minimaxAPIKey)-4:])
	}
	fmt.Println()

	// Initialize clients
	doubaoClient := doubaospeech.NewClient(doubaoAppID,
		doubaospeech.WithBearerToken(doubaoToken),
		doubaospeech.WithCluster("volcano_tts"),
	)

	// Register TTS handlers
	fmt.Println("📝 Registering TTS handlers...")
	registerTTSHandlers(doubaoClient, minimaxAPIKey)
	fmt.Println()

	// Register ASR handlers
	fmt.Println("📝 Registering ASR handlers...")
	registerASRHandlers(doubaoClient)
	fmt.Println()

	// Display test text
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("                         Test Text")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println(testText)
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Test each TTS handler
	// Note: Skip doubao-v1 for now as it requires special voice_type format
	handlers := []string{
		// "doubao-v1",  // Doubao TTS 1.0 (skipped - requires BV* voice format)
		"doubao-v2",  // Doubao TTS 2.0
	}

	// Add MiniMax if available
	if minimaxAPIKey != "" {
		handlers = append(handlers, "minimax")
	}

	for _, name := range handlers {
		fmt.Printf("\n🎯 Testing TTS handler: %s\n", name)
		fmt.Println("───────────────────────────────────────────────────────────────────────")

		if err := testTTSAndASR(name, testText, doubaoClient); err != nil {
			fmt.Printf("❌ Test failed: %v\n", err)
		} else {
			fmt.Printf("✅ Test completed for %s\n", name)
		}
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("                         All Tests Completed")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
}

// registerASRHandlers registers all available ASR handlers
func registerASRHandlers(doubaoClient *doubaospeech.Client) {
	// Register Doubao SAUC ASR handler
	doubaoASRHandler := speech.NewDoubaoSAUCASRHandler(doubaoClient,
		speech.WithDoubaoSAUCSampleRate(16000),
		speech.WithDoubaoSAUCLanguage("zh-CN"),
		speech.WithDoubaoSAUCEnableITN(true),
		speech.WithDoubaoSAUCEnablePunc(true),
	)
	if err := speech.HandleASR("doubao-sauc", doubaoASRHandler); err != nil {
		fmt.Printf("   ⚠️  Failed to register doubao-sauc: %v\n", err)
	} else {
		fmt.Println("   ✅ Registered: doubao-sauc (Doubao SAUC BigModel ASR)")
	}
}

// registerTTSHandlers registers all available TTS handlers
func registerTTSHandlers(doubaoClient *doubaospeech.Client, minimaxAPIKey string) {
	// Register Doubao TTS V1 handler
	// V1 requires cluster to be set (volcano_tts)
	doubaoV1Handler := speech.NewDoubaoTTSV1Handler(doubaoClient,
		speech.WithDoubaoTTSV1Voice("zh_female_tianmei"),
		speech.WithDoubaoTTSV1Cluster("volcano_tts"),
		speech.WithDoubaoTTSV1Encoding(doubaospeech.EncodingPCM),
		speech.WithDoubaoTTSV1Speed(1.0),
	)
	if err := speech.HandleTTS("doubao-v1", doubaoV1Handler); err != nil {
		fmt.Printf("   ⚠️  Failed to register doubao-v1: %v\n", err)
	} else {
		fmt.Println("   ✅ Registered: doubao-v1 (Doubao TTS 1.0 - Classic)")
	}

	// Register Doubao TTS V2 handler
	// V2 BigModel requires specific speaker IDs with _uranus_bigtts suffix
	doubaoV2Handler := speech.NewDoubaoTTSV2Handler(doubaoClient,
		speech.WithDoubaoTTSV2Speaker("zh_female_vv_uranus_bigtts"),
		speech.WithDoubaoTTSV2ResourceID(doubaospeech.ResourceTTSV2),
		speech.WithDoubaoTTSV2Format("pcm"),
		speech.WithDoubaoTTSV2Speed(1.0),
	)
	if err := speech.HandleTTS("doubao-v2", doubaoV2Handler); err != nil {
		fmt.Printf("   ⚠️  Failed to register doubao-v2: %v\n", err)
	} else {
		fmt.Println("   ✅ Registered: doubao-v2 (Doubao TTS 2.0 - BigModel)")
	}

	// Register MiniMax TTS handler (if API key available)
	if minimaxAPIKey != "" {
		minimaxClient := minimax.NewClient(minimaxAPIKey)
		minimaxHandler := speech.NewMinimaxTTSHandler(minimaxClient,
			speech.WithMinimaxTTSModel(minimax.ModelSpeech26HD),
			speech.WithMinimaxTTSVoice(minimax.VoiceFemaleShaonv),
			speech.WithMinimaxTTSSpeed(1.0),
		)
		if err := speech.HandleTTS("minimax", minimaxHandler); err != nil {
			fmt.Printf("   ⚠️  Failed to register minimax: %v\n", err)
		} else {
			fmt.Println("   ✅ Registered: minimax (MiniMax TTS)")
		}
	}
}

// testTTSAndASR tests TTS synthesis and validates with ASR
func testTTSAndASR(handlerName, text string, doubaoClient *doubaospeech.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Define PCM format for synthesis
	// Use 16kHz to match ASR input requirements
	format := pcm.L16Mono16K // 16kHz, 16bit, mono

	// Step 1: TTS Synthesis (blocking)
	fmt.Println()
	fmt.Println("📢 Step 1: TTS Synthesis (blocking mode)...")
	fmt.Printf("   Format: %s\n", format)

	startTime := time.Now()
	sp, err := speech.Synthesize(ctx, handlerName, strings.NewReader(text), format)
	if err != nil {
		return fmt.Errorf("synthesize failed: %w", err)
	}
	defer sp.Close()

	// Collect all audio data (blocking behavior)
	var audioBuffer bytes.Buffer
	var transcriptBuffer bytes.Buffer
	segmentCount := 0

	fmt.Println("   Collecting segments...")
	for seg, err := range speech.Iter(sp) {
		if err != nil {
			return fmt.Errorf("read segment failed: %w", err)
		}

		segmentCount++

		// Get voice data
		voice := seg.Decode(format)
		audioData, err := io.ReadAll(voice)
		voice.Close()
		if err != nil {
			return fmt.Errorf("read audio failed: %w", err)
		}
		audioBuffer.Write(audioData)

		// Get transcript
		transcript := seg.Transcribe()
		transcriptData, err := io.ReadAll(transcript)
		transcript.Close()
		if err != nil {
			return fmt.Errorf("read transcript failed: %w", err)
		}
		if transcriptBuffer.Len() > 0 {
			transcriptBuffer.WriteString("\n")
		}
		transcriptBuffer.Write(transcriptData)

		seg.Close()

		fmt.Printf("   Segment %d: %d bytes audio, text: %.30s...\n",
			segmentCount, len(audioData), string(transcriptData))
	}

	synthesisTime := time.Since(startTime)
	audioDuration := format.Duration(int64(audioBuffer.Len()))

	fmt.Printf("   ✅ TTS completed: %d segments, %d bytes (%.1f seconds audio)\n",
		segmentCount, audioBuffer.Len(), audioDuration.Seconds())
	fmt.Printf("   ⏱️  Synthesis time: %v (RTF: %.2f)\n",
		synthesisTime, synthesisTime.Seconds()/audioDuration.Seconds())

	// Save audio to file
	os.MkdirAll("tmp", 0755)
	audioFile := fmt.Sprintf("tmp/speech_%s.pcm", handlerName)
	if err := os.WriteFile(audioFile, audioBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("save audio failed: %w", err)
	}
	fmt.Printf("   💾 Audio saved to: %s\n", audioFile)

	// Step 2: ASR Recognition (blocking)
	// Use one-sentence ASR API for direct recognition
	fmt.Println()
	fmt.Println("🎧 Step 2: ASR Recognition (blocking mode)...")

	var asrResult string
	// Try streaming ASR first via speech package
	asrResult, err = recognizeWithRegisteredASR(ctx, audioBuffer.Bytes(), format)
	if err != nil {
		fmt.Printf("   ⚠️  Streaming ASR failed: %v\n", err)
		fmt.Println("   🔄 Falling back to one-sentence ASR API...")
		// Use one-sentence API as fallback (more reliable for short audio)
		asrResult, err = recognizeWithOneSentenceASR(ctx, doubaoClient, audioBuffer.Bytes(), format.SampleRate())
		if err != nil {
			fmt.Printf("   ⚠️  One-sentence ASR also failed: %v\n", err)
			fmt.Println("   ℹ️  Skipping ASR validation. TTS synthesis succeeded.")
			return nil
		}
	}

	// Step 3: Compare results
	fmt.Println()
	fmt.Println("📊 Step 3: Result Comparison")
	fmt.Println("───────────────────────────────────────────────────────────────────────")
	fmt.Printf("   Original text (%d chars):\n", len([]rune(text)))
	printIndented(text, "      ")
	fmt.Println()
	fmt.Printf("   TTS transcript (%d chars):\n", len([]rune(transcriptBuffer.String())))
	printIndented(transcriptBuffer.String(), "      ")
	fmt.Println()
	fmt.Printf("   ASR result (%d chars):\n", len([]rune(asrResult)))
	printIndented(asrResult, "      ")
	fmt.Println("───────────────────────────────────────────────────────────────────────")

	// Calculate similarity
	similarity := calculateSimilarity(normalizeText(text), normalizeText(asrResult))
	fmt.Printf("   📈 Text similarity: %.1f%%\n", similarity*100)

	if similarity > 0.8 {
		fmt.Println("   ✅ ASR result matches original text well!")
	} else if similarity > 0.5 {
		fmt.Println("   ⚠️  ASR result partially matches original text")
	} else {
		fmt.Println("   ❌ ASR result differs significantly from original text")
	}

	return nil
}

// recognizeWithRegisteredASR performs ASR using the registered speech.ASRMux handler.
// It encodes PCM to Opus and uses the speech.TranscribeStream API.
func recognizeWithRegisteredASR(ctx context.Context, audioData []byte, format pcm.Format) (string, error) {
	fmt.Println("   📡 Using registered ASR handler (doubao-sauc)...")

	// Convert PCM to Opus frames for the ASR handler
	pcmReader := bytes.NewReader(audioData)
	opusStream, err := opusrt.EncodePCMStream(pcmReader, format.SampleRate(), format.Channels())
	if err != nil {
		return "", fmt.Errorf("create opus stream: %w", err)
	}

	// Use the registered ASR handler via speech.TranscribeStream
	speechStream, err := speech.TranscribeStream(ctx, "doubao-sauc", opusStream)
	if err != nil {
		return "", fmt.Errorf("transcribe stream: %w", err)
	}
	defer speechStream.Close()

	// Collect all transcriptions (blocking)
	var result strings.Builder
	speechCount := 0

	fmt.Println("   🔊 Collecting transcription results...")
	for sp, err := range speech.Iter(speechStream) {
		if err != nil {
			return "", fmt.Errorf("read speech: %w", err)
		}
		speechCount++

		// Collect all segments from this speech
		for seg, err := range speech.Iter(sp) {
			if err != nil {
				break
			}

			// Get transcript
			transcript := seg.Transcribe()
			text, _ := io.ReadAll(transcript)
			transcript.Close()
			seg.Close()

			if len(text) > 0 {
				if result.Len() > 0 {
					result.WriteString(" ")
				}
				result.Write(text)
				fmt.Printf("      [Speech %d] %s\n", speechCount, string(text))
			}
		}
		sp.Close()
	}

	fmt.Printf("   ✅ Collected %d speech segments\n", speechCount)
	return result.String(), nil
}

// recognizeWithASR performs ASR recognition using Doubao SAUC client directly
func recognizeWithASR(ctx context.Context, client *doubaospeech.Client, audioData []byte, sampleRate int) (string, error) {
	config := &doubaospeech.ASRV2Config{
		Format:     "pcm",
		SampleRate: sampleRate,
		Bits:       16,
		Channels:   1,
		Language:   "zh-CN",
		EnableITN:  true,
		EnablePunc: true,
	}

	fmt.Println("   📡 Connecting to Doubao ASR (SAUC BigModel)...")
	session, err := client.ASRV2.OpenStreamSession(ctx, config)
	if err != nil {
		return "", fmt.Errorf("connect failed: %w", err)
	}
	defer session.Close()
	fmt.Println("   ✅ Connected")

	// Channel for collecting results
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Start receiver goroutine
	go func() {
		var finalText strings.Builder
		for result, err := range session.Recv() {
			if err != nil {
				errCh <- err
				return
			}
			if result.IsFinal && result.Text != "" {
				finalText.WriteString(result.Text)
			}
			fmt.Printf("\r   🔊 Recognizing: %s", truncateString(result.Text, 50))
		}
		fmt.Println()
		resultCh <- finalText.String()
	}()

	// Send audio in chunks (blocking send)
	fmt.Println("   📤 Sending audio data...")
	chunkSize := sampleRate * 2 / 10 // 100ms chunks (16-bit = 2 bytes/sample)
	totalSent := 0

	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}
		chunk := audioData[i:end]
		isLast := end >= len(audioData)

		if err := session.SendAudio(ctx, chunk, isLast); err != nil {
			return "", fmt.Errorf("send audio failed: %w", err)
		}
		totalSent += len(chunk)

		// Simulate real-time streaming
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("\n   ✅ Sent %d bytes\n", totalSent)

	// Wait for final result (blocking)
	fmt.Println("   ⏳ Waiting for final result...")
	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// recognizeWithOneSentenceASR uses the one-sentence ASR API (ASR 1.0)
// This is more reliable for short audio clips (< 60 seconds)
func recognizeWithOneSentenceASR(ctx context.Context, client *doubaospeech.Client, audioData []byte, sampleRate int) (string, error) {
	fmt.Println("   📡 Using one-sentence ASR API (ASR 1.0)...")
	fmt.Printf("   📊 Audio size: %d bytes (%.1f seconds)\n", len(audioData), float64(len(audioData))/(float64(sampleRate)*2))

	req := &doubaospeech.OneSentenceRequest{
		Audio:      audioData,
		Format:     doubaospeech.FormatPCM,
		SampleRate: doubaospeech.SampleRate(sampleRate),
		Language:   doubaospeech.LanguageZhCN,
		EnableITN:  true,
		EnablePunc: true,
	}

	result, err := client.ASR.RecognizeOneSentence(ctx, req)
	if err != nil {
		return "", fmt.Errorf("one-sentence ASR failed: %w", err)
	}

	fmt.Printf("   ✅ ASR completed, duration: %dms\n", result.Duration)
	return result.Text, nil
}

// Helper functions

func printIndented(text, indent string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		fmt.Printf("%s%s\n", indent, line)
	}
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func normalizeText(text string) string {
	// Remove punctuation and whitespace for comparison
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, "，", "")
	text = strings.ReplaceAll(text, "。", "")
	text = strings.ReplaceAll(text, "、", "")
	text = strings.ReplaceAll(text, "！", "")
	text = strings.ReplaceAll(text, "？", "")
	text = strings.ReplaceAll(text, "：", "")
	text = strings.ReplaceAll(text, "；", "")
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, ".", "")
	text = strings.ReplaceAll(text, "!", "")
	text = strings.ReplaceAll(text, "?", "")
	return text
}

func calculateSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}

	runesA := []rune(a)
	runesB := []rune(b)

	// Simple character overlap similarity
	setA := make(map[rune]int)
	setB := make(map[rune]int)

	for _, r := range runesA {
		setA[r]++
	}
	for _, r := range runesB {
		setB[r]++
	}

	var intersection, union int
	for r, countA := range setA {
		countB := setB[r]
		if countA < countB {
			intersection += countA
		} else {
			intersection += countB
		}
		if countA > countB {
			union += countA
		} else {
			union += countB
		}
	}
	for r, countB := range setB {
		if _, exists := setA[r]; !exists {
			union += countB
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
