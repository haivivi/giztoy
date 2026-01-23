// Package openairealtime provides a client for OpenAI's Realtime API.
//
// The Realtime API enables low-latency, multimodal conversations with
// GPT-4o class models. It supports both WebSocket and WebRTC connections.
//
// # Connection Modes
//
// WebSocket mode is suitable for server-side applications:
//
//	client := openairealtime.NewClient(apiKey)
//	session, err := client.ConnectWebSocket(ctx, &openairealtime.ConnectConfig{
//	    Model: openairealtime.ModelGPT4oRealtimePreview,
//	})
//	if err != nil {
//	    return err
//	}
//	defer session.Close()
//
// WebRTC mode is suitable for client-side applications with lower latency:
//
//	session, err := client.ConnectWebRTC(ctx, &openairealtime.ConnectConfig{
//	    Model: openairealtime.ModelGPT4oRealtimePreview,
//	})
//	if err != nil {
//	    return err
//	}
//	defer session.Close()
//
// # Session Configuration
//
// After connecting, configure the session:
//
//	err = session.UpdateSession(&openairealtime.SessionConfig{
//	    Voice:        openairealtime.VoiceAlloy,
//	    Instructions: "You are a helpful assistant.",
//	    TurnDetection: &openairealtime.TurnDetection{
//	        Type: openairealtime.VADServerVAD,
//	    },
//	})
//
// # Sending Audio
//
// Send audio data to the input buffer:
//
//	// PCM 16-bit, 24kHz, mono
//	err = session.AppendAudio(pcmData)
//
// # Receiving Events
//
// Use the Events iterator to receive server events:
//
//	for event, err := range session.Events() {
//	    if err != nil {
//	        return err
//	    }
//	    switch event.Type {
//	    case openairealtime.EventResponseAudioDelta:
//	        playAudio(event.Audio)
//	    case openairealtime.EventResponseTextDelta:
//	        fmt.Print(event.Delta)
//	    }
//	}
package openairealtime
