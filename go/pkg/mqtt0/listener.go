package mqtt0

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Listen creates a listener for the given network and address.
//
// Network can be:
//   - "tcp" for plain TCP (default port 1883)
//   - "tls" for TLS (default port 8883)
//   - "ws" for WebSocket (default port 80)
//   - "wss" for WebSocket over TLS (default port 443)
//
// For TLS connections, tlsConfig must be provided.
// For WebSocket connections, path defaults to "/mqtt".
func Listen(network, addr string, tlsConfig *tls.Config) (net.Listener, error) {
	network = strings.ToLower(network)

	switch network {
	case "tcp", "":
		if !strings.Contains(addr, ":") {
			addr += ":1883"
		}
		return net.Listen("tcp", addr)

	case "tls":
		if tlsConfig == nil {
			return nil, fmt.Errorf("mqtt0: tls config required for tls listener")
		}
		if !strings.Contains(addr, ":") {
			addr += ":8883"
		}
		return tls.Listen("tcp", addr, tlsConfig)

	case "ws":
		if !strings.Contains(addr, ":") {
			addr += ":80"
		}
		return newWSListener(addr, nil)

	case "wss":
		if tlsConfig == nil {
			return nil, fmt.Errorf("mqtt0: tls config required for wss listener")
		}
		if !strings.Contains(addr, ":") {
			addr += ":443"
		}
		return newWSListener(addr, tlsConfig)

	default:
		return nil, fmt.Errorf("mqtt0: unsupported network: %s", network)
	}
}

// wsListener implements net.Listener for WebSocket connections.
type wsListener struct {
	addr      string
	tlsConfig *tls.Config
	connCh    chan net.Conn
	errCh     chan error
	closeOnce sync.Once
	closeCh   chan struct{}
	server    *http.Server
	upgrader  websocket.Upgrader
}

func newWSListener(addr string, tlsConfig *tls.Config) (*wsListener, error) {
	l := &wsListener{
		addr:      addr,
		tlsConfig: tlsConfig,
		connCh:    make(chan net.Conn, 100),
		errCh:     make(chan error, 1),
		closeCh:   make(chan struct{}),
		upgrader: websocket.Upgrader{
			Subprotocols: []string{"mqtt"},
			CheckOrigin:  func(r *http.Request) bool { return true },
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", l.handleWS)
	mux.HandleFunc("/mqtt", l.handleWS)

	l.server = &http.Server{
		Addr:      addr,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	// Start HTTP server
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		ln = tls.NewListener(ln, tlsConfig)
	}

	go func() {
		err := l.server.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			select {
			case l.errCh <- err:
			default:
			}
		}
	}()

	return l, nil
}

func (l *wsListener) handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	conn := &wsConn{ws: ws}

	select {
	case l.connCh <- conn:
	case <-l.closeCh:
		conn.Close()
	}
}

func (l *wsListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connCh:
		return conn, nil
	case err := <-l.errCh:
		return nil, err
	case <-l.closeCh:
		return nil, net.ErrClosed
	}
}

func (l *wsListener) Close() error {
	l.closeOnce.Do(func() {
		close(l.closeCh)
		l.server.Close()
	})
	return nil
}

func (l *wsListener) Addr() net.Addr {
	return &net.TCPAddr{Port: 0} // Placeholder
}

// MultiListener combines multiple listeners into one.
type MultiListener struct {
	listeners []net.Listener
	connCh    chan net.Conn
	errCh     chan error
	closeOnce sync.Once
	closeCh   chan struct{}
}

// NewMultiListener creates a listener that accepts from multiple underlying listeners.
func NewMultiListener(listeners ...net.Listener) *MultiListener {
	ml := &MultiListener{
		listeners: listeners,
		connCh:    make(chan net.Conn, 100),
		errCh:     make(chan error, len(listeners)),
		closeCh:   make(chan struct{}),
	}

	for _, ln := range listeners {
		go ml.acceptLoop(ln)
	}

	return ml
}

func (ml *MultiListener) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case ml.errCh <- err:
			case <-ml.closeCh:
			}
			return
		}

		select {
		case ml.connCh <- conn:
		case <-ml.closeCh:
			conn.Close()
			return
		}
	}
}

func (ml *MultiListener) Accept() (net.Conn, error) {
	select {
	case conn := <-ml.connCh:
		return conn, nil
	case err := <-ml.errCh:
		return nil, err
	case <-ml.closeCh:
		return nil, net.ErrClosed
	}
}

func (ml *MultiListener) Close() error {
	ml.closeOnce.Do(func() {
		close(ml.closeCh)
		for _, ln := range ml.listeners {
			ln.Close()
		}
	})
	return nil
}

func (ml *MultiListener) Addr() net.Addr {
	if len(ml.listeners) > 0 {
		return ml.listeners[0].Addr()
	}
	return nil
}
