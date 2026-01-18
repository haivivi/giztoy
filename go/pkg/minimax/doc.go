// Package minimax provides a Go client for the MiniMax API.
//
// This package provides HTTP communication with MiniMax API endpoints
// for text generation, speech synthesis, video generation, and more.
//
// # Basic Usage
//
//	client := minimax.NewClient("your-api-key")
//
//	// Text generation
//	resp, err := client.Text.CreateChatCompletion(ctx, &minimax.ChatCompletionRequest{
//	    Model: "MiniMax-M2.1",
//	    Messages: []minimax.Message{
//	        {Role: "user", Content: "Hello!"},
//	    },
//	})
//
//	// Speech synthesis
//	resp, err := client.Speech.Synthesize(ctx, &minimax.SpeechRequest{
//	    Model: "speech-2.6-hd",
//	    Text:  "Hello, world!",
//	    VoiceSetting: &minimax.VoiceSetting{
//	        VoiceID: "male-qn-qingse",
//	    },
//	})
//
// # Streaming
//
// Streaming methods return iter.Seq2 iterators that can be used with Go 1.23+ range:
//
//	for chunk, err := range client.Text.CreateChatCompletionStream(ctx, req) {
//	    if err != nil {
//	        return err
//	    }
//	    if len(chunk.Choices) > 0 {
//	        fmt.Print(chunk.Choices[0].Delta.Content)
//	    }
//	}
//
// # Async Tasks
//
// Long-running operations return Task objects that can be polled:
//
//	task, err := client.Video.CreateTextToVideo(ctx, req)
//	if err != nil {
//	    return err
//	}
//
//	// Wait with default 5s polling interval
//	result, err := task.Wait(ctx)
//
//	// Or with custom interval
//	result, err := task.WaitWithInterval(ctx, 10*time.Second)
//
// # Error Handling
//
//	resp, err := client.Text.CreateChatCompletion(ctx, req)
//	if err != nil {
//	    if e, ok := minimax.AsError(err); ok {
//	        if e.IsRateLimit() {
//	            // Handle rate limiting
//	        }
//	    }
//	    return err
//	}
//
// # Configuration
//
//	client := minimax.NewClient("api-key",
//	    minimax.WithBaseURL("https://api.minimax.chat"),
//	    minimax.WithRetry(3),
//	)
//
// Request timeouts should be controlled via context.Context:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	result, err := client.Text.CreateChatCompletion(ctx, req)
//
// For more information, see: https://platform.minimaxi.com/docs
package minimax
