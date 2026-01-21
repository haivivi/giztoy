package mqtt0

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DefaultDialer is the default dialer for MQTT connections.
// It supports tcp://, tls://, mqtts://, ws://, and wss:// schemes.
func DefaultDialer(ctx context.Context, addr string, tlsConfig *tls.Config) (net.Conn, error) {
	// Parse URL
	u, err := url.Parse(addr)
	if err != nil {
		// Try as plain host:port (assume tcp)
		return dialTCP(ctx, addr)
	}

	scheme := strings.ToLower(u.Scheme)
	host := u.Host

	switch scheme {
	case "", "tcp", "mqtt":
		// Plain TCP
		if host == "" {
			host = addr
		}
		if !strings.Contains(host, ":") {
			host += ":1883"
		}
		return dialTCP(ctx, host)

	case "tls", "mqtts", "ssl":
		// TLS
		if !strings.Contains(host, ":") {
			host += ":8883"
		}
		return dialTLS(ctx, host, tlsConfig)

	case "ws":
		// WebSocket
		if !strings.Contains(host, ":") {
			host += ":80"
		}
		wsURL := "ws://" + host + u.Path
		if u.Path == "" {
			wsURL += "/mqtt"
		}
		return dialWebSocket(ctx, wsURL, nil)

	case "wss":
		// WebSocket over TLS
		if !strings.Contains(host, ":") {
			host += ":443"
		}
		wsURL := "wss://" + host + u.Path
		if u.Path == "" {
			wsURL += "/mqtt"
		}
		return dialWebSocket(ctx, wsURL, tlsConfig)

	default:
		return nil, fmt.Errorf("mqtt0: unsupported scheme: %s", scheme)
	}
}

func dialTCP(ctx context.Context, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "tcp", addr)
}

func dialTLS(ctx context.Context, addr string, config *tls.Config) (net.Conn, error) {
	if config == nil {
		// Extract hostname for SNI
		host, _, _ := net.SplitHostPort(addr)
		config = &tls.Config{
			ServerName: host,
		}
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, config)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	return tlsConn, nil
}

func dialWebSocket(ctx context.Context, urlStr string, tlsConfig *tls.Config) (net.Conn, error) {
	dialer := websocket.Dialer{
		Subprotocols:    []string{"mqtt"},
		TLSClientConfig: tlsConfig,
	}

	ws, _, err := dialer.DialContext(ctx, urlStr, nil)
	if err != nil {
		return nil, err
	}

	return &wsConn{ws: ws}, nil
}

// wsConn wraps a websocket connection to implement net.Conn.
type wsConn struct {
	ws      *websocket.Conn
	reader  *wsReader
	writeMu sync.Mutex // protects Write operations
}

type wsReader struct {
	data []byte
	pos  int
}

func (c *wsConn) Read(b []byte) (int, error) {
	if c.reader != nil && c.reader.pos < len(c.reader.data) {
		n := copy(b, c.reader.data[c.reader.pos:])
		c.reader.pos += n
		if c.reader.pos >= len(c.reader.data) {
			c.reader = nil
		}
		return n, nil
	}

	_, data, err := c.ws.ReadMessage()
	if err != nil {
		return 0, err
	}

	n := copy(b, data)
	if n < len(data) {
		c.reader = &wsReader{data: data, pos: n}
	}
	return n, nil
}

func (c *wsConn) Write(b []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	err := c.ws.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wsConn) Close() error {
	return c.ws.Close()
}

func (c *wsConn) LocalAddr() net.Addr {
	return c.ws.LocalAddr()
}

func (c *wsConn) RemoteAddr() net.Addr {
	return c.ws.RemoteAddr()
}

func (c *wsConn) SetDeadline(t time.Time) error {
	if err := c.ws.SetReadDeadline(t); err != nil {
		return err
	}
	return c.ws.SetWriteDeadline(t)
}

func (c *wsConn) SetReadDeadline(t time.Time) error {
	return c.ws.SetReadDeadline(t)
}

func (c *wsConn) SetWriteDeadline(t time.Time) error {
	return c.ws.SetWriteDeadline(t)
}

var _ net.Conn = (*wsConn)(nil)
