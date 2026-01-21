package mqtt

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/eclipse/paho.golang/paho"
	mochimqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// ErrServerClosed is returned by the Server's Serve method after a call to Close.
var ErrServerClosed = errors.New("mqtt: server closed")

// ErrServerRunning is returned when Serve is called on a server that is already running.
var ErrServerRunning = errors.New("mqtt: server already running")

// clientIDUserProperty is the user property key for injecting client ID into messages.
const clientIDUserProperty = "_client_id"

// Server is an embedded MQTT broker.
type Server struct {
	// Handler is the message handler (similar to http.Server.Handler).
	// If nil, DefaultServeMux is used.
	Handler Handler

	// Authenticator optionally provides authentication.
	// If nil, all connections are allowed.
	Authenticator Authenticator

	// OnConnect is called when a client connects.
	OnConnect func(clientID string)

	// OnDisconnect is called when a client disconnects.
	OnDisconnect func(clientID string)

	// internal
	mochi      *mochimqtt.Server
	mu         sync.Mutex
	inShutdown atomic.Bool
}

// Authenticator provides authentication and ACL for MQTT clients.
type Authenticator interface {
	// Authenticate validates client credentials.
	// Return true to allow the connection.
	Authenticate(clientID, username string, password []byte) bool

	// ACL checks publish/subscribe permissions.
	// write=true for publish, write=false for subscribe.
	ACL(clientID, topic string, write bool) bool
}

// Serve starts the MQTT broker with the given listeners.
// It blocks until the server is closed.
//
// Serve can only be called once. If called again while running, it returns ErrServerRunning.
// After Close is called, subsequent calls to Serve return ErrServerClosed.
//
// Example:
//
//	srv := &mqtt.Server{Handler: mux}
//	tcp := listeners.NewTCP(listeners.Config{ID: "tcp", Address: ":1883"})
//	ws := listeners.NewWebsocket(listeners.Config{ID: "ws", Address: ":8083"})
//	err := srv.Serve(tcp, ws)
func (srv *Server) Serve(lns ...listeners.Listener) error {
	mochi, err := srv.init(lns)
	if err != nil {
		return err
	}
	return mochi.Serve()
}

// init initializes the mochi server with hooks and listeners.
// It returns the mochi server instance for Serve to call.
func (srv *Server) init(lns []listeners.Listener) (*mochimqtt.Server, error) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	if srv.inShutdown.Load() {
		return nil, ErrServerClosed
	}

	if srv.mochi != nil {
		return nil, ErrServerRunning
	}

	// Initialize mochi server
	mochi := mochimqtt.New(&mochimqtt.Options{
		InlineClient: true,
	})

	// Add auth hook
	if srv.Authenticator != nil {
		hook := &serverAuthHook{auth: srv.Authenticator}
		if err := mochi.AddHook(hook, nil); err != nil {
			return nil, err
		}
	} else {
		// Allow all connections if no authenticator
		if err := mochi.AddHook(new(auth.AllowHook), nil); err != nil {
			return nil, err
		}
	}

	// Determine the handler to use (similar to http.Server behavior)
	handler := srv.Handler
	if handler == nil {
		handler = DefaultServeMux
	}

	// Add callback hook
	if srv.OnConnect != nil || srv.OnDisconnect != nil || handler != nil {
		hook := &serverCallbackHook{
			handler:      handler,
			onConnect:    srv.OnConnect,
			onDisconnect: srv.OnDisconnect,
		}
		if err := mochi.AddHook(hook, nil); err != nil {
			return nil, err
		}
	}

	// Add all listeners
	for _, ln := range lns {
		if err := mochi.AddListener(ln); err != nil {
			mochi.Close() // cleanup on error
			return nil, err
		}
	}

	srv.mochi = mochi
	return mochi, nil
}

// Close gracefully closes the server. It is safe to call Close multiple times.
func (srv *Server) Close() error {
	srv.inShutdown.Store(true)

	srv.mu.Lock()
	mochi := srv.mochi
	srv.mochi = nil // Prevent double close
	srv.mu.Unlock()

	if mochi == nil {
		return nil
	}

	return mochi.Close()
}

