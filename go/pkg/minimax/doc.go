// Package minimax provides a Go client for the MiniMax API.
//
// This package implements the interfaces defined in minimax_interface package,
// providing actual HTTP communication with MiniMax API endpoints.
//
// # Basic Usage
//
//	client := minimax.NewClient("your-api-key")
//
//	// Text generation
//	resp, err := client.Text.CreateChatCompletion(ctx, &minimax_interface.ChatCompletionRequest{
//	    Model: "abab6.5s-chat",
//	    Messages: []minimax_interface.Message{
//	        {Role: "user", Content: "Hello!"},
//	    },
//	})
//
//	// Speech synthesis
//	resp, err := client.Speech.Synthesize(ctx, &minimax_interface.SpeechRequest{
//	    Model: "speech-01-turbo",
//	    Text:  "Hello, world!",
//	    VoiceSetting: &minimax_interface.VoiceSetting{
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
//	    fmt.Print(chunk.Delta.Content)
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
//	    if e, ok := minimax_interface.AsError(err); ok {
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
//	    minimax.WithTimeout(30*time.Second),
//	    minimax.WithRetry(3),
//	)
//
// For more information, see: https://platform.minimaxi.com/docs
package minimax
