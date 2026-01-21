// Package mqtt0 provides a lightweight QoS 0 MQTT client and broker implementation
// with full control over authentication and ACL.
//
// This package supports both MQTT 3.1.1 (v4) and MQTT 5.0 (v5) protocols,
// and provides multiple transport options: TCP, TLS, WebSocket, and WebSocket over TLS.
//
// # Components
//
//   - [Client]: QoS 0 MQTT client
//   - [Broker]: QoS 0 MQTT broker
//
// # Example - Client
//
//	client, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
//	    Addr:     "tcp://127.0.0.1:1883",
//	    ClientID: "my-client",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Subscribe
//	if err := client.Subscribe(ctx, "test/topic"); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Publish
//	if err := client.Publish(ctx, "test/topic", []byte("hello")); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Receive
//	msg, err := client.Recv(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Received: %s -> %s\n", msg.Topic, msg.Payload)
//
// # Example - Broker
//
//	broker := &mqtt0.Broker{
//	    Authenticator: myAuthenticator,
//	    OnConnect:     func(clientID string) { log.Printf("Connected: %s", clientID) },
//	    OnDisconnect:  func(clientID string) { log.Printf("Disconnected: %s", clientID) },
//	}
//
//	ln, err := mqtt0.Listen("tcp", ":1883", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	log.Fatal(broker.Serve(ln))
//
// # Protocol Support
//
// | Protocol | Support |
// |----------|---------|
// | MQTT 3.1.1 (v4) | ✅ Full |
// | MQTT 5.0 (v5) | ✅ Full |
//
// # Transport Support
//
// | Transport | URL Scheme | Example |
// |-----------|------------|---------|
// | TCP | tcp:// | tcp://localhost:1883 |
// | TLS | tls://, mqtts:// | tls://localhost:8883 |
// | WebSocket | ws:// | ws://localhost:8083/mqtt |
// | WebSocket+TLS | wss:// | wss://localhost:8084/mqtt |
package mqtt0
