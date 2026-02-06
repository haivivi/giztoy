// Package main demonstrates two DashScope Realtime AI agents having a conversation.
//
// Architecture: Pure streaming pipe with Tee for audio recording.
//
//	TTS -> bufA -> AI_A -> Tee(Track) -> filter(audio) -> bufB -> AI_B -> Tee(Track) -> filter(audio) -> bufA
//
// Both AIs run concurrently, exchanging audio in a streaming fashion.
//
// Usage:
//
//	bazel run //examples/go/genx/transformers/dashscope_realtime_chat -- -rounds=20
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
	"github.com/haivivi/giztoy/go/pkg/audio/portaudio"
	"github.com/haivivi/giztoy/go/pkg/audio/resampler"
	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/dashscope"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
	"github.com/haivivi/giztoy/examples/go/genx/transformers/internal"
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

var (
	rounds  = flag.Int("rounds", 4, "Number of conversation rounds")
	timeout = flag.Duration("timeout", 10*time.Minute, "Overall timeout")
)

// AI personas
const (
	instructionsA = `你是一个热情的东北大妈，名叫王大姐。说话要带东北口音和特色，比如"哎呀妈呀"、"老妹儿"、"整挺好"、"嘎哈呢"这些词。性格爽朗热情，爱唠嗑，喜欢聊家常。回答要简短有趣，每次说1-2句话就行。`

	instructionsB = `你是一个温柔的上海小姐姐，名叫小云。说话要带上海腔调，偶尔用"阿拉"、"侬"、"老好的"、"嗲"这些词。性格温柔细腻，说话轻声细语。回答要简短优雅，每次说1-2句话就行。`

	initialMessage = "你好呀，我是小云，今天天气真好，出来逛逛呀？"
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("=== DashScope Realtime Chat ===")
	fmt.Printf("Rounds:  %d\n", *rounds)
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

	// Create TTS for initial message - output PCM 16kHz for DashScope
	tts := transformers.NewMinimaxTTS(mmClient, "female-shaonv",
		transformers.WithMinimaxTTSFormat("pcm"),
		transformers.WithMinimaxTTSSampleRate(16000),
	)

	// Create two AI agents in manual mode (no VAD)
	// Use PCM16 for both input and output (compatible with AudioTrack)
	// Note: DashScope input is 16kHz, output is 24kHz
	aiA := transformers.NewDashScopeRealtime(dsClient,
		transformers.WithDashScopeRealtimeVoice(dashscope.VoiceCherry),
		transformers.WithDashScopeRealtimeInstructions(instructionsA),
	)

	aiB := transformers.NewDashScopeRealtime(dsClient,
		transformers.WithDashScopeRealtimeVoice(dashscope.VoiceChelsie),
		transformers.WithDashScopeRealtimeInstructions(instructionsB),
	)

	// Run the chat
	runChat(ctx, tts, aiA, aiB)
}

