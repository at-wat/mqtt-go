// +build integration

package mqtt

import (
	"bytes"
	"context"
	"crypto/tls"
	"testing"
)

func TestIntegration_Connect(t *testing.T) {
	cli, err := Dial("mqtt://localhost:1883")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_ConnectTLS(t *testing.T) {
	cli, err := Dial("tls://localhost:8883",
		WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_ConnectWebSocket(t *testing.T) {
	cli, err := Dial("ws://localhost:9001")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_ConnectWebSockets(t *testing.T) {
	cli, err := Dial("wss://localhost:9443",
		WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_PublishQoS0(t *testing.T) {
	cli, err := Dial("mqtt://localhost:1883")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Publish(ctx, &Message{
		Topic:   "test",
		Payload: []byte("message"),
	}); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_PublishQoS1(t *testing.T) {
	cli, err := Dial("mqtt://localhost:1883")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
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
}

func TestIntegration_PublishQoS2_SubscribeQoS1(t *testing.T) {
	cli, err := Dial("mqtt://localhost:1883")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Subscribe(ctx, Subscription{Topic: "test", QoS: QoS1}); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Publish(ctx, &Message{
		Topic:   "test",
		QoS:     QoS2,
		Payload: []byte("message"),
	}); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_PublishQoS2_SubscribeQoS2(t *testing.T) {
	cli, err := Dial("mqtt://localhost:1883")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	chReceived := make(chan *Message, 1)
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

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
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
}

func TestIntegration_SubscribeUnsubscribe(t *testing.T) {
	cli, err := Dial("mqtt://localhost:1883")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
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
}

func TestIntegration_Ping(t *testing.T) {
	cli, err := Dial("mqtt://localhost:1883")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx := context.Background()
	if err := cli.Connect(ctx, "Client1"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Ping(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}
