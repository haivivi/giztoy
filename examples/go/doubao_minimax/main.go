// Doubao Realtime + MiniMax Integration Test
//
// This example demonstrates multi-turn conversation testing using:
// 1. Doubao Realtime for Speech-to-Speech dialogue
// 2. MiniMax LLM for text generation (optional)
// 3. MiniMax TTS for speech synthesis (optional)
//
// Test scenarios:
// - Basic: Doubao Realtime full pipeline (ASR -> LLM -> TTS)
// - Hybrid: Doubao ASR -> MiniMax LLM -> MiniMax TTS
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	export MINIMAX_API_KEY="your_api_key"
//	export MINIMAX_GROUP_ID="your_group_id"  # optional
//	bazel run //examples/go/doubao_minimax
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	ds "github.com/haivivi/giztoy/pkg/doubaospeech"
	mm "github.com/haivivi/giztoy/pkg/minimax"
)

func main() {
	// Doubao configuration
	doubaoAppID := os.Getenv("DOUBAO_APP_ID")
	doubaoToken := os.Getenv("DOUBAO_TOKEN")

	// MiniMax configuration
	minimaxAPIKey := os.Getenv("MINIMAX_API_KEY")
	minimaxGroupID := os.Getenv("MINIMAX_GROUP_ID")

	if doubaoAppID == "" || doubaoToken == "" {
		fmt.Println("Please set DOUBAO_APP_ID and DOUBAO_TOKEN")
		os.Exit(1)
	}

	fmt.Println("========================================")
	fmt.Println("  Doubao Realtime + MiniMax Test")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Printf("Doubao App ID: %s\n", doubaoAppID)
	fmt.Printf("Doubao Token: %s...\n", truncate(doubaoToken, 10))

	// Test 1: Doubao Realtime basic test
	fmt.Println("\n[Test 1] Doubao Realtime Basic Test")
	fmt.Println("------------------------------------")
	testDoubaoRealtime(doubaoAppID, doubaoToken)

	// Test 2: Multi-turn conversation with Doubao Realtime
	fmt.Println("\n[Test 2] Multi-turn Conversation")
	fmt.Println("---------------------------------")
	testMultiTurnConversation(doubaoAppID, doubaoToken)

	// Test 3: Hybrid mode with MiniMax (if configured)
	if minimaxAPIKey != "" {
		fmt.Printf("\nMiniMax API Key: %s...\n", truncate(minimaxAPIKey, 10))
		if minimaxGroupID != "" {
			fmt.Printf("MiniMax Group ID: %s\n", minimaxGroupID)
		}

		fmt.Println("\n[Test 3] Hybrid Mode (Doubao + MiniMax)")
		fmt.Println("---------------------------------------")
		testHybridMode(doubaoAppID, doubaoToken, minimaxAPIKey, minimaxGroupID)
	} else {
		fmt.Println("\n[Test 3] Skipped (MINIMAX_API_KEY not set)")
	}

	fmt.Println("\n========================================")
	fmt.Println("  All tests completed!")
	fmt.Println("========================================")
}

// testDoubaoRealtime tests basic Doubao Realtime functionality
func testDoubaoRealtime(appID, token string) {
	client := ds.NewClient(appID, ds.WithBearerToken(token))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &ds.RealtimeConfig{
		TTS: ds.RealtimeTTSConfig{
			AudioConfig: ds.RealtimeAudioConfig{
				Channel:    1,
				Format:     "mp3",
				SampleRate: 24000,
			},
		},
		Dialog: ds.RealtimeDialogConfig{
			BotName:    "小豆",
			SystemRole: "你是一个友好的助手，回答要简短精炼",
		},
	}

	fmt.Println("  Connecting to Doubao Realtime...")
	session, err := client.Realtime.Connect(ctx, config)
	if err != nil {
		fmt.Printf("  ❌ Connection failed: %v\n", err)
		return
	}
	defer session.Close()
	fmt.Println("  ✅ Connected!")

	// Send greeting
	fmt.Println("  Sending: 你好，今天天气怎么样？")
	if err := session.SendText(ctx, "你好，今天天气怎么样？"); err != nil {
		fmt.Printf("  ❌ Send failed: %v\n", err)
		return
	}

	// Receive response
	var audioSize int
	var responseText string

	for event, err := range session.Recv() {
		if err != nil {
			break
		}

		audioSize += len(event.Audio)
		if event.Text != "" {
			responseText += event.Text
		}

		if event.Type == ds.EventTTSFinished {
			break
		}
	}

	fmt.Printf("  ✅ Response: %s\n", truncate(responseText, 50))
	fmt.Printf("  ✅ Audio received: %d bytes\n", audioSize)
}

// testMultiTurnConversation tests multi-turn dialogue
func testMultiTurnConversation(appID, token string) {
	client := ds.NewClient(appID, ds.WithBearerToken(token))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	config := &ds.RealtimeConfig{
		TTS: ds.RealtimeTTSConfig{
			AudioConfig: ds.RealtimeAudioConfig{
				Channel:    1,
				Format:     "mp3",
				SampleRate: 24000,
			},
		},
		Dialog: ds.RealtimeDialogConfig{
			BotName:    "小豆",
			SystemRole: "你是一个知识丰富的助手，擅长回答各种问题",
		},
	}

	fmt.Println("  Connecting...")
	session, err := client.Realtime.Connect(ctx, config)
	if err != nil {
		fmt.Printf("  ❌ Connection failed: %v\n", err)
		return
	}
	defer session.Close()
	fmt.Println("  ✅ Connected!")

	// Multi-turn conversation
	turns := []string{
		"你好，请用一句话介绍一下自己",
		"北京有哪些著名的景点？",
		"长城有多长？",
	}

	for i, question := range turns {
		fmt.Printf("\n  [Turn %d] User: %s\n", i+1, question)

		if err := session.SendText(ctx, question); err != nil {
			fmt.Printf("  ❌ Send failed: %v\n", err)
			return
		}

		var responseText strings.Builder
		var audioSize int

		for event, err := range session.Recv() {
			if err != nil {
				break
			}

			audioSize += len(event.Audio)
			if event.Text != "" {
				responseText.WriteString(event.Text)
			}

			if event.Type == ds.EventTTSFinished {
				break
			}
		}

		fmt.Printf("  [Turn %d] Bot: %s\n", i+1, truncate(responseText.String(), 100))
		fmt.Printf("  Audio: %d bytes\n", audioSize)
	}

	fmt.Println("\n  ✅ Multi-turn conversation completed!")
}

