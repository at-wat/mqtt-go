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
	"errors"
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
		_, _ = cli.Connect(ctx, "cli",
			WithUserNamePassword("user", "pass"),
			WithKeepAlive(0x0123),
			WithCleanSession(true),
			WithProtocolLevel(ProtocolLevel4),
			WithWill(&Message{QoS: QoS1, Topic: "topic", Payload: []byte{0x01}}),
		)
	}()

	b := make([]byte, 100)
	n, err := ca.Read(b)
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
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
		t.Fatalf("Expected CONNECT packet: \n  '%v',\ngot: \n  '%v'", expected, b[:n])
	}
	cli.Close()
}

func TestProtocolViolation(t *testing.T) {
	ca, cb := net.Pipe()
	cli := &BaseClient{Transport: cb}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		if _, err := cli.Connect(ctx, "cli"); err != nil {
			if ctx.Err() == nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
		}
	}()

	errCh := make(chan error, 10)
	cli.ConnState = func(s ConnState, err error) {
		if s == StateClosed {
			errCh <- err
		}
	}

	b := make([]byte, 100)
	if _, err := ca.Read(b); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	// Send CONNACK.
	if _, err := ca.Write([]byte{
		0x20, 0x02, 0x00, 0x00,
	}); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	// Send SUBSCRIBE from broker.
	if _, err := ca.Write([]byte{
		0x80, 0x01, 0x00,
	}); err != nil {
		t.Fatalf("Unexpected error: ''%v''", err)
	}

	select {
	case err := <-errCh:
		if err != ErrInvalidPacket {
			t.Errorf("Expected error against invalid packet: '%v', got: '%v'", ErrInvalidPacket, err)
		}
	case <-ctx.Done():
		t.Error("Timeout")
	}
}

func TestConnect_OptionsError(t *testing.T) {
	errExpected := errors.New("an error")
	sessionPresent, err := (&BaseClient{}).Connect(
		context.Background(), "cli",
		func(*ConnectOptions) error {
			return errExpected
		},
	)
	if err != errExpected {
		t.Errorf("Expected error: ''%v'', got: ''%v''", errExpected, err)
	}
	if sessionPresent {
		t.Errorf("SessionPresent flag must not be set on options error")
	}
}
