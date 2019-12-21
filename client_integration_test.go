// +build integration

package mqtt

import (
	"bytes"
	"context"
	"testing"
)

func TestIntegration_Connect(t *testing.T) {
	cli := &Client{}
	if err := cli.Dial("mqtt://localhost:1883"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	go cli.Serve()

	ctx := context.Background()
	err := cli.Connect(ctx, "Client1")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_PublishQoS0(t *testing.T) {
	cli := &Client{}
	if err := cli.Dial("mqtt://localhost:1883"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	go cli.Serve()

	ctx := context.Background()
	err := cli.Connect(ctx, "Client1")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	err = cli.Publish(ctx, &Message{
		Topic:   "test",
		Payload: []byte("message"),
	})
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_PublishQoS1(t *testing.T) {
	cli := &Client{}
	if err := cli.Dial("mqtt://localhost:1883"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	go cli.Serve()

	ctx := context.Background()
	err := cli.Connect(ctx, "Client1")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	err = cli.Publish(ctx, &Message{
		Topic:   "test",
		QoS:     QoS1,
		Payload: []byte("message"),
	})
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_PublishQoS2_SubscribeQoS1(t *testing.T) {
	cli := &Client{}
	if err := cli.Dial("mqtt://localhost:1883"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	go cli.Serve()

	ctx := context.Background()
	err := cli.Connect(ctx, "Client1")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	err = cli.Subscribe(ctx, []*Message{
		{
			Topic: "test",
			QoS:   QoS1,
		},
	})
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	err = cli.Publish(ctx, &Message{
		Topic:   "test",
		QoS:     QoS2,
		Payload: []byte("message"),
	})
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_PublishQoS2_SubscribeQoS2(t *testing.T) {
	cli := &Client{}
	if err := cli.Dial("mqtt://localhost:1883"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	go cli.Serve()

	ctx := context.Background()
	err := cli.Connect(ctx, "Client1")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	chReceived := make(chan *Message, 100)
	cli.Handler = HandlerFunc(func(msg *Message) {
		chReceived <- msg
	})

	err = cli.Subscribe(ctx, []*Message{
		{
			Topic: "test",
			QoS:   QoS2,
		},
	})
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	err = cli.Publish(ctx, &Message{
		Topic:   "test",
		QoS:     QoS2,
		Payload: []byte("message"),
	})
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	select {
	case msg := <-chReceived:
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
	cli := &Client{}
	if err := cli.Dial("mqtt://localhost:1883"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	go cli.Serve()

	ctx := context.Background()
	err := cli.Connect(ctx, "Client1")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	err = cli.Subscribe(ctx, []*Message{
		{
			Topic: "test",
			QoS:   QoS2,
		},
	})
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	err = cli.Unsubscribe(ctx, []string{
		"test",
	})
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_Ping(t *testing.T) {
	cli := &Client{}
	if err := cli.Dial("mqtt://localhost:1883"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	go cli.Serve()

	ctx := context.Background()
	err := cli.Connect(ctx, "Client1")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	err = cli.Ping(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}
