// Package main tests DashScope Qwen-Omni-Realtime transformer features.
//
// This test validates:
// 1. Basic streaming pipeline (TTS -> CompositeSeq -> Realtime)
// 2. ASR transcription with various models
// 3. Server VAD mode with automatic turn detection
// 4. Different voices and models
//
// Required environment variables:
//   - DASHSCOPE_API_KEY or QWEN_API_KEY
//   - MINIMAX_API_KEY (for TTS)
//
// Usage:
//
//	bazel run //examples/go/genx/transformers/dashscope_realtime -- -mode=basic
//	bazel run //examples/go/genx/transformers/dashscope_realtime -- -mode=asr
//	bazel run //examples/go/genx/transformers/dashscope_realtime -- -mode=vad
//	bazel run //examples/go/genx/transformers/dashscope_realtime -- -voice=Cherry
//	bazel run //examples/go/genx/transformers/dashscope_realtime -- -model=flash
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/dashscope"
	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
	"github.com/haivivi/giztoy/e2e/go/genx/transformers/internal"
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

// Test sentences
var sentences = []string{
	"你好，我是小明。",
	"今天天气怎么样？",
	"请给我讲一个笑话。",
}

// Voice test sentences (for testing voice switching)
var voiceSentences = []string{
	"你好，请用标准普通话跟我打个招呼。",
	"大妈你好啊，最近咋样啊？吃饭了没？",
}

var (
	mode      = flag.String("mode", "basic", "Test mode: basic, asr, vad, voice")
	voice     = flag.String("voice", "Chelsie", "Voice: Chelsie, Cherry, Serena, Ethan")
	model     = flag.String("model", "turbo", "Model: turbo, flash")
	timeout   = flag.Duration("timeout", 3*time.Minute, "Test timeout")
	outputDir = flag.String("output", "/tmp/dashscope_test", "Output directory for audio files")
	verify    = flag.Bool("verify", false, "Verify generated audio with ASR (requires DOUBAO_API_KEY)")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	fmt.Println("=== DashScope Realtime Test ===")
	fmt.Printf("Mode:   %s\n", *mode)
	fmt.Printf("Voice:  %s\n", *voice)
	fmt.Printf("Model:  %s\n", *model)
	fmt.Printf("Output: %s\n", *outputDir)
	fmt.Println()

	// Get API keys
	dsKey := os.Getenv("DASHSCOPE_API_KEY")
	if dsKey == "" {
		dsKey = os.Getenv("QWEN_API_KEY")
	}
	mmKey := os.Getenv("MINIMAX_API_KEY")

	if dsKey == "" {
		log.Fatal("DASHSCOPE_API_KEY or QWEN_API_KEY required")
	}
	if mmKey == "" {
		log.Fatal("MINIMAX_API_KEY required")
	}

	// Create clients
	dsClient := dashscope.NewClient(dsKey)
	mmClient := minimax.NewClient(mmKey)

	switch *mode {
	case "basic":
		runBasicTest(ctx, dsClient, mmClient)
	case "asr":
		runASRTest(ctx, dsClient, mmClient)
	case "vad":
		runVADTest(ctx, dsClient, mmClient)
	case "voice":
		runVoiceTest(ctx, dsClient, mmClient)
	default:
		log.Fatalf("Unknown mode: %s", *mode)
	}
}

func getModelID() string {
	switch *model {
	case "flash":
		return dashscope.ModelQwen3OmniFlashRealtime
	default:
		return dashscope.ModelQwenOmniTurboRealtimeLatest
	}
}

func getVoiceID() string {
	switch *voice {
	case "Cherry":
		return dashscope.VoiceCherry
	case "Serena":
		return dashscope.VoiceSerena
	case "Ethan":
		return dashscope.VoiceEthan
	default:
		return dashscope.VoiceChelsie
	}
}

