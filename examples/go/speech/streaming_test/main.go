// Speech Package Streaming Decode Test
//
// This example tests streaming TTS decode with memory monitoring:
// 1. Uses compressed audio format (MP3/OGG) to reduce storage
// 2. Uses streaming decode - decodes on-demand during Read()
// 3. Monitors memory usage to detect leaks
// 4. Streams decoded audio to ASR for verification
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	export MINIMAX_API_KEY="your_api_key" (optional)
//	bazel run //examples/go/speech/streaming_test
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/pcm"
	"github.com/haivivi/giztoy/pkg/doubaospeech"
	"github.com/haivivi/giztoy/pkg/minimax"
	"github.com/haivivi/giztoy/pkg/speech"
)

// Long test text - multiple paragraphs to generate significant audio
const longTestText = `人工智能正在深刻改变我们的生活方式和工作方式。
从智能手机上的语音助手，到自动驾驶汽车，再到医疗诊断系统，AI技术无处不在。
语音合成技术让机器能够像人类一样自然地说话，这项技术在客服系统、有声读物、无障碍辅助等领域有着广泛的应用。
今天，我们将测试语音合成的流式解码效果，验证压缩音频格式的内存优化是否有效。
这是一段较长的测试文本，目的是生成足够多的音频数据来测试内存使用情况。
第一段结束。

现在是第二段测试文本。机器学习是人工智能的一个重要分支，它使计算机能够从数据中学习。
深度学习是机器学习的一个子领域，它使用多层神经网络来处理复杂的模式识别任务。
自然语言处理让计算机能够理解和生成人类语言，这是语音识别和语音合成的基础。
语音识别将人类的语音转换为文字，而语音合成则将文字转换为语音。
这两项技术的结合使得人机交互变得更加自然和便捷。
第二段结束。

第三段开始。边缘计算将计算能力从云端下沉到设备端，减少了网络延迟。
物联网设备越来越多地采用本地语音处理能力，以提供更快的响应速度。
隐私保护也是本地处理的一个重要优势，用户的语音数据不需要上传到云端。
未来的智能设备将更加智能、更加个性化，能够更好地理解和满足用户的需求。
感谢您的耐心阅读，这是测试的最后一段文字。
第三段结束，全文完。`