func runChat(ctx context.Context, tts *transformers.MinimaxTTS, aiA, aiB *transformers.DashScopeRealtime) {
	// Initialize portaudio
	if err := portaudio.Initialize(); err != nil {
		log.Fatalf("Failed to initialize portaudio: %v", err)
	}
	defer portaudio.Terminate()

	// Create output stream for playback (24kHz mono, 20ms buffer)
	speaker, err := portaudio.NewOutputStream(pcm.L16Mono24K, 20*time.Millisecond)
	if err != nil {
		log.Fatalf("Failed to create output stream: %v", err)
	}
	defer speaker.Close()

	// Audio player that resamples and plays (writes in chunks)
	// Buffer size: 20ms @ 24kHz = 480 samples = 960 bytes
	const chunkSize = 960
	playAudio := func(data []byte, srcRate int) {
		if len(data) == 0 {
			return
		}
		// Resample to 24kHz if needed
		if srcRate != 24000 {
			resampled, err := resamplePCM(data, srcRate, 24000)
			if err != nil {
				log.Printf("Resample error: %v", err)
				return
			}
			data = resampled
		}
		// Write in chunks to ensure all data is played
		for len(data) > 0 {
			n := chunkSize
			if n > len(data) {
				n = len(data)
			}
			speaker.WriteBytes(data[:n])
			data = data[n:]
		}
	}

	// Create audio track (no file output, just for tracking)
	track := internal.NewAudioTrack("", 24000, 1)

	// Create buffer streams as inputs for each AI
	bufA := newBufferStream(100)
	bufB := newBufferStream(100)

	// Generate initial TTS message
	fmt.Println("[1] Generating initial message with TTS...")
	initialStream := textToStream(initialMessage)
	ttsStream, err := tts.Transform(ctx, "", initialStream)
	if err != nil {
		log.Fatalf("TTS error: %v", err)
	}

	// Connect AI_A (东北大妈)
	fmt.Println("[2] Connecting AI_A (东北大妈 - Cherry)...")
	streamA, err := aiA.Transform(ctx, "", bufA)
	if err != nil {
		log.Fatalf("AI_A connect error: %v", err)
	}

	// Connect AI_B (上海小姐姐)
	fmt.Println("[3] Connecting AI_B (上海小姐姐 - Chelsie)...")
	streamB, err := aiB.Transform(ctx, "", bufB)
	if err != nil {
		log.Fatalf("AI_B connect error: %v", err)
	}

	fmt.Printf("[4] Starting conversation (%d rounds)...\n\n", *rounds)

	var wg sync.WaitGroup
	roundsA := 0
	roundsB := 0
	maxRounds := *rounds

	// Goroutine: Send initial TTS to AI_A
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			chunk, err := ttsStream.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("TTS read error: %v", err)
				break
			}
			if chunk != nil {
				// Play TTS audio (16kHz)
				if blob, ok := chunk.Part.(*genx.Blob); ok && len(blob.Data) > 0 {
					playAudio(blob.Data, 16000)
				}
				// Tee to track (user audio)
				track.HandleChunk(chunk)
				// Send to AI_A
				if err := bufA.Write(chunk); err != nil {
					break
				}
			}
		}
	}()

	// Goroutine: AI_A output -> Tee to Track -> filter audio -> pipe to bufB
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer bufB.Close()

		currentStreamID := ""
		sentBOS := false

		for {
			chunk, err := streamA.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("AI_A read error: %v", err)
				break
			}
			if chunk == nil {
				continue
			}

			// Tee all chunks to track
			track.HandleChunk(chunk)

			// Filter and pipe audio to AI_B (including EOS)
			if isAudioChunk(chunk) || isAudioEOS(chunk) {
				// Play AI_A audio (24kHz)
				if blob, ok := chunk.Part.(*genx.Blob); ok && len(blob.Data) > 0 {
					playAudio(blob.Data, 24000)
				}

				// Generate StreamID once per turn
				if currentStreamID == "" {
					currentStreamID = genx.NewStreamID()
				}

				isEOS := isAudioEOS(chunk)

				// Resample audio from 24kHz (DashScope output) to 16kHz (DashScope input)
				var audioPart genx.Part = chunk.Part
				if blob, ok := chunk.Part.(*genx.Blob); ok && len(blob.Data) > 0 {
					resampled, err := resamplePCM(blob.Data, 24000, 16000)
					if err != nil {
						log.Printf("Resample error: %v", err)
					} else {
						audioPart = &genx.Blob{MIMEType: blob.MIMEType, Data: resampled}
					}
				}

				// Clone chunk and modify for AI_B
				outChunk := &genx.MessageChunk{
					Role: genx.RoleUser,
					Part: audioPart,
					Ctrl: &genx.StreamCtrl{
						StreamID:      currentStreamID,
						BeginOfStream: !sentBOS,
						EndOfStream:   isEOS,
					},
				}
				sentBOS = true

				if err := bufB.Write(outChunk); err != nil {
					break
				}

				// Check for audio EOS (round complete)
				if isEOS {
					roundsA++
					fmt.Printf("  [AI_A Round %d] completed\n", roundsA)
					// Reset for next turn
					currentStreamID = ""
					sentBOS = false
					if roundsA >= maxRounds {
						fmt.Println("  [AI_A] Max rounds reached, stopping")
						return
					}
				}
			}
		}
	}()

	// Goroutine: AI_B output -> Tee to Track -> filter audio -> pipe to bufA
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer bufA.Close()

		currentStreamID := ""
		sentBOS := false

		for {
			chunk, err := streamB.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("AI_B read error: %v", err)
				break
			}
			if chunk == nil {
				continue
			}

			// Tee all chunks to track
			track.HandleChunk(chunk)

			// Filter and pipe audio to AI_A (including EOS)
			if isAudioChunk(chunk) || isAudioEOS(chunk) {
				// Play AI_B audio (24kHz)
				if blob, ok := chunk.Part.(*genx.Blob); ok && len(blob.Data) > 0 {
					playAudio(blob.Data, 24000)
				}

				// Generate StreamID once per turn
				if currentStreamID == "" {
					currentStreamID = genx.NewStreamID()
				}

				isEOS := isAudioEOS(chunk)

				// Resample audio from 24kHz (DashScope output) to 16kHz (DashScope input)
				var audioPart genx.Part = chunk.Part
				if blob, ok := chunk.Part.(*genx.Blob); ok && len(blob.Data) > 0 {
					resampled, err := resamplePCM(blob.Data, 24000, 16000)
					if err != nil {
						log.Printf("Resample error: %v", err)
					} else {
						audioPart = &genx.Blob{MIMEType: blob.MIMEType, Data: resampled}
					}
				}

				// Clone chunk and modify for AI_A
				outChunk := &genx.MessageChunk{
					Role: genx.RoleUser,
					Part: audioPart,
					Ctrl: &genx.StreamCtrl{
						StreamID:      currentStreamID,
						BeginOfStream: !sentBOS,
						EndOfStream:   isEOS,
					},
				}
				sentBOS = true

				if err := bufA.Write(outChunk); err != nil {
					break
				}

				// Check for audio EOS (round complete)
				if isEOS {
					roundsB++
					fmt.Printf("  [AI_B Round %d] completed\n", roundsB)
					// Reset for next turn
					currentStreamID = ""
					sentBOS = false
					if roundsB >= maxRounds {
						fmt.Println("  [AI_B] Max rounds reached, stopping")
						return
					}
				}
			}
		}
	}()

	// Wait for all goroutines
	wg.Wait()

	fmt.Printf("\n=== Chat Complete ===\n")
	fmt.Printf("AI_A rounds: %d\n", roundsA)
	fmt.Printf("AI_B rounds: %d\n", roundsB)
	fmt.Printf("Duration: %.2fs\n", track.Duration())
}