// runBasicTest tests basic streaming without ASR
func runBasicTest(ctx context.Context, dsClient *dashscope.Client, mmClient *minimax.Client) {
	fmt.Println("--- Basic Streaming Test ---")

	// Create TTS
	tts := transformers.NewMinimaxTTS(mmClient, "female-shaonv",
		transformers.WithMinimaxTTSFormat("pcm"),
		transformers.WithMinimaxTTSSampleRate(16000),
	)

	// Create realtime transformer (no ASR)
	realtime := transformers.NewDashScopeRealtime(dsClient,
		transformers.WithDashScopeRealtimeModel(getModelID()),
		transformers.WithDashScopeRealtimeVoice(getVoiceID()),
		transformers.WithDashScopeRealtimeModalities([]string{"text", "audio"}),
		transformers.WithDashScopeRealtimeInstructions("你是一个友好的助手，用简短的话回答问题。"),
		transformers.WithDashScopeRealtimeEnableASR(false),
		transformers.WithDashScopeRealtimeOutputAudioFormat("mp3"),
	)

	runStreamingPipeline(ctx, tts, realtime, "Basic", sentences)
}

// runASRTest tests with ASR transcription enabled
func runASRTest(ctx context.Context, dsClient *dashscope.Client, mmClient *minimax.Client) {
	fmt.Println("--- ASR Transcription Test ---")

	// Create TTS
	tts := transformers.NewMinimaxTTS(mmClient, "female-shaonv",
		transformers.WithMinimaxTTSFormat("pcm"),
		transformers.WithMinimaxTTSSampleRate(16000),
	)

	// Create realtime transformer with ASR
	realtime := transformers.NewDashScopeRealtime(dsClient,
		transformers.WithDashScopeRealtimeModel(getModelID()),
		transformers.WithDashScopeRealtimeVoice(getVoiceID()),
		transformers.WithDashScopeRealtimeModalities([]string{"text", "audio"}),
		transformers.WithDashScopeRealtimeInstructions("你是一个友好的助手，用简短的话回答问题。"),
		transformers.WithDashScopeRealtimeEnableASR(true),
		transformers.WithDashScopeRealtimeASRModel("qwen-audio-turbo"),
		transformers.WithDashScopeRealtimeOutputAudioFormat("mp3"),
	)

	runStreamingPipeline(ctx, tts, realtime, "ASR", sentences)
}

// runVADTest tests with server-side VAD
func runVADTest(ctx context.Context, dsClient *dashscope.Client, mmClient *minimax.Client) {
	fmt.Println("--- Server VAD Test ---")

	// Create TTS
	tts := transformers.NewMinimaxTTS(mmClient, "female-shaonv",
		transformers.WithMinimaxTTSFormat("pcm"),
		transformers.WithMinimaxTTSSampleRate(16000),
	)

	// Create realtime transformer with server VAD
	realtime := transformers.NewDashScopeRealtime(dsClient,
		transformers.WithDashScopeRealtimeModel(getModelID()),
		transformers.WithDashScopeRealtimeVoice(getVoiceID()),
		transformers.WithDashScopeRealtimeModalities([]string{"text", "audio"}),
		transformers.WithDashScopeRealtimeInstructions("你是一个友好的助手，用简短的话回答问题。"),
		transformers.WithDashScopeRealtimeEnableASR(true),
		transformers.WithDashScopeRealtimeTurnDetection(&dashscope.TurnDetection{
			Type:              dashscope.VADModeServerVAD,
			SilenceDurationMs: 800,  // 800ms silence to detect end of speech
			Threshold:         0.5,  // VAD sensitivity
			PrefixPaddingMs:   300,  // Padding before speech
		}),
		transformers.WithDashScopeRealtimeOutputAudioFormat("mp3"),
	)

	runStreamingPipeline(ctx, tts, realtime, "VAD", sentences)
}

// runVoiceTest tests dynamic voice switching
func runVoiceTest(ctx context.Context, dsClient *dashscope.Client, mmClient *minimax.Client) {
	fmt.Println("--- Voice Switching Test ---")

	// Create TTS
	tts := transformers.NewMinimaxTTS(mmClient, "female-shaonv",
		transformers.WithMinimaxTTSFormat("pcm"),
		transformers.WithMinimaxTTSSampleRate(16000),
	)

	// Create realtime transformer with first voice (Chelsie)
	realtime := transformers.NewDashScopeRealtime(dsClient,
		transformers.WithDashScopeRealtimeModel(getModelID()),
		transformers.WithDashScopeRealtimeVoice(dashscope.VoiceChelsie),
		transformers.WithDashScopeRealtimeModalities([]string{"text", "audio"}),
		transformers.WithDashScopeRealtimeInstructions("你是一个友好的助手，用简短的话回答问题。"),
		transformers.WithDashScopeRealtimeEnableASR(true),
		transformers.WithDashScopeRealtimeOutputAudioFormat("mp3"),
	)

	runVoiceSwitchPipeline(ctx, tts, realtime, "VoiceSwitch", voiceSentences)
}

