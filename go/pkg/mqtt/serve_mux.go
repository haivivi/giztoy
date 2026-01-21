package mqtt

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/eclipse/paho.golang/paho"
)

type Message = paho.PublishReceived

// Handler is the interface that wraps the HandleMessage method.
type Handler interface {
	HandleMessage(Message) error
}

// HandlerFunc is a function that takes a Publish message.
type HandlerFunc func(Message) error

func (f HandlerFunc) HandleMessage(m Message) error {
	return f(m)
}

type ServeMux struct {
	mu        sync.RWMutex
	matchRoot *trie

	aliases map[uint16]string
}

type ServeMuxOption interface {
	apply(*ServeMux)
}

// NewServeMux instantiates and returns an instance of a StandardRouter
func NewServeMux(opts ...ServeMuxOption) *ServeMux {
	sm := &ServeMux{
		matchRoot: &trie{},
		aliases:   make(map[uint16]string),
	}
	for _, opt := range opts {
		opt.apply(sm)
	}
	return sm
}

// HandleFunc registers the handler function for the given pattern. It
// subscribes to the topic pattern with the default QoS of 0.
func (sm *ServeMux) HandleFunc(pattern string, h HandlerFunc) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.matchRoot.Set(pattern, func(node *trie) {
		node.handlers = append(node.handlers, h)
	})
}

// HandleFuncWithOptions registers the handler function for the given pattern
// and subscribes to the topic pattern with the given options.
func (sm *ServeMux) HandleFuncWithOptions(pattern paho.SubscribeOptions, h HandlerFunc) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.matchRoot.Set(pattern.Topic, func(node *trie) {
		node.handlers = append(node.handlers, h)
	})
}

// Handle registers the handler for the given pattern. It subscribes to the
// topic pattern with the default QoS of 0.
func (sm *ServeMux) Handle(pattern string, h Handler) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.matchRoot.Set(pattern, func(node *trie) {
		slog.Debug("handle pattern", "pattern", pattern, "handler_type", fmt.Sprintf("%T", h))
		node.handlers = append(node.handlers, h)
	})
}

// HandleWithOptions registers the handler for the given pattern and subscribes
// to the topic pattern with the given options.
func (sm *ServeMux) HandleWithOptions(pattern paho.SubscribeOptions, h Handler) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.matchRoot.Set(pattern.Topic, func(node *trie) {
		node.handlers = append(node.handlers, h)
	})
}

// Route is the library provided StandardRouter's implementation
// of the required interface function()
func (sm *ServeMux) HandleMessage(pr paho.PublishReceived) error {
	if pr.AlreadyHandled {
		return nil
	}
	msg := pr.Packet
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var topic string
	if msg.Properties.TopicAlias != nil {
		slog.Debug("message is using topic aliasing")
		if msg.Topic != "" {
			// Register new alias
			slog.Debug("registering new topic alias", "alias", *msg.Properties.TopicAlias, "topic", msg.Topic)
			sm.aliases[*msg.Properties.TopicAlias] = msg.Topic
		}
		if t, ok := sm.aliases[*msg.Properties.TopicAlias]; ok {
			slog.Debug("aliased topic translates", "alias", *msg.Properties.TopicAlias, "topic", msg.Topic)
			topic = t
		}
	} else {
		topic = msg.Topic
	}
	_, handlers, ok := sm.matchRoot.match("", topic)
	if !ok {
		slog.Debug("no handler found", "topic", topic)
		return fmt.Errorf("no handler found for %v", topic)
	}
	for _, h := range handlers {
		if err := h.HandleMessage(pr); err != nil {
			return err
		}
	}
	return nil
}

func (sm *ServeMux) String() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.matchRoot.String()
}
