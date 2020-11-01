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
	"io"
	"net"
	"testing"
	"time"
)

func TestRemainingLengthParse(t *testing.T) {
	ca, cb := net.Pipe()
	cli := &BaseClient{Transport: cb}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connected := make(chan struct{})
	go func() {
		if _, err := cli.Connect(ctx, "cli"); err != nil {
			if ctx.Err() == nil {
				t.Errorf("Unexpected error: '%v'", err)
				cancel()
			}
		}
		if cli.connState.String() != "Active" {
			t.Errorf("State after Connect must be 'Active', but is '%s'", cli.connState)
			cancel()
		}
		close(connected)
	}()

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

	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("Timeout")
	}

	chMsg := make(chan *Message)
	cli.Handle(HandlerFunc(func(msg *Message) {
		chMsg <- msg
	}))

	// Send PUBLISH from broker.
	if _, err := ca.Write(
		(&pktPublish{Message: &Message{Topic: "a", Payload: make([]byte, 256)}}).Pack(),
	); err != nil {
		t.Fatalf("Unexpected error: ''%v''", err)
	}

	select {
	case msg := <-chMsg:
		if len(msg.Payload) != 256 {
			t.Errorf("Expected message payload size: 256, got: %d", len(msg.Payload))
		}
	case <-ctx.Done():
		t.Error("Timeout")
	}
}

func TestServeParseError(t *testing.T) {
	cases := map[string]struct {
		input []byte
		err   error
	}{
		"InvalidConnAck":  {[]byte{0x21, 0x00}, ErrInvalidPacket},
		"InvalidPublish":  {[]byte{0x32, 0x00}, ErrInvalidPacketLength},
		"InvalidPubAck":   {[]byte{0x41, 0x00}, ErrInvalidPacket},
		"InvalidPubRec":   {[]byte{0x51, 0x00}, ErrInvalidPacket},
		"InvalidPubRel":   {[]byte{0x61, 0x00}, ErrInvalidPacket},
		"InvalidPubComp":  {[]byte{0x71, 0x00}, ErrInvalidPacket},
		"InvalidSubAck":   {[]byte{0x91, 0x00}, ErrInvalidPacket},
		"InvalidUnsubAck": {[]byte{0xB1, 0x00}, ErrInvalidPacket},
		"InvalidPingResp": {[]byte{0xD1, 0x00}, ErrInvalidPacket},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ca, cb := net.Pipe()
			cli := &BaseClient{Transport: cb}

			go func() {
				if _, err := ca.Write(c.input); err != nil {
					t.Fatalf("Unexpected error: '%v'", err)
				}
			}()
			if err := cli.serve(); !errors.Is(err, c.err) {
				t.Fatalf("Expected error: '%v', got: '%v'", c.err, err)
			}
		})
	}
}

func TestReadPacketError(t *testing.T) {
	pkt := []byte{0x10, 0x80, 0x01}
	pkt = append(pkt, make([]byte, 128)...)

	// Ensure full packet doesn't error
	_, _, _, err := readPacket(bytes.NewReader(pkt))
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	for i := 1; i < len(pkt)-1; i++ {
		_, _, _, err := readPacket(bytes.NewReader(pkt[:i]))
		if err != io.ErrUnexpectedEOF && err != io.EOF {
			t.Fatalf(
				"Expected error for %d: '%v' or %v'', got: '%v'",
				i, io.ErrUnexpectedEOF, io.EOF, err,
			)
		}
	}
}