func runStreamingPipeline(ctx context.Context, tts *transformers.MinimaxTTS, realtime *transformers.DashScopeRealtime, testName string, testSentences []string) {
	fmt.Println("[1] Testing TTS...")
	testStream, err := tts.Transform(ctx, "", textToStream(testSentences[0]))
	if err != nil {
		log.Printf("TTS transform error: %v", err)
		return
	}
	ttsBytes := 0
	for {
		chunk, err := testStream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("TTS error: %v", err)
			return
		}
		if chunk != nil {
			if blob, ok := chunk.Part.(*genx.Blob); ok {
				ttsBytes += len(blob.Data)
			}
		}
	}
	fmt.Printf("  TTS OK: %d bytes\n", ttsBytes)

	// Build streaming pipeline
	// TTS now generates StreamID internally (BOS + StreamID on all chunks + EOS)
	fmt.Println("[2] Building pipeline...")
	ttsStreams := make([]genx.Stream, len(testSentences))
	for i, sentence := range testSentences {
		stream, err := tts.Transform(ctx, "", textToStream(sentence))
		if err != nil {
			log.Printf("TTS transform error for sentence %d: %v", i, err)
			return
		}
		ttsStreams[i] = stream
	}

	combined := genx.CompositeSeq(ttsStreams...)
	eosToSilence := internal.NewEOSToSilence(2*time.Second, 16000, 1)
	withSilence, err := eosToSilence.Transform(ctx, combined)
	if err != nil {
		log.Printf("EOSToSilence transform error: %v", err)
		return
	}

	// Create audio track - groups audio by (role, mimetype, streamid) tuple
	audioPath := filepath.Join(*outputDir, "conversation.mp3")
	track := internal.NewAudioTrack(audioPath, 24000, 1) // 24kHz mono output

	// Tee input audio to track (user's voice from TTS)
	teedInput := internal.TeeToTrack(withSilence, track)

	fmt.Println("[3] Connecting to DashScope Realtime...")
	// DashScope Realtime tracks input StreamID and applies it to output
	output, err := realtime.Transform(ctx, "", teedInput)
	if err != nil {
		log.Printf("DashScope Realtime transform error: %v", err)
		return
	}

	// Tee output audio to track (AI's response)
	teedOutput := internal.TeeToTrack(output, track)

	fmt.Println("[4] Streaming output...")
	results := collectResults(teedOutput)

	// Save collected audio
	if err := track.Save(); err != nil {
		log.Printf("Failed to save audio: %v", err)
	} else {
		fmt.Printf("\nAudio saved: %s (%.2fs)\n", audioPath, track.Duration())

		// Verify with ASR if requested
		if *verify {
			verifyAudio(ctx, audioPath)
		}
	}

	// Print results
	printResults(testName, results, testSentences)
}

// runVoiceSwitchPipeline tests voice switching mid-conversation
func runVoiceSwitchPipeline(ctx context.Context, tts *transformers.MinimaxTTS, realtime *transformers.DashScopeRealtime, testName string, testSentences []string) {
	fmt.Println("[1] Building TTS streams...")

	// Build TTS streams for all sentences
	ttsStreams := make([]genx.Stream, len(testSentences))
	for i, sentence := range testSentences {
		stream, err := tts.Transform(ctx, "", textToStream(sentence))
		if err != nil {
			log.Printf("TTS transform error for sentence %d: %v", i, err)
			return
		}
		ttsStreams[i] = stream
	}

	// Combine all TTS streams
	combined := genx.CompositeSeq(ttsStreams...)
	eosToSilence := internal.NewEOSToSilence(2*time.Second, 16000, 1)
	withSilence, err := eosToSilence.Transform(ctx, combined)
	if err != nil {
		log.Printf("EOSToSilence transform error: %v", err)
		return
	}

	// Create audio track
	audioPath := filepath.Join(*outputDir, "voice_switch.mp3")
	track := internal.NewAudioTrack(audioPath, 24000, 1)

	// Tee input audio to track
	teedInput := internal.TeeToTrack(withSilence, track)

	fmt.Println("[2] Connecting to DashScope Realtime (voice: Chelsie)...")
	output, err := realtime.Transform(ctx, "", teedInput)
	if err != nil {
		log.Printf("DashScope Realtime transform error: %v", err)
		return
	}

	// Get DashScopeStream for voice switching
	dsStream, ok := output.(*transformers.DashScopeStream)
	if !ok {
		log.Printf("Warning: output is not *DashScopeStream")
		return
	}

	// Tee output audio to track
	teedOutput := internal.TeeToTrack(output, track)

	fmt.Println("[3] Streaming with voice switch after first response...")
	results := collectResultsWithVoiceSwitch(teedOutput, dsStream)

	// Save collected audio
	if err := track.Save(); err != nil {
		log.Printf("Failed to save audio: %v", err)
	} else {
		fmt.Printf("\nAudio saved: %s (%.2fs)\n", audioPath, track.Duration())
		fmt.Println("Listen to verify: first response=Chelsie, second response=Cherry")

		// Verify with ASR if requested
		if *verify {
			verifyAudio(ctx, audioPath)
		}
	}

	printResults(testName, results, testSentences)
}

