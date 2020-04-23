// Copyright 2019 The mqtt-go authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"

	"golang.org/x/net/websocket"
)

// ErrUnsupportedProtocol means that the specified scheme in the URL is not supported.
var ErrUnsupportedProtocol = errors.New("unsupported protocol")

// ErrClosedTransport means that the underlying connection is closed.
var ErrClosedTransport = errors.New("read/write on closed transport")

// URLDialer is a Dialer using URL string.
type URLDialer struct {
	URL     string
	Options []DialOption
}

// Dialer is an interface to create connection.
type Dialer interface {
	Dial() (ClientCloser, error)
}

// DialerFunc type is an adapter to use functions as MQTT connection dialer.
type DialerFunc func() (ClientCloser, error)

// Dial calls d().
func (d DialerFunc) Dial() (ClientCloser, error) {
	return d()
}

// Dial creates connection using its values.
func (d *URLDialer) Dial() (ClientCloser, error) {
	return Dial(d.URL, d.Options...)
}

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
	ConnState func(ConnState, error)
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

// WithTLSCertFiles loads certificate files
func WithTLSCertFiles(host, caFile, certFile, privateKeyFile string) DialOption {
	return func(o *DialOptions) error {
		certpool := x509.NewCertPool()
		cas, err := ioutil.ReadFile(caFile)
		if err != nil {
			return err
		}
		certpool.AppendCertsFromPEM(cas)

		cert, err := tls.LoadX509KeyPair(certFile, privateKeyFile)
		if err != nil {
			return err
		}

		if o.TLSConfig == nil {
			o.TLSConfig = &tls.Config{}
		}
		o.TLSConfig.ServerName = host
		o.TLSConfig.RootCAs = certpool
		o.TLSConfig.Certificates = []tls.Certificate{cert}
		return nil
	}
}

// WithConnStateHandler sets connection state change handler.
func WithConnStateHandler(handler func(ConnState, error)) DialOption {
	return func(o *DialOptions) error {
		o.ConnState = handler
		return nil
	}
}

func (d *DialOptions) dial(urlStr string) (*BaseClient, error) {
	c := &BaseClient{
		ConnState: d.ConnState,
	}

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
	case "tls", "ssl", "mqtts":
		conn, err := tls.DialWithDialer(d.Dialer, "tcp", u.Host, d.TLSConfig)
		if err != nil {
			return nil, err
		}
		c.Transport = conn
	case "ws", "wss":
		wsc, err := websocket.NewConfig(u.String(), fmt.Sprintf("https://%s", u.Host))
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
		ws.PayloadType = websocket.BinaryFrame
		c.Transport = ws
	default:
		return nil, wrapErrorf(ErrUnsupportedProtocol, "protocol %s", u.Scheme)
	}
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

// Done is a channel to signal connection close.
func (c *BaseClient) Done() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connClosed
}

// Err returns connection error.
func (c *BaseClient) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}
