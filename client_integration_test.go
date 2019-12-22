// +build integration

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
	baseCli.Handler = HandlerFunc(func(msg *Message) {
		fmt.Printf("%s[%d]: %s", msg.Topic, int(msg.QoS), []byte(msg.Payload))
		close(done)
	})

	// store as Client to make it easy to enable high level wrapper later
	var cli Client = baseCli
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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
			if _, err := cli.Connect(ctx, "Client1"); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Disconnect(ctx); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
		})
	}
}

func TestIntegration_Publish(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			cli, err := Dial(url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := cli.Connect(ctx, "Client1"); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Publish(ctx, &Message{
				Topic:   "test",
				Payload: []byte("message"),
			}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			if err := cli.Publish(ctx, &Message{
				Topic:   "test",
				QoS:     QoS1,
				Payload: []byte("message"),
			}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Disconnect(ctx); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
		})
	}
}

func TestIntegration_PublishQoS2_SubscribeQoS2(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			cli, err := Dial(url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			chReceived := make(chan *Message, 100)
			cli.Handler = HandlerFunc(func(msg *Message) {
				chReceived <- msg
			})
			cli.ConnState = func(s ConnState, err error) {
				switch s {
				case StateActive:
				case StateClosed:
					close(chReceived)
					t.Errorf("Connection is expected to be disconnected, but closed.")
				case StateDisconnected:
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := cli.Connect(ctx, "Client1"); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Subscribe(ctx, Subscription{Topic: "test", QoS: QoS2}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Publish(ctx, &Message{
				Topic:   "test",
				QoS:     QoS2,
				Payload: []byte("message"),
			}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			select {
			case msg, ok := <-chReceived:
				if !ok {
					t.Errorf("Connection closed unexpectedly")
					break
				}
				if msg.Topic != "test" {
					t.Errorf("Expected topic name of 'test', got '%s'", msg.Topic)
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
			if _, err := cli.Connect(ctx, "Client1"); err != nil {
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
			if _, err := cli.Connect(ctx, "Client1"); err != nil {
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
			cli, err := Dial(url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				b.Fatalf("Unexpected error: '%v'", err)
			}

			chReceived := make(chan *Message, 100)
			cli.Handler = HandlerFunc(func(msg *Message) {
				chReceived <- msg
			})
			cli.ConnState = func(s ConnState, err error) {
				switch s {
				case StateActive:
				case StateClosed:
					close(chReceived)
				case StateDisconnected:
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := cli.Connect(ctx, "Client1"); err != nil {
				b.Fatalf("Unexpected error: '%v'", err)
			}

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