// collectResultsWithVoiceSwitch collects results and switches voice after first audio EOS
func collectResultsWithVoiceSwitch(stream genx.Stream, dsStream *transformers.DashScopeStream) *Results {
	results := &Results{}

	var currentASRText strings.Builder
	var currentLLMText strings.Builder
	var currentAudioBytes int
	var currentAudioMIME string

	audioEOSCount := 0
	voiceSwitched := false

	for {
		chunk, err := stream.Next()
		if err != nil {
			if err != io.EOF {
				log.Printf("Stream error: %v", err)
			}
			break
		}

		if chunk == nil {
			continue
		}

		switch chunk.Role {
		case genx.RoleUser:
			// ASR result
			if text, ok := chunk.Part.(genx.Text); ok {
				currentASRText.WriteString(string(text))
				if chunk.Ctrl != nil && chunk.Ctrl.EndOfStream {
					results.ASRSegments = append(results.ASRSegments, Segment{
						Text:   currentASRText.String(),
						HasEOS: true,
					})
					fmt.Printf("  [ASR EOS] %s\n", truncate(currentASRText.String(), 60))
					currentASRText.Reset()
				}
			}

		case genx.RoleModel:
			if text, ok := chunk.Part.(genx.Text); ok {
				// Model text response
				currentLLMText.WriteString(string(text))
				if chunk.Ctrl != nil && chunk.Ctrl.EndOfStream {
					results.LLMSegments = append(results.LLMSegments, Segment{
						Text:   currentLLMText.String(),
						HasEOS: true,
					})
					fmt.Printf("  [LLM EOS] %s\n", truncate(currentLLMText.String(), 60))
					currentLLMText.Reset()
				}
			} else if blob, ok := chunk.Part.(*genx.Blob); ok {
				// Model audio response
				currentAudioBytes += len(blob.Data)
				if blob.MIMEType != "" {
					currentAudioMIME = blob.MIMEType
				}

				if chunk.Ctrl != nil && chunk.Ctrl.EndOfStream {
					results.AudioSegments = append(results.AudioSegments, AudioSegment{
						Bytes:    currentAudioBytes,
						MIMEType: currentAudioMIME,
						HasEOS:   true,
					})
					fmt.Printf("  [AUDIO EOS] %d bytes (%s)\n", currentAudioBytes, currentAudioMIME)
					currentAudioBytes = 0
					audioEOSCount++

					// Switch voice and character after first audio response completes
					if audioEOSCount == 1 && !voiceSwitched {
						fmt.Println("  [CHARACTER SWITCH] 普通助手 -> 东北大妈")
						newVoice := dashscope.VoiceCherry
						newInstructions := "你现在是一个热情的东北大妈，说话要带东北口音和特色，比如'哎呀妈呀'、'老妹儿'、'整挺好'这些词，性格爽朗热情，爱唠嗑。回答要简短有趣。"
						if err := dsStream.Update(&transformers.UpdateRequest{
							Voice:        &newVoice,
							Instructions: &newInstructions,
						}); err != nil {
							log.Printf("Update error: %v", err)
						}
						voiceSwitched = true
					}
				}
			}
		}
	}

	return results
}

// Results holds categorized output
type Results struct {
	ASRSegments   []Segment
	LLMSegments   []Segment
	AudioSegments []AudioSegment
}

