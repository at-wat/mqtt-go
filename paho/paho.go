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
	cli        mqtt.ClientCloser
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

type token struct {
	err  error
	done chan struct{}
}

func newToken() *token {
	return &token{
		done: make(chan struct{}),
	}
}

func (t *token) release() {
	close(t.done)
}

func (t *token) Wait() bool {
	<-t.done
	return true
}

func (t *token) WaitTimeout(d time.Duration) bool {
	select {
	case <-t.done:
		return true
	case <-time.After(d):
		return false
	}
}

func (t *token) Error() error {
	return t.err
}

type wrappedMessage struct {
	*mqtt.Message
}

func (m *wrappedMessage) Duplicate() bool {
	return m.Message.Dup
}

func (m *wrappedMessage) Qos() byte {
	return byte(m.Message.QoS)
}

func (m *wrappedMessage) Retained() bool {
	return m.Message.Retain
}

func (m *wrappedMessage) Topic() string {
	return m.Message.Topic
}

func (m *wrappedMessage) MessageID() uint16 {
	return m.Message.ID
}

func (m *wrappedMessage) Payload() []byte {
	return m.Message.Payload
}

func (m *wrappedMessage) Ack() {
}

func wrapMessage(msg *mqtt.Message) paho.Message {
	return &wrappedMessage{msg}
}

func (c *pahoWrapper) wrapMessageHandler(h paho.MessageHandler) mqtt.Handler {
	return mqtt.HandlerFunc(func(m *mqtt.Message) {
		h(c, wrapMessage(m))
	})
}

func (c *pahoWrapper) IsConnected() bool {
	select {
	case <-c.cli.Done():
	default:
		if c.cli.Err() != nil {
			return true
		}
	}
	return false
}

func (c *pahoWrapper) IsConnectionOpen() bool {
	select {
	case <-c.cli.Done():
	default:
		return true
	}
	return false
}

func (c *pahoWrapper) Connect() paho.Token {
	token := newToken()
	go func() {
		cli, err := mqtt.Dial(
			c.pahoConfig.Servers[0].String(),
			mqtt.WithTLSConfig(c.pahoConfig.TLSConfig),
		)
		if err != nil {
			token.err = err
			token.release()
			return
		}
		cli.ConnState = func(s mqtt.ConnState, err error) {
			switch s {
			case mqtt.StateActive:
				c.pahoConfig.OnConnect(c)
			case mqtt.StateClosed:
				c.pahoConfig.OnConnectionLost(c, err)
			}
		}
		cli.Handle(c.serveMux)
		c.mu.Lock()
		c.cli = cli
		c.mu.Unlock()

		opts := []mqtt.ConnectOption{
			mqtt.WithUserNamePassword(c.pahoConfig.Username, c.pahoConfig.Password),
			mqtt.WithCleanSession(c.pahoConfig.CleanSession),
			mqtt.WithKeepAlive(uint16(c.pahoConfig.KeepAlive)),
			mqtt.WithProtocolLevel(mqtt.ProtocolLevel(c.pahoConfig.ProtocolVersion)),
		}
		if c.pahoConfig.WillEnabled {
			opts = append(opts, mqtt.WithWill(&mqtt.Message{
				Topic:   c.pahoConfig.WillTopic,
				Payload: c.pahoConfig.WillPayload,
				QoS:     mqtt.QoS(c.pahoConfig.WillQos),
				Retain:  c.pahoConfig.WillRetained,
			}))
		}
		_, token.err = c.cli.Connect(context.Background(), c.pahoConfig.ClientID, opts...)
		if token.err == nil {
			if c.pahoConfig.KeepAlive > 0 {
				// Start keep alive.
				go func() {
					timeout := c.pahoConfig.PingTimeout
					if timeout < time.Second {
						timeout = time.Second
					}
					_ = mqtt.KeepAlive(
						context.Background(), cli,
						time.Duration(c.pahoConfig.KeepAlive)*time.Second,
						timeout,
					)
				}()
			}
		}
		token.release()
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