// WriteToTopic publishes a message to the given topic.
// All clients subscribed to matching topics will receive the message.
func (srv *Server) WriteToTopic(ctx context.Context, payload []byte, topic string, opts ...WriteOption) error {
	// Check if context is already canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	srv.mu.Lock()
	mochi := srv.mochi
	srv.mu.Unlock()

	if mochi == nil {
		return errors.New("mqtt: server not running")
	}

	var (
		retainFlag bool
		qos        byte
	)
	for _, opt := range opts {
		switch v := opt.(type) {
		case retain:
			retainFlag = true
		case QoS:
			qos = byte(v)
			// packetID and publishProperties are not supported by mochi.Publish
		}
	}

	return mochi.Publish(topic, payload, retainFlag, qos)
}

// serverAuthHook implements mochi-mqtt hook for authentication.
type serverAuthHook struct {
	mochimqtt.HookBase
	auth Authenticator
}

func (h *serverAuthHook) ID() string {
	return "server-auth"
}

func (h *serverAuthHook) Provides(b byte) bool {
	return b == mochimqtt.OnConnectAuthenticate || b == mochimqtt.OnACLCheck
}

func (h *serverAuthHook) OnConnectAuthenticate(cl *mochimqtt.Client, pk packets.Packet) bool {
	return h.auth.Authenticate(cl.ID, string(pk.Connect.Username), pk.Connect.Password)
}

func (h *serverAuthHook) OnACLCheck(cl *mochimqtt.Client, topic string, write bool) bool {
	return h.auth.ACL(cl.ID, topic, write)
}

// serverCallbackHook implements mochi-mqtt hook for callbacks.
type serverCallbackHook struct {
	mochimqtt.HookBase
	handler      Handler
	onConnect    func(clientID string)
	onDisconnect func(clientID string)
}

func (h *serverCallbackHook) ID() string {
	return "server-callback"
}

func (h *serverCallbackHook) Provides(b byte) bool {
	return b == mochimqtt.OnSessionEstablished ||
		b == mochimqtt.OnDisconnect ||
		b == mochimqtt.OnPublished
}

func (h *serverCallbackHook) OnSessionEstablished(cl *mochimqtt.Client, pk packets.Packet) {
	if h.onConnect != nil {
		h.onConnect(cl.ID)
	}
}

func (h *serverCallbackHook) OnDisconnect(cl *mochimqtt.Client, err error, expire bool) {
	if h.onDisconnect != nil {
		h.onDisconnect(cl.ID)
	}
}

func (h *serverCallbackHook) OnPublished(cl *mochimqtt.Client, pk packets.Packet) {
	if h.handler != nil {
		// Convert to paho.PublishReceived and dispatch via ServeMux
		// Convert MQTT v5 properties from mochi-mqtt to paho format
		props := &paho.PublishProperties{
			ContentType:     pk.Properties.ContentType,
			ResponseTopic:   pk.Properties.ResponseTopic,
			CorrelationData: pk.Properties.CorrelationData,
		}
		if pk.Properties.MessageExpiryInterval > 0 {
			props.MessageExpiry = &pk.Properties.MessageExpiryInterval
		}
		if pk.Properties.PayloadFormatFlag {
			props.PayloadFormat = &pk.Properties.PayloadFormat
		}
		// Convert user properties
		for _, up := range pk.Properties.User {
			props.User = append(props.User, paho.UserProperty{
				Key:   up.Key,
				Value: up.Val,
			})
		}
		// Inject client ID as user property for handler to access
		props.User = append(props.User, paho.UserProperty{
			Key:   clientIDUserProperty,
			Value: cl.ID,
		})

		pr := paho.PublishReceived{
			Packet: &paho.Publish{
				Topic:      pk.TopicName,
				Payload:    pk.Payload,
				Properties: props,
			},
		}
		// Recover from handler panics to prevent crashing the broker (similar to http.Server).
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 64<<10)
				buf = buf[:runtime.Stack(buf, false)]
				slog.Error("mqtt: panic in message handler", "topic", pk.TopicName, "panic", r, "stack", string(buf))
			}
		}()
		// Error is intentionally ignored here. This hook is called by the broker
		// after a message is published - there's no way to return an error to the
		// publishing client at this point. Handler errors should be logged within
		// the handler itself if needed.
		_ = h.handler.HandleMessage(pr)
	}
}
