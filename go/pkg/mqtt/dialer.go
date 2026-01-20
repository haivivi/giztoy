package mqtt

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"
)

const defaultConnectRetryDelay = 3 * time.Second

// Dialer is a MQTT client dialer, it contains all the options to establish and
// maintain a MQTT connection.
type Dialer struct {

	// Keepalive period in seconds (the maximum time interval that is permitted
	// to elapse between the point at which the Client finishes transmitting one
	// MQTT Control Packet and the point it starts sending the next)
	KeepAlive int

	// Session Expiry Interval in seconds (if 0 the Session ends when the
	// Network Connection is closed)
	SessionExpiryInterval int

	// How long to wait between connection attempts (defaults to 3s)
	ConnectRetryDelay time.Duration

	// How long to wait for the connection process to complete (defaults to 10s)
	ConnectTimeout time.Duration

	// ID is the client identifier (defaults to a random string)
	ID string

	// SubscribeProperties is the properties to be used when subscribing to a
	// topic (defaults to nil)
	SubscribeProperties *paho.SubscribeProperties

	// ServeMux is the MQTT message handler (defaults to DefaultServeMux).
	ServeMux *ServeMux

	// OnConnectError is called when a connection attempt fails. This is useful
	// for logging retry attempts. The error passed is the error from the
	// connection attempt.
	OnConnectError func(error)

	// OnConnectionUp is called when a connection is established (including
	// reconnections). This is useful for logging successful connections.
	OnConnectionUp func()
}

func (dl *Dialer) keepAlive() uint16 {
	if dl.KeepAlive == 0 {
		return 20
	}
	return uint16(dl.KeepAlive)
}

func (dl *Dialer) sessionExpiryInterval() uint32 {
	return uint32(dl.SessionExpiryInterval)
}

func (dl *Dialer) connectRetryDelay() time.Duration {
	if dl.ConnectRetryDelay == 0 {
		return defaultConnectRetryDelay
	}
	return dl.ConnectRetryDelay
}

// DialOption is an option for dialing a MQTT connection.
type DialOption interface {
	apply(*autopaho.ClientConfig) error
}

type Creds interface {
	User() (*url.Userinfo, bool)
}

type userinfo url.Userinfo

func (up *userinfo) User() (*url.Userinfo, bool) {
	return (*url.Userinfo)(up), true
}

func WithUser(username, password string) DialOption {
	return &withCreds{(*userinfo)(url.UserPassword(username, password))}
}

func WithCredentials(creds Creds) DialOption {
	return &withCreds{creds}
}

type withCreds struct {
	Creds
}

func (wc *withCreds) apply(cfg *autopaho.ClientConfig) error {
	if wc.Creds == nil {
		return nil
	}
	if fn := cfg.ConnectPacketBuilder; fn != nil {
		cfg.ConnectPacketBuilder = func(pc *paho.Connect, uri *url.URL) (*paho.Connect, error) {
			pc, err := fn(pc, uri)
			if err != nil {
				return nil, err
			}
			return wc.setCreds(pc, uri), nil
		}
		return nil
	}
	cfg.ConnectPacketBuilder = func(pc *paho.Connect, uri *url.URL) (*paho.Connect, error) {
		return wc.setCreds(pc, uri), nil
	}
	return nil
}

func (wc *withCreds) setCreds(pc *paho.Connect, _ *url.URL) *paho.Connect {
	ui, ok := wc.User()
	if !ok {
		// keep the existing credentials
		return pc
	}
	if ui == nil {
		pc.UsernameFlag = false
		pc.PasswordFlag = false
		pc.Username = ""
		pc.Password = nil
		return pc
	}
	pwd, hasPwd := ui.Password()
	pc.UsernameFlag = true
	pc.Username = ui.Username()
	if hasPwd {
		pc.Password = []byte(pwd)
		pc.PasswordFlag = true
	} else {
		pc.Password = nil
		pc.PasswordFlag = false
	}
	return pc
}

type WithAdditionalAddr string

func (a WithAdditionalAddr) apply(cfg *autopaho.ClientConfig) error {
	u, err := url.Parse(string(a))
	if err != nil {
		return err
	}
	cfg.ServerUrls = append(cfg.ServerUrls, u)
	return nil
}

