// +build integration

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
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"testing"
	"time"
)

var (
	urls = map[string]string{
		"MQTT":       "mqtt://localhost:1883",
		"MQTTs":      "mqtts://localhost:8883",
		"WebSocket":  "ws://localhost:9001",
		"WebSockets": "wss://localhost:9443",
	}
)

func ExampleClient() {
	done := make(chan struct{})

	baseCli, err := Dial("mqtt://localhost:1883")
	if err != nil {
		panic(err)
	}

	// store as Client to make it easy to enable high level wrapper later
	var cli Client = baseCli
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cli.Handle(HandlerFunc(func(msg *Message) {
		fmt.Printf("%s[%d]: %s", msg.Topic, int(msg.QoS), []byte(msg.Payload))
		close(done)
	}))

	if _, err := cli.Connect(ctx, "TestClient", WithCleanSession(true)); err != nil {
		panic(err)
	}
	if err := cli.Subscribe(ctx, Subscription{Topic: "test/topic", QoS: QoS1}); err != nil {
		panic(err)
	}

	if err := cli.Publish(ctx, &Message{
		Topic: "test/topic", QoS: QoS1, Payload: []byte("message"),
	}); err != nil {
		panic(err)
	}

	<-done
	if err := cli.Disconnect(ctx); err != nil {
		panic(err)
	}

	// Output: test/topic[1]: message
}

func TestIntegration_Connect(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			cli, err := Dial(url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := cli.Connect(ctx, "Client"); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Disconnect(ctx); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
		})
	}
}

func TestIntegration_Publish(t *testing.T) {
	for _, size := range []int{0x100, 0x3FF7, 0x3FF8, 0x7FF7, 0x7FF8, 0x20000} {
		t.Run(fmt.Sprintf("%dBytes", size), func(t *testing.T) {
			for name, url := range urls {
				t.Run(name, func(t *testing.T) {
					cli, err := Dial(url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if _, err := cli.Connect(ctx, fmt.Sprintf("Client%s%x", name, size)); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					if err := cli.Publish(ctx, &Message{
						Topic:   "test",
						Payload: make([]byte, size),
					}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					if err := cli.Publish(ctx, &Message{
						Topic:   "test",
						QoS:     QoS1,
						Payload: make([]byte, size),
					}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					if err := cli.Disconnect(ctx); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
				})
			}
		})
	}
}

func TestIntegration_PublishSubscribe(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			for _, qos := range []QoS{QoS0, QoS1, QoS2} {
				t.Run(fmt.Sprintf("QoS%d", int(qos)), func(t *testing.T) {
					chReceived := make(chan *Message, 100)

					cli, err := Dial(url,
						WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
						WithConnStateHandler(func(s ConnState, err error) {
							switch s {
							case StateClosed:
								close(chReceived)
								t.Errorf("Connection is expected to be disconnected, but closed.")
							}
						}),
					)
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if _, err := cli.Connect(ctx, "PubSubClient"+name, WithCleanSession(true)); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					cli.Handle(HandlerFunc(func(msg *Message) {
						chReceived <- msg
					}))

					topic := "test_pubsub_" + name
					if err := cli.Subscribe(ctx, Subscription{Topic: topic, QoS: qos}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					if err := cli.Publish(ctx, &Message{
						Topic:   topic,
						QoS:     qos,
						Payload: []byte("message"),
					}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					select {
					case <-ctx.Done():
						t.Fatalf("Unexpected error: '%v'", ctx.Err())
					case msg, ok := <-chReceived:
						if !ok {
							t.Errorf("Connection closed unexpectedly")
							break
						}
						if msg.Topic != topic {
							t.Errorf("Expected topic name of '%s', got '%s'", topic, msg.Topic)
						}
						if !bytes.Equal(msg.Payload, []byte("message")) {
							t.Errorf("Expected payload of '%v', got '%v'", []byte("message"), msg.Payload)
						}
					}

					if err := cli.Disconnect(ctx); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
				})
			}
		})
	}
}

func TestIntegration_SubscribeUnsubscribe(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			cli, err := Dial(url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := cli.Connect(ctx, "SubUnsubClient"+name); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Subscribe(ctx, Subscription{Topic: "test", QoS: QoS2}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Unsubscribe(ctx, "test"); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Disconnect(ctx); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
		})
	}
}

func TestIntegration_Ping(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			cli, err := Dial(url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := cli.Connect(ctx, "PingClient"+name); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Ping(ctx); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Disconnect(ctx); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
		})
	}
}

func BenchmarkPublishSubscribe(b *testing.B) {
	for name, url := range urls {
		b.Run(name, func(b *testing.B) {
			chReceived := make(chan *Message, 100)

			cli, err := Dial(url,
				WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
				WithConnStateHandler(func(s ConnState, err error) {
					switch s {
					case StateClosed:
						close(chReceived)
					}
				}),
			)
			if err != nil {
				b.Fatalf("Unexpected error: '%v'", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := cli.Connect(ctx, "PubSubBenchClient"+name); err != nil {
				b.Fatalf("Unexpected error: '%v'", err)
			}

			cli.Handle(HandlerFunc(func(msg *Message) {
				chReceived <- msg
			}))

			if err := cli.Subscribe(ctx, Subscription{Topic: "test", QoS: QoS2}); err != nil {
				b.Fatalf("Unexpected error: '%v'", err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := cli.Publish(ctx, &Message{
					Topic:   "test",
					QoS:     QoS2,
					Payload: []byte("message"),
				}); err != nil {
					b.Fatalf("Unexpected error: '%v'", err)
				}

				if _, ok := <-chReceived; !ok {
					b.Fatal("Connection closed unexpectedly")
				}
			}
			b.StopTimer()

			if err := cli.Disconnect(ctx); err != nil {
				b.Fatalf("Unexpected error: '%v'", err)
			}
		})
	}
}