// textToStream converts text to a genx stream
func textToStream(text string) genx.Stream {
	buf := newBufferStream(10)
	go func() {
		defer buf.Close()
		buf.Write(&genx.MessageChunk{
			Role: genx.RoleUser,
			Part: genx.Text(text),
			Ctrl: &genx.StreamCtrl{
				StreamID:      genx.NewStreamID(),
				BeginOfStream: true,
			},
		})
		buf.Write(&genx.MessageChunk{
			Role: genx.RoleUser,
			Part: genx.Text(""),
			Ctrl: &genx.StreamCtrl{
				EndOfStream: true,
			},
		})
	}()
	return buf
}

// bufferStream wraps buffer.Buffer to implement genx.Stream
type bufferStream struct {
	buf *buffer.Buffer[*genx.MessageChunk]
}

func newBufferStream(size int) *bufferStream {
	return &bufferStream{buf: buffer.N[*genx.MessageChunk](size)}
}

func (s *bufferStream) Next() (*genx.MessageChunk, error) {
	chunk, err := s.buf.Next()
	if err != nil {
		if err == buffer.ErrIteratorDone {
			return nil, io.EOF
		}
		return nil, err
	}
	return chunk, nil
}

func (s *bufferStream) Close() error {
	return s.buf.CloseWrite()
}

func (s *bufferStream) CloseWithError(err error) error {
	return s.buf.CloseWithError(err)
}

func (s *bufferStream) Write(chunk *genx.MessageChunk) error {
	return s.buf.Add(chunk)
}

// isAudioChunk checks if chunk contains audio data
func isAudioChunk(chunk *genx.MessageChunk) bool {
	if chunk == nil {
		return false
	}
	blob, ok := chunk.Part.(*genx.Blob)
	if !ok {
		return false
	}
	return internal.IsAudioMIME(blob.MIMEType) && len(blob.Data) > 0
}

// isAudioEOS checks if chunk is an audio end-of-stream marker
func isAudioEOS(chunk *genx.MessageChunk) bool {
	if chunk == nil || chunk.Ctrl == nil || !chunk.Ctrl.EndOfStream {
		return false
	}
	blob, ok := chunk.Part.(*genx.Blob)
	if !ok {
		return false
	}
	return internal.IsAudioMIME(blob.MIMEType)
}

// resamplePCM converts PCM from one sample rate to another
func resamplePCM(data []byte, fromRate, toRate int) ([]byte, error) {
	if len(data) == 0 || fromRate == toRate {
		return data, nil
	}
	srcFmt := resampler.Format{SampleRate: fromRate, Stereo: false}
	dstFmt := resampler.Format{SampleRate: toRate, Stereo: false}

	rs, err := resampler.New(bytes.NewReader(data), srcFmt, dstFmt)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	var out bytes.Buffer
	_, err = io.Copy(&out, rs)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return out.Bytes(), nil
}
