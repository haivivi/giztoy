// Package dashscope provides a Go client for Aliyun DashScope (Model Studio) APIs.
//
// This package implements the Qwen-Omni-Realtime API for real-time multimodal
// conversations over WebSocket.
//
// # Quick Start
//
//	client := dashscope.NewClient("your-api-key")
//	session, err := client.Realtime.Connect(ctx, &dashscope.RealtimeConfig{
//	    Model: dashscope.ModelQwenOmniTurboRealtimeLatest,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer session.Close()
//
//	// Send audio
//	session.AppendAudio(audioData)
//
//	// Receive events
//	for event, err := range session.Events() {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    // Handle event
//	}
//
// # Authentication
//
// DashScope supports API Key authentication:
//
//	client := dashscope.NewClient("sk-xxxxxxxx")
//
// For workspace isolation, use WithWorkspace option:
//
//	client := dashscope.NewClient("sk-xxxxxxxx",
//	    dashscope.WithWorkspace("ws-xxxxxxxx"),
//	)
package dashscope
