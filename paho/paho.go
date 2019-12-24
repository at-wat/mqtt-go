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
	"errors"
	"sync"
	"time"

	"github.com/at-wat/mqtt-go"
	paho "github.com/eclipse/paho.mqtt.golang"
)

var errNotConnected = errors.New("not connected")

type pahoWrapper struct {
	cli        mqtt.Client
	cliCloser  mqtt.Closer
	serveMux   *mqtt.ServeMux
	pahoConfig *paho.ClientOptions
	mu         sync.Mutex
}

// NewClient creates paho.mqtt.golang interface wrapping at-wat/mqtt-go.
// It's very experimental and some of the options are not supported.
func NewClient(o *paho.ClientOptions) paho.Client {
	w := &pahoWrapper{
		pahoConfig: o,
		serveMux:   &mqtt.ServeMux{},
	}
	if len(o.Servers) != 1 {
		panic("unsupported number of servers")
	}
	if o.AutoReconnect {
		panic("paho style auto-reconnect is not supported")
	}

	return w
}

func (c *pahoWrapper) IsConnected() bool {
	select {
	case <-c.cliCloser.Done():
		if c.cliCloser.Err() == nil {
			return true
		}
	default:
		return true
	}
	return false
}

func (c *pahoWrapper) IsConnectionOpen() bool {
	select {
	case <-c.cliCloser.Done():
	default:
		return true
	}
	return false
}

func (c *pahoWrapper) Connect() paho.Token {
	opts := []mqtt.ConnectOption{
		mqtt.WithUserNamePassword(c.pahoConfig.Username, c.pahoConfig.Password),
		mqtt.WithCleanSession(c.pahoConfig.CleanSession),
		mqtt.WithKeepAlive(uint16(c.pahoConfig.KeepAlive)),
	}
	if c.pahoConfig.ProtocolVersion > 0 {
		opts = append(opts,
			mqtt.WithProtocolLevel(mqtt.ProtocolLevel(c.pahoConfig.ProtocolVersion)),
		)
	}
	if c.pahoConfig.WillEnabled {
		opts = append(opts, mqtt.WithWill(&mqtt.Message{
			Topic:   c.pahoConfig.WillTopic,
			Payload: c.pahoConfig.WillPayload,
			QoS:     mqtt.QoS(c.pahoConfig.WillQos),
			Retain:  c.pahoConfig.WillRetained,
		}))
	}
	if c.pahoConfig.AutoReconnect {
		return c.connectRetry(opts)
	}
	return c.connectOnce(opts)
}

func (c *pahoWrapper) connectRetry(opts []mqtt.ConnectOption) paho.Token {
	token := newToken()
	go func() {
		pingInterval := time.Duration(c.pahoConfig.KeepAlive) * time.Second

		cli, err := mqtt.NewReconnectClient(context.Background(),
			mqtt.DialerFunc(func() (mqtt.ClientCloser, error) {
				cb, err := mqtt.Dial(c.pahoConfig.Servers[0].String(),
					mqtt.WithTLSConfig(c.pahoConfig.TLSConfig),
				)
				if err != nil {
					return nil, err
				}
				cb.ConnState = func(s mqtt.ConnState, err error) {
					switch s {
					case mqtt.StateActive:
						c.pahoConfig.OnConnect(c)
					case mqtt.StateClosed:
						c.pahoConfig.OnConnectionLost(c, err)
					}
				}
				cb.Handle(c.serveMux)
				return cb, err
			}),
			c.pahoConfig.ClientID,
			mqtt.WithConnectOption(opts...),
			mqtt.WithPingInterval(pingInterval),
			mqtt.WithTimeout(c.pahoConfig.PingTimeout),
			mqtt.WithReconnectWait(
				time.Second,    // c.pahoConfig.ConnectRetryInterval,
				10*time.Second, // c.pahoConfig.MaxReconnectInterval,
			),
		)
		if err != nil {
			token.err = err
			token.release()
			return
		}
		c.mu.Lock()
		c.cli = cli
		c.mu.Unlock()

		token.release()
	}()
	return token
}