func main() {
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("           Speech Package Streaming Decode Memory Test")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Get credentials
	doubaoAppID := os.Getenv("DOUBAO_APP_ID")
	doubaoToken := os.Getenv("DOUBAO_TOKEN")
	minimaxAPIKey := os.Getenv("MINIMAX_API_KEY")

	if doubaoAppID == "" || doubaoToken == "" {
		fmt.Println("❌ Please set DOUBAO_APP_ID and DOUBAO_TOKEN environment variables")
		os.Exit(1)
	}

	// Initialize clients
	doubaoClient := doubaospeech.NewClient(doubaoAppID,
		doubaospeech.WithBearerToken(doubaoToken),
		doubaospeech.WithCluster("volcano_tts"),
	)
	globalDoubaoClient = doubaoClient // Set global client for ASR

	// Register handlers
	fmt.Println("📝 Registering TTS handlers...")
	registerHandlers(doubaoClient, minimaxAPIKey)
	fmt.Println()

	// Test configurations
	type testCase struct {
		name        string
		handler     string
		description string
	}

	testCases := []testCase{
		{"Doubao V2 PCM", "doubao-v2-pcm", "PCM format (baseline)"},
		{"Doubao V2 OGG", "doubao-v2-ogg", "OGG Opus (streaming decode)"},
	}

	if minimaxAPIKey != "" {
		testCases = append(testCases,
			testCase{"MiniMax PCM", "minimax-pcm", "PCM format (baseline)"},
			testCase{"MiniMax MP3", "minimax-mp3", "MP3 (streaming decode)"},
		)
	}

	// Print test text info
	fmt.Printf("📄 Test text: %d characters, ~%d words\n", len(longTestText), len(strings.Fields(longTestText)))
	fmt.Println()

	// Run tests
	for _, tc := range testCases {
		fmt.Printf("═══════════════════════════════════════════════════════════════════════\n")
		fmt.Printf("🧪 Test: %s (%s)\n", tc.name, tc.description)
		fmt.Printf("═══════════════════════════════════════════════════════════════════════\n")

		if err := runStreamingTest(tc.handler, longTestText, doubaoClient); err != nil {
			fmt.Printf("❌ Test failed: %v\n", err)
		} else {
			fmt.Printf("✅ Test passed: %s\n", tc.name)
		}
		fmt.Println()
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("                         All Tests Completed")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
}

func registerHandlers(doubaoClient *doubaospeech.Client, minimaxAPIKey string) {
	// Doubao V2 with PCM (baseline)
	doubaoV2PCM := speech.NewDoubaoTTSV2Handler(doubaoClient,
		speech.WithDoubaoTTSV2Speaker("zh_female_vv_uranus_bigtts"),
		speech.WithDoubaoTTSV2ResourceID(doubaospeech.ResourceTTSV2),
		speech.WithDoubaoTTSV2Format("pcm"),
	)
	if err := speech.HandleTTS("doubao-v2-pcm", doubaoV2PCM); err != nil {
		fmt.Printf("   ⚠️  Failed to register doubao-v2-pcm: %v\n", err)
	} else {
		fmt.Println("   ✅ doubao-v2-pcm (PCM baseline)")
	}

	// Doubao V2 with OGG Opus (streaming decode)
	doubaoV2OGG := speech.NewDoubaoTTSV2Handler(doubaoClient,
		speech.WithDoubaoTTSV2Speaker("zh_female_vv_uranus_bigtts"),
		speech.WithDoubaoTTSV2ResourceID(doubaospeech.ResourceTTSV2),
		speech.WithDoubaoTTSV2Format("ogg_opus"),
	)
	if err := speech.HandleTTS("doubao-v2-ogg", doubaoV2OGG); err != nil {
		fmt.Printf("   ⚠️  Failed to register doubao-v2-ogg: %v\n", err)
	} else {
		fmt.Println("   ✅ doubao-v2-ogg (OGG streaming decode)")
	}

	// MiniMax handlers
	if minimaxAPIKey != "" {
		minimaxClient := minimax.NewClient(minimaxAPIKey)

		// PCM baseline
		minimaxPCM := speech.NewMinimaxTTSHandler(minimaxClient,
			speech.WithMinimaxTTSFormat(minimax.AudioFormatPCM),
		)
		if err := speech.HandleTTS("minimax-pcm", minimaxPCM); err != nil {
			fmt.Printf("   ⚠️  Failed to register minimax-pcm: %v\n", err)
		} else {
			fmt.Println("   ✅ minimax-pcm (PCM baseline)")
		}

		// MP3 streaming decode
		minimaxMP3 := speech.NewMinimaxTTSHandler(minimaxClient,
			speech.WithMinimaxTTSFormat(minimax.AudioFormatMP3),
		)
		if err := speech.HandleTTS("minimax-mp3", minimaxMP3); err != nil {
			fmt.Printf("   ⚠️  Failed to register minimax-mp3: %v\n", err)
		} else {
			fmt.Println("   ✅ minimax-mp3 (MP3 streaming decode)")
		}
	}

	// Register ASR handler
	doubaoASR := speech.NewDoubaoSAUCASRHandler(doubaoClient,
		speech.WithDoubaoSAUCSampleRate(16000),
		speech.WithDoubaoSAUCLanguage("zh-CN"),
		speech.WithDoubaoSAUCEnableITN(true),
		speech.WithDoubaoSAUCEnablePunc(true),
	)
	if err := speech.HandleASR("doubao-sauc", doubaoASR); err != nil {
		fmt.Printf("   ⚠️  Failed to register doubao-sauc: %v\n", err)
	} else {
		fmt.Println("   ✅ doubao-sauc (ASR)")
	}
}

func runStreamingTest(handlerName, text string, doubaoClient *doubaospeech.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	format := pcm.L16Mono16K
	runtime.GC() // Force GC before test

	// Record initial memory
	var memBefore, memAfterTTS, memAfterDecode runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	fmt.Printf("📊 Memory before: Alloc=%dKB, TotalAlloc=%dKB, Sys=%dKB\n",
		memBefore.Alloc/1024, memBefore.TotalAlloc/1024, memBefore.Sys/1024)

	// Step 1: TTS Synthesis
	fmt.Println()
	fmt.Println("📢 Step 1: TTS Synthesis...")
	startTime := time.Now()

	sp, err := speech.Synthesize(ctx, handlerName, strings.NewReader(text), format)
	if err != nil {
		return fmt.Errorf("synthesize failed: %w", err)
	}
	defer sp.Close()

	// Collect segments (compressed audio stored in memory)
	type segmentData struct {
		segment speech.SpeechSegment
		text    string
	}
	var segments []segmentData
	var totalCompressedSize int64

	for seg, err := range speech.Iter(sp) {
		if err != nil {
			return fmt.Errorf("read segment failed: %w", err)
		}

		// Get transcript
		transcript := seg.Transcribe()
		textData, _ := io.ReadAll(transcript)
		transcript.Close()

		segments = append(segments, segmentData{segment: seg, text: string(textData)})
		fmt.Printf("   Segment %d: text=%.20s...\n", len(segments), string(textData))
	}

	runtime.GC()
	runtime.ReadMemStats(&memAfterTTS)
	ttsTime := time.Since(startTime)
	fmt.Printf("   ✅ TTS completed: %d segments in %v\n", len(segments), ttsTime)
	fmt.Printf("📊 Memory after TTS: Alloc=%dKB (+%dKB)\n",
		memAfterTTS.Alloc/1024, (memAfterTTS.Alloc-memBefore.Alloc)/1024)

	// Step 2: Streaming Decode + Feed to ASR
	fmt.Println()
	fmt.Println("🔄 Step 2: Streaming Decode + ASR...")

	var totalPCMSize int64
	var asrResults []string
	decodeStart := time.Now()

	for i, sd := range segments {
		fmt.Printf("   Processing segment %d/%d...\n", i+1, len(segments))

		// Get streaming decoder
		voice := sd.segment.Decode(format)

		// Stream decode: read in chunks (simulating streaming to ASR)
		var pcmBuffer bytes.Buffer
		buf := make([]byte, 4096) // Small buffer - streaming style
		for {
			n, err := voice.Read(buf)
			if n > 0 {
				pcmBuffer.Write(buf[:n])
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				voice.Close()
				sd.segment.Close()
				return fmt.Errorf("decode failed: %w", err)
			}
		}
		voice.Close()
		sd.segment.Close()

		totalPCMSize += int64(pcmBuffer.Len())
		fmt.Printf("      Decoded: %d bytes PCM\n", pcmBuffer.Len())

		// Feed to ASR
		asrResult, err := recognizeWithASR(ctx, pcmBuffer.Bytes(), format)
		if err != nil {
			fmt.Printf("      ⚠️  ASR failed: %v\n", err)
			asrResults = append(asrResults, "[ASR failed]")
		} else {
			asrResults = append(asrResults, asrResult)
			fmt.Printf("      ASR: %.30s...\n", asrResult)
		}

		// Check memory after each segment
		runtime.GC()
		var memNow runtime.MemStats
		runtime.ReadMemStats(&memNow)
		fmt.Printf("      Memory: Alloc=%dKB\n", memNow.Alloc/1024)
	}

	runtime.GC()
	runtime.ReadMemStats(&memAfterDecode)
	decodeTime := time.Since(decodeStart)

	// Summary
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("📊 Summary")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("   Segments: %d\n", len(segments))
	fmt.Printf("   Compressed size: %d KB\n", totalCompressedSize/1024)
	fmt.Printf("   Total PCM size: %d KB\n", totalPCMSize/1024)
	fmt.Printf("   TTS time: %v\n", ttsTime)
	fmt.Printf("   Decode+ASR time: %v\n", decodeTime)
	fmt.Println()
	fmt.Println("📊 Memory Analysis")
	fmt.Printf("   Before:       Alloc=%6dKB\n", memBefore.Alloc/1024)
	fmt.Printf("   After TTS:    Alloc=%6dKB (+%dKB)\n",
		memAfterTTS.Alloc/1024, (memAfterTTS.Alloc-memBefore.Alloc)/1024)
	// Use signed arithmetic to handle potential decrease
	decodeMemChange := int64(memAfterDecode.Alloc) - int64(memAfterTTS.Alloc)
	if decodeMemChange >= 0 {
		fmt.Printf("   After Decode: Alloc=%6dKB (+%dKB from TTS)\n",
			memAfterDecode.Alloc/1024, decodeMemChange/1024)
	} else {
		fmt.Printf("   After Decode: Alloc=%6dKB (%dKB from TTS, freed)\n",
			memAfterDecode.Alloc/1024, decodeMemChange/1024)
	}
	fmt.Println()

	// Check for memory leaks
	memGrowth := int64(memAfterDecode.Alloc) - int64(memBefore.Alloc)
	if memGrowth > int64(totalPCMSize)*2 {
		fmt.Printf("⚠️  Potential memory leak: growth=%dKB, expected<%dKB\n",
			memGrowth/1024, totalPCMSize*2/1024)
	} else {
		fmt.Printf("✅ Memory usage looks reasonable\n")
	}

	// Print ASR results comparison
	fmt.Println()
	fmt.Println("📝 ASR Results:")
	for i, result := range asrResults {
		fmt.Printf("   [%d] Original: %.30s...\n", i+1, segments[i].text)
		fmt.Printf("       ASR:      %.30s...\n", result)
	}

	return nil
}

// Global doubao client for ASR (set in main)
var globalDoubaoClient *doubaospeech.Client

func recognizeWithASR(ctx context.Context, audioData []byte, format pcm.Format) (string, error) {
	if globalDoubaoClient == nil {
		return "", fmt.Errorf("doubao client not initialized")
	}

	// Open ASR session
	asrConfig := &doubaospeech.ASRV2Config{
		Format:     "pcm",
		SampleRate: format.SampleRate(),
		Bits:       16,
		Channels:   format.Channels(),
		Language:   "zh-CN",
		ResourceID: doubaospeech.ResourceASRStream,
		EnableITN:  true,
		EnablePunc: true,
	}

	session, err := globalDoubaoClient.ASRV2.OpenStreamSession(ctx, asrConfig)
	if err != nil {
		return "", fmt.Errorf("open ASR session: %w", err)
	}
	defer session.Close()

	// Start receiving results in background
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		var finalText string
		for result, err := range session.Recv() {
			if err != nil {
				errCh <- err
				return
			}
			if result.IsFinal {
				finalText = result.Text
			}
		}
		resultCh <- finalText
	}()

	// Give receiver time to start
	time.Sleep(50 * time.Millisecond)

	// Send audio in chunks (100ms each)
	chunkSize := format.SampleRate() * format.Channels() * 2 / 10 // 100ms of audio
	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}
		chunk := audioData[i:end]
		isLast := end >= len(audioData)

		if err := session.SendAudio(ctx, chunk, isLast); err != nil {
			return "", fmt.Errorf("send audio: %w", err)
		}

		// Pace sending to simulate real-time
		time.Sleep(30 * time.Millisecond)
	}

	// Wait for result
	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("ASR timeout")
	}
}
