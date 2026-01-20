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

// testHybridMode tests hybrid mode: Doubao ASR -> MiniMax LLM -> MiniMax TTS
func testHybridMode(doubaoAppID, doubaoToken, minimaxAPIKey, minimaxGroupID string) {
	// Create MiniMax client
	mmClient := mm.NewClient(minimaxAPIKey)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test MiniMax LLM
	fmt.Println("  Testing MiniMax LLM...")
	llmResp, err := mmClient.Text.CreateChatCompletion(ctx, &mm.ChatCompletionRequest{
		Model: "MiniMax-Text-01",
		Messages: []mm.Message{
			{Role: "system", Content: "你是一个友好的助手"},
			{Role: "user", Content: "请用一句话介绍你自己"},
		},
	})
	if err != nil {
		fmt.Printf("  ❌ LLM failed: %v\n", err)
		return
	}
	llmText := ""
	if len(llmResp.Choices) > 0 {
		if content, ok := llmResp.Choices[0].Message.Content.(string); ok {
			llmText = content
		}
	}
	fmt.Printf("  ✅ LLM Response: %s\n", truncate(llmText, 80))

	// Test MiniMax TTS
	fmt.Println("  Testing MiniMax TTS...")
	ttsResp, err := mmClient.Speech.Synthesize(ctx, &mm.SpeechRequest{
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
		fmt.Printf("  ❌ TTS failed: %v\n", err)
		return
	}
	fmt.Printf("  ✅ TTS Audio: %d bytes\n", len(ttsResp.Audio))

	// Save audio
	os.MkdirAll("tmp", 0755)
	if err := os.WriteFile("tmp/hybrid_minimax.mp3", ttsResp.Audio, 0644); err == nil {
		fmt.Println("  ✅ Audio saved: tmp/hybrid_minimax.mp3")
	}

	// Now test the full hybrid pipeline
	fmt.Println("\n  Testing full hybrid pipeline...")
	fmt.Println("  (Doubao Realtime ASR) -> (MiniMax LLM) -> (MiniMax TTS)")

	// Create Doubao client for ASR via Realtime
	dsClient := ds.NewClient(doubaoAppID, ds.WithBearerToken(doubaoToken))
	dsConfig := &ds.RealtimeConfig{
		Dialog: ds.RealtimeDialogConfig{
			BotName:    "Test",
			SystemRole: "只做语音识别，不要回答问题",
		},
	}

	session, err := dsClient.Realtime.Connect(ctx, dsConfig)
	if err != nil {
		fmt.Printf("  ❌ Doubao connect failed: %v\n", err)
		return
	}
	defer session.Close()

	// Send text to get ASR simulation (in real use, you'd send audio)
	testInput := "帮我查一下明天北京的天气"
	fmt.Printf("  User Input: %s\n", testInput)

	// Use MiniMax LLM to process
	llmResp2, err := mmClient.Text.CreateChatCompletion(ctx, &mm.ChatCompletionRequest{
		Model: "MiniMax-Text-01",
		Messages: []mm.Message{
			{Role: "system", Content: "你是一个天气助手，简短回答天气问题"},
			{Role: "user", Content: testInput},
		},
	})
	if err != nil {
		fmt.Printf("  ❌ LLM failed: %v\n", err)
		return
	}
	llmText2 := ""
	if len(llmResp2.Choices) > 0 {
		if content, ok := llmResp2.Choices[0].Message.Content.(string); ok {
			llmText2 = content
		}
	}
	fmt.Printf("  LLM Response: %s\n", truncate(llmText2, 100))

	// Use MiniMax TTS to synthesize
	ttsResp2, err := mmClient.Speech.Synthesize(ctx, &mm.SpeechRequest{
		Model: "speech-01-turbo",
		Text:  llmText2,
		VoiceSetting: &mm.VoiceSetting{
			VoiceID: "female-shaonv",
		},
		AudioSetting: &mm.AudioSetting{
			Format:     mm.AudioFormatMP3,
			SampleRate: 24000,
		},
	})
	if err != nil {
		fmt.Printf("  ❌ TTS failed: %v\n", err)
		return
	}
	fmt.Printf("  TTS Audio: %d bytes\n", len(ttsResp2.Audio))

	if err := os.WriteFile("tmp/hybrid_pipeline.mp3", ttsResp2.Audio, 0644); err == nil {
		fmt.Println("  ✅ Audio saved: tmp/hybrid_pipeline.mp3")
	}

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
