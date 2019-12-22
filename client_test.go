package mqtt

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"
)

func TestConnect(t *testing.T) {
	ca, cb := net.Pipe()
	cli := &BaseClient{Transport: cb}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = cli.Connect(ctx, "cli",
			WithUserNamePassword("user", "pass"),
			WithKeepalive(0x0123),
			WithCleanSession(true),
			WithWill(&Message{QoS: QoS1, Topic: "topic", Payload: []byte{0x01}}),
		)
	}()

	b := make([]byte, 100)
	n, err := ca.Read(b)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := []byte{
		0x10, // CONNECT
		0x25,
		0x00, 0x04, 0x4D, 0x51, 0x54, 0x54, // MQTT
		0x04,             // 3.1.1
		0xCE, 0x01, 0x23, // flags, keepalive
		0x00, 0x03, 0x63, 0x6C, 0x69, // cli
		0x00, 0x05, 0x74, 0x6F, 0x70, 0x69, 0x63, // topic
		0x00, 0x01, 0x01, // payload
		0x00, 0x04, 0x75, 0x73, 0x65, 0x72, // user
		0x00, 0x04, 0x70, 0x61, 0x73, 0x73, // pass
	}
	if !bytes.Equal(expected, b[:n]) {
		t.Fatalf("Expected CONNECT packet: \n  %v,\ngot: \n  %v", expected, b[:n])
	}
	cli.Close()
}
