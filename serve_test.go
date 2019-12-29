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
		(&pktPublish{Message: &Message{Topic: "a", Payload: make([]byte, 256)}}).pack(),
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
