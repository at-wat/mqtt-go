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

func TestProtocolViolation(t *testing.T) {
	ca, cb := net.Pipe()
	cli := &BaseClient{Transport: cb}

	errCh := make(chan error, 10)
	cli.ConnState = func(s ConnState, err error) {
		if s == StateClosed {
			errCh <- err
		}
	}

	if cli.connState.String() != "New" {
		t.Errorf("Initial state must be 'New', but is '%s'", cli.connState)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connected := make(chan struct{})
	go func() {
		if _, err := cli.Connect(ctx, "cli"); err != nil {
			if ctx.Err() == nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
		}
		if cli.connState.String() != "Active" {
			t.Errorf("State after Connect must be 'Active', but is '%s'", cli.connState)
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
		t.Error("Timeout")
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
	if cli.connState.String() != "Closed" {
		t.Errorf("Final state must be 'Closed' after protocol violation, but is '%s'", cli.connState)
	}
}