// testHybridMode tests hybrid mode: MiniMax TTS -> Doubao ASR -> MiniMax LLM -> MiniMax TTS
func testHybridMode(doubaoAppID, doubaoToken, minimaxAPIKey, minimaxGroupID string) {
	// Create MiniMax client
	mmClient := mm.NewClient(minimaxAPIKey)

	// Create Doubao client
	dsClient := ds.NewClient(doubaoAppID, ds.WithBearerToken(doubaoToken))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fmt.Println("  Full hybrid pipeline:")
	fmt.Println("  MiniMax TTS → Doubao ASR → MiniMax LLM → MiniMax TTS")
	fmt.Println()

	// Step 1: Use MiniMax TTS to generate audio for a question
	userQuestion := "明天北京的天气怎么样"
	fmt.Printf("  [Step 1] MiniMax TTS: Generating audio for '%s'\n", userQuestion)

	ttsResp1, err := mmClient.Speech.Synthesize(ctx, &mm.SpeechRequest{
		Model: "speech-01-turbo",
		Text:  userQuestion,
		VoiceSetting: &mm.VoiceSetting{
			VoiceID: "female-shaonv",
		},
		AudioSetting: &mm.AudioSetting{
			Format:     mm.AudioFormatPCM,
			SampleRate: 16000,
		},
	})
	if err != nil {
		fmt.Printf("  ❌ MiniMax TTS failed: %v\n", err)
		return
	}
	fmt.Printf("  ✅ Generated audio: %d bytes (PCM 16kHz)\n", len(ttsResp1.Audio))

	// Save the question audio
	os.MkdirAll("tmp", 0755)
	if err := os.WriteFile("tmp/hybrid_question.pcm", ttsResp1.Audio, 0644); err == nil {
		fmt.Println("  ✅ Saved: tmp/hybrid_question.pcm")
	}

	// Step 2: Use Doubao ASR to recognize the audio
	fmt.Println("\n  [Step 2] Doubao ASR: Recognizing audio...")

	asrResp, err := dsClient.ASR.RecognizeOneSentence(ctx, &ds.OneSentenceRequest{
		Audio:      ttsResp1.Audio,
		Format:     ds.FormatPCM,
		SampleRate: ds.SampleRate16000,
		Language:   ds.LanguageZhCN,
	})
	if err != nil {
		fmt.Printf("  ❌ Doubao ASR failed: %v\n", err)
		return
	}

	recognizedText := asrResp.Text
	fmt.Printf("  ✅ ASR Result: '%s'\n", recognizedText)

	// Step 3: Use MiniMax LLM to process the recognized text
	fmt.Println("\n  [Step 3] MiniMax LLM: Processing recognized text...")

	llmResp, err := mmClient.Text.CreateChatCompletion(ctx, &mm.ChatCompletionRequest{
		Model: "MiniMax-Text-01",
		Messages: []mm.Message{
			{Role: "system", Content: "你是一个天气助手，简短回答天气问题。回答不要超过50字。"},
			{Role: "user", Content: recognizedText},
		},
	})
	if err != nil {
		fmt.Printf("  ❌ MiniMax LLM failed: %v\n", err)
		return
	}

	llmText := ""
	if len(llmResp.Choices) > 0 {
		if content, ok := llmResp.Choices[0].Message.Content.(string); ok {
			llmText = content
		}
	}
	fmt.Printf("  ✅ LLM Response: '%s'\n", truncate(llmText, 100))

	// Step 4: Use MiniMax TTS to synthesize the response
	fmt.Println("\n  [Step 4] MiniMax TTS: Synthesizing response...")

	ttsResp2, err := mmClient.Speech.Synthesize(ctx, &mm.SpeechRequest{
		Model: "speech-01-turbo",
		Text:  llmText,
		VoiceSetting: &mm.VoiceSetting{
			VoiceID: "female-shaonv",
		},
		AudioSetting: &mm.AudioSetting{
			Format:     mm.AudioFormatMP3,
			SampleRate: 24000,
		},
	})
	if err != nil {
		fmt.Printf("  ❌ MiniMax TTS failed: %v\n", err)
		return
	}
	fmt.Printf("  ✅ Response audio: %d bytes\n", len(ttsResp2.Audio))

	if err := os.WriteFile("tmp/hybrid_response.mp3", ttsResp2.Audio, 0644); err == nil {
		fmt.Println("  ✅ Saved: tmp/hybrid_response.mp3")
	}

	fmt.Println("\n  ========================================")
	fmt.Println("  Hybrid Pipeline Summary:")
	fmt.Printf("  Input Question:    '%s'\n", userQuestion)
	fmt.Printf("  ASR Recognized:    '%s'\n", recognizedText)
	fmt.Printf("  LLM Response:      '%s'\n", truncate(llmText, 60))
	fmt.Println("  Output Audio:      tmp/hybrid_response.mp3")
	fmt.Println("  ========================================")

	fmt.Println("\n  ✅ Hybrid mode test completed!")
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