func (c *pahoWrapper) connectOnce(opts []mqtt.ConnectOption) paho.Token {
	token := newToken()
	go func() {
		for { // Connect retry loop
			cli, err := mqtt.Dial(
				c.pahoConfig.Servers[0].String(),
				mqtt.WithTLSConfig(c.pahoConfig.TLSConfig),
			)
			if err != nil {
				// if c.pahoConfig.ConnectRetry {
				//   time.Sleep(c.pahoConfig.ConnectRetryInterval)
				//   continue
				// }
				token.err = err
				token.release()
				return
			}
			cli.ConnState = func(s mqtt.ConnState, err error) {
				switch s {
				case mqtt.StateActive:
					if c.pahoConfig.OnConnect != nil {
						c.pahoConfig.OnConnect(c)
					}
				case mqtt.StateClosed:
					if c.pahoConfig.OnConnectionLost != nil {
						c.pahoConfig.OnConnectionLost(c, err)
					}
				}
			}
			cli.Handle(c.serveMux)
			c.mu.Lock()
			c.cli = cli
			c.mu.Unlock()

			_, token.err = c.cli.Connect(context.Background(), c.pahoConfig.ClientID, opts...)
			if token.err == nil {
				if c.pahoConfig.KeepAlive > 0 {
					// Start keep alive.
					go func() {
						_ = mqtt.KeepAlive(
							context.Background(), cli,
							time.Duration(c.pahoConfig.KeepAlive)*time.Second,
							c.pahoConfig.PingTimeout,
						)
					}()
				}
			}
			token.release()
			return
		}
	}()
	return token
}

func (c *pahoWrapper) Disconnect(quiesce uint) {
	c.mu.Lock()
	cli := c.cli
	c.mu.Unlock()
	if cli == nil {
		return
	}

	time.Sleep(time.Duration(quiesce) * time.Millisecond)
	_ = cli.Disconnect(context.Background())
}

func (c *pahoWrapper) Publish(topic string, qos byte, retained bool, payload interface{}) paho.Token {
	var p []byte
	switch v := payload.(type) {
	case []byte:
		p = v
	case string:
		p = []byte(v)
	default:
		panic("payload type error")
	}
	token := newToken()
	go func() {
		c.mu.Lock()
		cli := c.cli
		c.mu.Unlock()
		if cli == nil {
			token.err = errNotConnected
			return
		}

		token.err = cli.Publish(
			context.Background(),
			&mqtt.Message{
				Topic:   topic,
				Payload: p,
			},
		)
		token.release()
	}()
	return token
}

func (c *pahoWrapper) Subscribe(topic string, qos byte, callback paho.MessageHandler) paho.Token {
	token := newToken()
	c.serveMux.Handle(topic, c.wrapMessageHandler(callback))
	go func() {
		c.mu.Lock()
		cli := c.cli
		c.mu.Unlock()
		if cli == nil {
			token.err = errNotConnected
			return
		}

		token.err = cli.Subscribe(
			context.Background(),
			mqtt.Subscription{
				Topic: topic,
				QoS:   mqtt.QoS(qos),
			},
		)
		token.release()
	}()
	return token
}

func (c *pahoWrapper) SubscribeMultiple(filters map[string]byte, callback paho.MessageHandler) paho.Token {
	token := newToken()
	var subs []mqtt.Subscription
	for f, qos := range filters {
		c.serveMux.Handle(f, c.wrapMessageHandler(callback))
		subs = append(subs,
			mqtt.Subscription{
				Topic: f,
				QoS:   mqtt.QoS(qos),
			},
		)
	}
	go func() {
		c.mu.Lock()
		cli := c.cli
		c.mu.Unlock()
		if cli == nil {
			token.err = errNotConnected
			return
		}

		token.err = cli.Subscribe(context.Background(), subs...)
		token.release()
	}()
	return token
}

func (c *pahoWrapper) Unsubscribe(topics ...string) paho.Token {
	token := newToken()
	go func() {
		c.mu.Lock()
		cli := c.cli
		c.mu.Unlock()
		if cli == nil {
			token.err = errNotConnected
			return
		}

		token.err = cli.Unsubscribe(context.Background(), topics...)
		token.release()
	}()
	return token
}

func (c *pahoWrapper) AddRoute(topic string, callback paho.MessageHandler) {
	c.serveMux.Handle(topic, c.wrapMessageHandler(callback))
}

func (c *pahoWrapper) OptionsReader() paho.ClientOptionsReader {
	panic("not implemented")
}