type Segment struct {
	Text   string
	HasEOS bool
}

type AudioSegment struct {
	Bytes    int
	Duration float64 // seconds
	MIMEType string
	HasEOS   bool
}

func collectResults(stream genx.Stream) *Results {
	results := &Results{}

	var currentASRText strings.Builder
	var currentLLMText strings.Builder
	var currentAudioBytes int
	var currentAudioMIMEType string

	for {
		chunk, err := stream.Next()
		if err != nil {
			if err != io.EOF {
				log.Printf("Stream error: %v", err)
			}
			// Flush remaining
			if currentASRText.Len() > 0 {
				results.ASRSegments = append(results.ASRSegments, Segment{Text: currentASRText.String()})
			}
			if currentLLMText.Len() > 0 {
				results.LLMSegments = append(results.LLMSegments, Segment{Text: currentLLMText.String()})
			}
			if currentAudioBytes > 0 {
				results.AudioSegments = append(results.AudioSegments, AudioSegment{
					Bytes:    currentAudioBytes,
					Duration: estimateDuration(currentAudioBytes, currentAudioMIMEType),
					MIMEType: currentAudioMIMEType,
				})
			}
			break
		}

		if chunk == nil {
			continue
		}

		isEOS := chunk.IsEndOfStream()

		switch chunk.Role {
		case genx.RoleUser:
			if text, ok := chunk.Part.(genx.Text); ok {
				if isEOS {
					results.ASRSegments = append(results.ASRSegments, Segment{
						Text:   currentASRText.String(),
						HasEOS: true,
					})
					currentASRText.Reset()
					fmt.Printf("  [ASR EOS] %s\n", truncate(results.ASRSegments[len(results.ASRSegments)-1].Text, 50))
				} else {
					currentASRText.WriteString(string(text))
				}
			}

		case genx.RoleModel:
			if text, ok := chunk.Part.(genx.Text); ok {
				if isEOS {
					results.LLMSegments = append(results.LLMSegments, Segment{
						Text:   currentLLMText.String(),
						HasEOS: true,
					})
					currentLLMText.Reset()
					fmt.Printf("  [LLM EOS] %s\n", truncate(results.LLMSegments[len(results.LLMSegments)-1].Text, 50))
				} else {
					currentLLMText.WriteString(string(text))
				}
			} else if blob, ok := chunk.Part.(*genx.Blob); ok {
				if isEOS {
					duration := estimateDuration(currentAudioBytes, currentAudioMIMEType)
					results.AudioSegments = append(results.AudioSegments, AudioSegment{
						Bytes:    currentAudioBytes,
						Duration: duration,
						MIMEType: currentAudioMIMEType,
						HasEOS:   true,
					})
					fmt.Printf("  [AUDIO EOS] %.2fs\n", duration)
					currentAudioBytes = 0
					currentAudioMIMEType = ""
				} else {
					currentAudioBytes += len(blob.Data)
					if currentAudioMIMEType == "" {
						currentAudioMIMEType = blob.MIMEType
					}
				}
			}
		}
	}

	return results
}

// estimateDuration estimates audio duration from bytes and MIME type.
func estimateDuration(bytes int, mimeType string) float64 {
	switch mimeType {
	case "audio/pcm":
		// PCM 24kHz 16-bit mono = 48000 bytes/sec
		return float64(bytes) / 48000.0
	case "audio/mpeg", "audio/mp3":
		// MP3 ~128kbps = 16000 bytes/sec
		return float64(bytes) / 16000.0
	default:
		return 0
	}
}

