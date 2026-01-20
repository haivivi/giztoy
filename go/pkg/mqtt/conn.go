package mqtt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path"
	"sync"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

// QoS is the MQTT Quality of Service.
type QoS byte

const (
	AtMostOnce QoS = iota
	AtLeastOnce
	ExactlyOnce
)

// Conn is a MQTT connection.
type Conn struct {
	cm *autopaho.ConnectionManager

	*ServeMux

	resubscribeMu     sync.Mutex
	resubscribeCtx    context.Context
	resubscribeCancel context.CancelCauseFunc
	subscriptions     []*paho.Subscribe
}

func (conn *Conn) resubscribe() {
	conn.resubscribeMu.Lock()
	defer conn.resubscribeMu.Unlock()

	if conn.resubscribeCtx != nil {
		conn.resubscribeCancel(errors.New("resubscribe"))
		conn.resubscribeCtx = nil
		conn.resubscribeCancel = nil
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	conn.resubscribeCtx = ctx
	conn.resubscribeCancel = cancel
	subscriptions := conn.subscriptions

	for _, s := range subscriptions {
		go func(subscription *paho.Subscribe) {
			if _, err := conn.cm.Subscribe(ctx, subscription); err != nil {
				slog.Error("mqtt resubscribe error", "error", err)
			}
		}(s)
	}
}

// Close closes the connection.
func (conn *Conn) Close() error {
	conn.resubscribeMu.Lock()
	defer conn.resubscribeMu.Unlock()
	if conn.resubscribeCtx != nil {
		conn.resubscribeCancel(net.ErrClosed)
		conn.resubscribeCtx = nil
		conn.resubscribeCancel = nil
	}
	return conn.cm.Disconnect(context.Background())
}

// WriteOption is an option for writing a message.
type WriteOption interface {
	applyToPublish(*paho.Publish)
}

type packetID int

func (id packetID) applyToPublish(pub *paho.Publish) {
	pub.PacketID = uint16(id)
}

// WithPacketID sets the packet ID of the message.
func WithPacketID(id uint16) WriteOption {
	return packetID(id)
}

func (qos QoS) applyToPublish(pub *paho.Publish) {
	pub.QoS = byte(qos)
}

type retain struct{}

func (r retain) applyToPublish(pub *paho.Publish) {
	pub.Retain = true
}

// WithRetain sets the retain flag of the message.
func WithRetain() WriteOption {
	return retain{}
}

type publishProperties paho.PublishProperties

func (props *publishProperties) applyToPublish(pub *paho.Publish) {
	pub.Properties = (*paho.PublishProperties)(props)
}

// WithPublishProperties sets the publish properties of the message.
func WithPublishProperties(props paho.PublishProperties) WriteOption {
	return (*publishProperties)(&props)
}

// WriteToTopic writes a message to the topic. It publish the message with QoS 0
// and no retain flag.
func (conn *Conn) WriteToTopic(ctx context.Context, b []byte, topic string, opts ...WriteOption) error {
	pub := &paho.Publish{
		Topic:   topic,
		Payload: b,
	}
	for _, opt := range opts {
		opt.applyToPublish(pub)
	}
	_, err := conn.cm.Publish(ctx, pub)
	return err
}

// SubscribeOption is an option for subscribing to a topic.
type SubscribeOption interface {
	apply(*Conn, *paho.Subscribe)
}

func (qos QoS) apply(conn *Conn, sub *paho.Subscribe) {
	for i := range sub.Subscriptions {
		sub.Subscriptions[i].QoS = byte(qos)
	}
}

type SharedGroup string

func (g SharedGroup) apply(conn *Conn, sub *paho.Subscribe) {
	for i := range sub.Subscriptions {
		sub.Subscriptions[i].Topic = path.Join("$share", string(g), sub.Subscriptions[i].Topic)
	}
}

type AutoResubscribe struct{}

func (AutoResubscribe) apply(conn *Conn, sub *paho.Subscribe) {
	conn.subscriptions = append(conn.subscriptions, sub)
}

func (conn *Conn) SubscribeAll(ctx context.Context, topics []string, opts ...SubscribeOption) error {
	s := &paho.Subscribe{
		Subscriptions: make([]paho.SubscribeOptions, 0, len(topics)),
	}
	for _, topic := range topics {
		s.Subscriptions = append(s.Subscriptions, paho.SubscribeOptions{
			Topic: topic,
		})
	}
	for _, opt := range opts {
		opt.apply(conn, s)
	}
	_, err := conn.cm.Subscribe(ctx, s)
	return err
}

// Subscribe subscribes to a topic.
func (conn *Conn) Subscribe(ctx context.Context, topic string, opts ...SubscribeOption) error {
	s := &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{
			{
				Topic: topic,
			},
		},
	}
	for _, opt := range opts {
		opt.apply(conn, s)
	}
	if _, err := conn.cm.Subscribe(ctx, s); err != nil {
		return fmt.Errorf("subscribe %s: %w", topic, err)
	}
	return nil
}

// Unsubscribe unsubscribes from a topic.
func (conn *Conn) Unsubscribe(ctx context.Context, topic string) error {
	_, err := conn.cm.Unsubscribe(ctx, &paho.Unsubscribe{
		Topics: []string{topic},
	})
	return err
}