// Dial connects to the MQTT server at the given address.
func (dl *Dialer) Dial(ctx context.Context, addr string, opts ...DialOption) (conn *Conn, err error) {
	id := dl.ID
	if id == "" {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return nil, err
		}
		id = base64.RawURLEncoding.EncodeToString(b[:])
	}
	sm := dl.ServeMux
	if sm == nil {
		sm = DefaultServeMux
	}
	addru, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	var (
		connected atomic.Bool
		cfg       autopaho.ClientConfig
	)
	cfg = autopaho.ClientConfig{
		ServerUrls:        []*url.URL{addru},
		AttemptConnection: dl.attemptConnection,
		OnConnectError: func(err error) {
			// Always call user callback if provided (for logging during initial connect retry)
			if dl.OnConnectError != nil {
				dl.OnConnectError(err)
			}
			// Internal logic only after initial connection established
			if !connected.Load() {
				return
			}
			conn.resubscribeMu.Lock()
			defer conn.resubscribeMu.Unlock()

			if conn.resubscribeCtx != nil {
				conn.resubscribeCancel(err)
				conn.resubscribeCtx = nil
				conn.resubscribeCancel = nil
			}
		},
		OnConnectionUp: func(cm *autopaho.ConnectionManager, c *paho.Connack) {
			// Call user callback if provided
			if dl.OnConnectionUp != nil {
				dl.OnConnectionUp()
			}
			// Internal logic only after initial connection established
			if !connected.Load() {
				return
			}
			conn.resubscribe()
		},
		CleanStartOnInitialConnection: true,
		KeepAlive:                     dl.keepAlive(),
		SessionExpiryInterval:         dl.sessionExpiryInterval(),
		ConnectRetryDelay:             dl.connectRetryDelay(),
		ConnectTimeout:                dl.ConnectTimeout,
		ConnectPacketBuilder: func(pc *paho.Connect, uri *url.URL) (*paho.Connect, error) {
			if uri.User == nil {
				pc.UsernameFlag = false
				pc.PasswordFlag = false
				pc.Username = ""
				pc.Password = nil
				return pc, nil
			}
			pc.UsernameFlag = true
			pc.Username = uri.User.Username()
			if pwd, ok := uri.User.Password(); ok {
				pc.PasswordFlag = true
				pc.Password = []byte(pwd)
			} else {
				pc.PasswordFlag = false
				pc.Password = nil
			}
			return pc, nil
		},
		ClientConfig: paho.ClientConfig{
			ClientID: id,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					if err := sm.HandleMessage(pr); err != nil {
						return false, err
					}
					return true, nil
				},
			},
		},
	}
	for _, opt := range opts {
		if err := opt.apply(&cfg); err != nil {
			return nil, err
		}
	}
	cm, err := autopaho.NewConnection(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	if err := cm.AwaitConnection(ctx); err != nil {
		return nil, err
	}
	connected.Store(true)
	return &Conn{cm: cm, ServeMux: dl.ServeMux}, nil
}

func (dl *Dialer) attemptConnection(ctx context.Context, cc autopaho.ClientConfig, u *url.URL) (net.Conn, error) {
	switch strings.ToLower(u.Scheme) {
	case "mqtt", "tcp", "":
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", u.Host)
		if err != nil {
			return nil, err
		}
		if err := conn.(*net.TCPConn).SetNoDelay(true); err != nil {
			return nil, err
		}
		return packets.NewThreadSafeConn(conn), nil
	case "ssl", "tls", "mqtts", "mqtt+ssl", "tcps":
		d := tls.Dialer{
			Config: cc.TlsCfg,
		}
		conn, err := d.DialContext(ctx, "tcp", u.Host)
		if err != nil {
			return nil, err
		}
		if err := conn.(*tls.Conn).NetConn().(*net.TCPConn).SetNoDelay(true); err != nil {
			return nil, err
		}
		return packets.NewThreadSafeConn(conn), nil
	default:
		// TODO: support ws/wss
		return nil, fmt.Errorf("unsupported scheme (%s) user in url %s", u.Scheme, u.String())
	}
}

// DefaultServeMux is the default MQTT message handler.
var DefaultServeMux = NewServeMux()

// Handle registers the handler for the given pattern to the default serve mux.
func Handle(pattern string, h Handler) error {
	return DefaultServeMux.Handle(pattern, h)
}

// HandleFunc registers the handler function for the given pattern to the default
// serve mux.
func HandleFunc(pattern string, h HandlerFunc) error {
	return DefaultServeMux.HandleFunc(pattern, h)
}

// HandleFuncWithOptions registers the handler function for the given pattern to
// the default serve mux with the given options.
func HandleFuncWithOptions(pattern paho.SubscribeOptions, h HandlerFunc) error {
	return DefaultServeMux.HandleFuncWithOptions(pattern, h)
}

// HandleWithOptions registers the handler for the given pattern to the default
// serve mux with the given options.
func HandleWithOptions(pattern paho.SubscribeOptions, h Handler) error {
	return DefaultServeMux.HandleWithOptions(pattern, h)
}

// Dial connects to the MQTT server at the given address with the default dialer.
func Dial(ctx context.Context, addr string, opts ...DialOption) (*Conn, error) {
	return (&Dialer{ServeMux: NewServeMux()}).Dial(ctx, addr, opts...)
}