func printResults(testName string, results *Results, testSentences []string) {
	fmt.Printf("\n=== %s Test - Conversation Results ===\n", testName)

	// Determine max turns
	maxTurns := len(testSentences)
	if len(results.LLMSegments) > maxTurns {
		maxTurns = len(results.LLMSegments)
	}
	if len(results.AudioSegments) > maxTurns {
		maxTurns = len(results.AudioSegments)
	}

	for i := 0; i < maxTurns; i++ {
		fmt.Printf("\n[Turn %d]\n", i+1)

		// Input
		if i < len(testSentences) {
			fmt.Printf("  Input:     %s\n", testSentences[i])
		} else {
			fmt.Printf("  Input:     (none)\n")
		}

		// ASR
		if i < len(results.ASRSegments) {
			fmt.Printf("  ASR:       %s\n", results.ASRSegments[i].Text)
		} else {
			fmt.Printf("  ASR:       (none)\n")
		}

		// LLM Text
		if i < len(results.LLMSegments) {
			fmt.Printf("  LLM_TEXT:  %s\n", truncate(results.LLMSegments[i].Text, 60))
		} else {
			fmt.Printf("  LLM_TEXT:  (none)\n")
		}

		// LLM Audio - show duration
		if i < len(results.AudioSegments) && results.AudioSegments[i].Bytes > 0 {
			fmt.Printf("  LLM_AUDIO: %.2fs\n", results.AudioSegments[i].Duration)
		} else {
			fmt.Printf("  LLM_AUDIO: (none)\n")
		}
	}

	// Summary
	fmt.Println("\n=== Summary ===")
	fmt.Printf("Total turns: %d\n", maxTurns)
	fmt.Printf("ASR segments: %d\n", len(results.ASRSegments))
	fmt.Printf("LLM segments: %d\n", len(results.LLMSegments))
	fmt.Printf("Audio segments: %d\n", len(results.AudioSegments))
}

// textToStream converts text to a single-chunk stream
func textToStream(text string) genx.Stream {
	return &singleTextStream{text: text}
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

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// verifyAudio uses ASR to verify the generated audio has no overlapping speech.
func verifyAudio(ctx context.Context, audioPath string) {
	fmt.Println("\n=== Verifying Audio with ASR ===")

	// Check for Doubao credentials
	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")
	if appID == "" || token == "" {
		fmt.Println("Skipping verification: DOUBAO_APP_ID or DOUBAO_TOKEN not set")
		return
	}

	// Read audio file
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		fmt.Printf("Failed to read audio: %v\n", err)
		return
	}

	// Create Doubao ASR client with Bearer token
	client := doubaospeech.NewClient(appID, doubaospeech.WithBearerToken(token))
	asr := transformers.NewDoubaoASRSAUC(client,
		transformers.WithDoubaoASRSAUCFormat("mp3"),
		transformers.WithDoubaoASRSAUCSampleRate(24000),
		transformers.WithDoubaoASRSAUCLanguage("zh-CN"),
		transformers.WithDoubaoASRSAUCEnablePunc(true),
	)

	// Create input stream from audio
	inputStream := &audioFileStream{
		data:   audioData,
		offset: 0,
	}

	// Run ASR
	output, err := asr.Transform(ctx, "", inputStream)
	if err != nil {
		fmt.Printf("ASR transform error: %v\n", err)
		return
	}

	// Collect results
	var text strings.Builder
	for {
		chunk, err := output.Next()
		if err != nil {
			if err != io.EOF {
				fmt.Printf("ASR error: %v\n", err)
			}
			break
		}
		if chunk != nil {
			if t, ok := chunk.Part.(genx.Text); ok && len(t) > 0 {
				text.WriteString(string(t))
			}
		}
	}

	fmt.Println("ASR Result:")
	fmt.Println(text.String())
	fmt.Println()
	fmt.Println("Check the above text for overlapping/repeated speech patterns.")
}

// audioFileStream wraps audio file data as a genx.Stream
type audioFileStream struct {
	data   []byte
	offset int
	done   bool
}

func (s *audioFileStream) Next() (*genx.MessageChunk, error) {
	if s.done {
		return nil, io.EOF
	}

	// Send audio in chunks
	chunkSize := 4096
	if s.offset >= len(s.data) {
		s.done = true
		// Return EOS marker
		return &genx.MessageChunk{
			Role: genx.RoleUser,
			Part: &genx.Blob{MIMEType: "audio/mp3"},
			Ctrl: &genx.StreamCtrl{EndOfStream: true},
		}, nil
	}

	end := s.offset + chunkSize
	if end > len(s.data) {
		end = len(s.data)
	}

	chunk := &genx.MessageChunk{
		Role: genx.RoleUser,
		Part: &genx.Blob{
			MIMEType: "audio/mp3",
			Data:     bytes.Clone(s.data[s.offset:end]),
		},
	}
	s.offset = end
	return chunk, nil
}

func (s *audioFileStream) Close() error {
	s.done = true
	return nil
}

func (s *audioFileStream) CloseWithError(err error) error {
	s.done = true
	return nil
}
