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
	"testing"
)

func TestPublish_ParseError(t *testing.T) {
	cases := []struct {
		flag     byte
		contents []byte
		err      error
	}{
		{0x00, []byte{0x00, 0x01, 0x61, 0x00, 0x00}, nil},
		{0x00, []byte{0x00, 0x01, 0x00, 0x00, 0x00}, ErrInvalidRune},
		{0x06, []byte{0x00, 0x01, 0x61, 0x00, 0x00}, ErrInvalidPacket},
		{0x02, []byte{0x00, 0x01, 0x01}, ErrInvalidPacketLength},
		{0x00, []byte{0x00, 0x01}, ErrInvalidPacketLength},
		{0x00, []byte{0x00}, ErrInvalidPacketLength},
	}

	for _, c := range cases {
		_, err := (&pktPublish{}).Parse(c.flag, c.contents)
		if !errors.Is(err, c.err) {
			t.Errorf("Parsing packet with flag=%x, contents=%v expected error: %v, got: %v",
				c.flag, c.contents,
				c.err, err,
			)
		}
	}
}

func TestPublish_MessageValidation(t *testing.T) {
	cli := &BaseClient{MaxPayloadLen: 100}

	cases := []struct {
		message *Message
		err     error
	}{
		{&Message{Payload: make([]byte, 101)}, ErrPayloadLenExceeded},
		{&Message{QoS: 3}, ErrInvalidQoS},
	}

	for _, c := range cases {
		if err := cli.Publish(
			context.Background(), c.message,
		); !errors.Is(err, c.err) {
			t.Errorf("Publishing packet %+v expected error: %v, got: %v",
				c.message, c.err, err,
			)
		}
	}
}
