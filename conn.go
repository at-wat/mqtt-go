package mqtt

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"

	"golang.org/x/net/websocket"
)

// ErrUnsupportedProtocol means that the specified scheme in the URL is not supported.
var ErrUnsupportedProtocol = errors.New("unsupported protocol")

// ErrClosedTransport means that the underlying connection is closed.
var ErrClosedTransport = errors.New("read/write on closed transport")

// Dial creates MQTT client using URL string.
func Dial(urlStr string, opts ...DialOption) (*BaseClient, error) {
	o := &DialOptions{
		Dialer: &net.Dialer{},
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o.dial(urlStr)
}

// DialOption sets option for Dial.
type DialOption func(*DialOptions) error

// DialOptions stores options for Dial.
type DialOptions struct {
	Dialer    *net.Dialer
	TLSConfig *tls.Config
}

// WithDialer sets dialer.
func WithDialer(dialer *net.Dialer) DialOption {
	return func(o *DialOptions) error {
		o.Dialer = dialer
		return nil
	}
}

// WithTLSConfig sets TLS configuration.
func WithTLSConfig(config *tls.Config) DialOption {
	return func(o *DialOptions) error {
		o.TLSConfig = config
		return nil
	}
}

func (d *DialOptions) dial(urlStr string) (*BaseClient, error) {
	c := &BaseClient{}

	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "tcp", "mqtt":
		conn, err := d.Dialer.Dial("tcp", u.Host)
		if err != nil {
			return nil, err
		}
		c.Transport = conn
	case "tls", "mqtts":
		conn, err := tls.DialWithDialer(d.Dialer, "tcp", u.Host, d.TLSConfig)
		if err != nil {
			return nil, err
		}
		c.Transport = conn
	case "ws", "wss":
		wsc, err := websocket.NewConfig(
			fmt.Sprintf("%s://%s:%s%s", u.Scheme, u.Hostname(), u.Port(), u.EscapedPath()), "ws://")
		if err != nil {
			return nil, err
		}
		wsc.Protocol = append(wsc.Protocol, "mqtt")
		wsc.Dialer = d.Dialer
		wsc.TlsConfig = d.TLSConfig
		ws, err := websocket.DialConfig(wsc)
		if err != nil {
			return nil, err
		}
		c.Transport = ws
	default:
		return nil, ErrUnsupportedProtocol
	}
	c.connStateUpdate(StateIdle)
	c.connClosed = make(chan struct{})

	go func() {
		err := c.serve()
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
	}()
	return c, nil
}

func (c *BaseClient) connStateUpdate(newState ConnState) {
	c.mu.Lock()
	lastState := c.connState
	if c.connState != StateDisconnected {
		c.connState = newState
	}
	state := c.connState
	err := c.err
	c.mu.Unlock()

	if c.ConnState != nil && lastState != state {
		c.ConnState(state, err)
	}
}

// Close force closes MQTT connection.
func (c *BaseClient) Close() error {
	return c.Transport.Close()
}
