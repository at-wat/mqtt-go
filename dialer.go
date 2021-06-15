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
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"sync"

	"golang.org/x/net/websocket"
)

// ErrUnsupportedProtocol means that the specified scheme in the URL is not supported.
var ErrUnsupportedProtocol = errors.New("unsupported protocol")

// URLDialer is a Dialer using URL string.
type URLDialer struct {
	URL     string
	Options []DialOption
}

// Dialer is an interface to create connection.
type Dialer interface {
	Dial() (*BaseClient, error)
	DialContext(context.Context) (*BaseClient, error)
}

// DialerFunc type is an adapter to use functions as MQTT connection dialer.
type DialerFunc func(ctx context.Context) (*BaseClient, error)

// Dial calls d() without context.
func (d DialerFunc) Dial() (*BaseClient, error) {
	return d(context.TODO())
}

// DialContext calls d().
func (d DialerFunc) DialContext(ctx context.Context) (*BaseClient, error) {
	return d(ctx)
}

// Dial creates connection using its values without context.
func (d *URLDialer) Dial() (*BaseClient, error) {
	return DialContext(context.TODO(), d.URL, d.Options...)
}

// DialContext creates connection using its values.
func (d *URLDialer) DialContext(ctx context.Context) (*BaseClient, error) {
	return DialContext(ctx, d.URL, d.Options...)
}

// Dial creates MQTT client using URL string without context.
func Dial(urlStr string, opts ...DialOption) (*BaseClient, error) {
	return DialContext(context.TODO(), urlStr, opts...)
}

// DialContext creates MQTT client using URL string.
func DialContext(ctx context.Context, urlStr string, opts ...DialOption) (*BaseClient, error) {
	o := &DialOptions{
		Dialer: &net.Dialer{},
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o.dial(ctx, urlStr)
}

// DialOption sets option for Dial.
type DialOption func(*DialOptions) error

// DialOptions stores options for Dial.
type DialOptions struct {
	Dialer        *net.Dialer
	TLSConfig     *tls.Config
	ConnState     func(ConnState, error)
	MaxPayloadLen int
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

// WithMaxPayloadLen sets maximum payload length of the BaseClient.
func WithMaxPayloadLen(l int) DialOption {
	return func(o *DialOptions) error {
		o.MaxPayloadLen = l
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

func (d *DialOptions) dial(ctx context.Context, urlStr string) (*BaseClient, error) {
	c := &BaseClient{
		ConnState:     d.ConnState,
		MaxPayloadLen: d.MaxPayloadLen,
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "tcp", "mqtt", "tls", "ssl", "mqtts", "wss", "ws":
	default:
		return nil, wrapErrorf(ErrUnsupportedProtocol, "protocol %s", u.Scheme)
	}

	baseConn, err := d.Dialer.DialContext(ctx, "tcp", u.Host)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "tcp", "mqtt":
		c.Transport = baseConn
	case "tls", "ssl", "mqtts":
		c.Transport = tls.Client(baseConn, d.TLSConfig)
	case "wss":
		baseConn = tls.Client(baseConn, d.TLSConfig)
		fallthrough
	case "ws":
		wsc, err := websocket.NewConfig(u.String(), fmt.Sprintf("https://%s", u.Host))
		if err != nil {
			return nil, err
		}
		wsc.Protocol = append(wsc.Protocol, "mqtt")
		wsc.TlsConfig = d.TLSConfig
		ws, err := websocket.NewClient(wsc, baseConn)
		if err != nil {
			return nil, err
		}
		ws.PayloadType = websocket.BinaryFrame
		c.Transport = ws
	}
	return c, nil
}

// BaseClientStoreDialer is a dialer wrapper which stores the latest BaseClient.
type BaseClientStoreDialer struct {
	// Dialer is a wrapped dialer. Valid Dialer must be set before use.
	Dialer

	mu         sync.RWMutex
	baseClient *BaseClient
}

// Dial creates a new BaseClient.
func (d *BaseClientStoreDialer) Dial() (*BaseClient, error) {
	cli, err := d.Dialer.Dial()
	d.mu.Lock()
	d.baseClient = cli
	d.mu.Unlock()
	return cli, err
}

// BaseClient returns latest BaseClient created by the dialer.
// It returns nil before the first call of Dial.
func (d *BaseClientStoreDialer) BaseClient() *BaseClient {
	d.mu.RLock()
	cli := d.baseClient
	d.mu.RUnlock()
	return cli
}
