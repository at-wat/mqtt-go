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
	cases := map[string]struct {
		opts     []ConnectOption
		expected []byte
	}{
		"UserPassCleanWill": {
			opts: []ConnectOption{
				WithUserNamePassword("user", "pass"),
				WithKeepAlive(0x0123),
				WithCleanSession(true),
				WithProtocolLevel(ProtocolLevel4),
				WithWill(&Message{QoS: QoS1, Topic: "topic", Payload: []byte{0x01}}),
			},
			expected: []byte{
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
			},
		},
		"WillQoS0": {
			opts: []ConnectOption{
				WithKeepAlive(0x0123),
				WithWill(&Message{QoS: QoS0, Topic: "topic", Payload: []byte{0x01}}),
			},
			expected: []byte{
				0x10, // CONNECT
				0x19,
				0x00, 0x04, 0x4D, 0x51, 0x54, 0x54, // MQTT
				0x04,             // 3.1.1
				0x04, 0x01, 0x23, // flags, keepalive
				0x00, 0x03, 0x63, 0x6C, 0x69, // cli
				0x00, 0x05, 0x74, 0x6F, 0x70, 0x69, 0x63, // topic
				0x00, 0x01, 0x01, // payload
			},
		},
		"WillQoS2Retain": {
			opts: []ConnectOption{
				WithKeepAlive(0x0123),
				WithWill(&Message{QoS: QoS2, Retain: true, Topic: "topic", Payload: []byte{0x01}}),
			},
			expected: []byte{
				0x10, // CONNECT
				0x19,
				0x00, 0x04, 0x4D, 0x51, 0x54, 0x54, // MQTT
				0x04,             // 3.1.1
				0x34, 0x01, 0x23, // flags, keepalive
				0x00, 0x03, 0x63, 0x6C, 0x69, // cli
				0x00, 0x05, 0x74, 0x6F, 0x70, 0x69, 0x63, // topic
				0x00, 0x01, 0x01, // payload
			},
		},
		"ProtocolLv3": {
			opts: []ConnectOption{
				WithKeepAlive(0x0123),
				WithProtocolLevel(ProtocolLevel3),
			},
			expected: []byte{
				0x10, // CONNECT
				0x0F,
				0x00, 0x04, 0x4D, 0x51, 0x54, 0x54, // MQTT
				0x03,             // 3.1.1
				0x00, 0x01, 0x23, // flags, keepalive
				0x00, 0x03, 0x63, 0x6C, 0x69, // cli
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ca, cb := net.Pipe()
			cli := &BaseClient{Transport: cb}

			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_, _ = cli.Connect(ctx, "cli", c.opts...)
			}()

			b := make([]byte, 100)
			n, err := ca.Read(b)
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if !bytes.Equal(c.expected, b[:n]) {
				t.Fatalf("Expected CONNECT packet: \n  '%v',\ngot: \n  '%v'", c.expected, b[:n])
			}
			cli.Close()
		})
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
