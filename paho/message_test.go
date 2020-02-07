// Copyright 2020 The mqtt-go authors.
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
	"testing"

	"github.com/at-wat/mqtt-go"
	paho "github.com/eclipse/paho.mqtt.golang"
)

func TestWrapMessageHandler(t *testing.T) {
	msg := make(chan paho.Message, 100)
	ph := func(c paho.Client, m paho.Message) {
		msg <- m
	}
	h := (&pahoWrapper{}).wrapMessageHandler(ph)

	h.Serve(&mqtt.Message{
		Topic:   "topic",
		QoS:     mqtt.QoS1,
		Payload: []byte{0x01, 0x02},
		Dup:     true,
		Retain:  true,
		ID:      0x1234,
	})

	if len(msg) != 1 {
		t.Fatalf("Expected number of handled messages: 1, got: %d", len(msg))
	}
	m := <-msg
	if m.Topic() != "topic" {
		t.Errorf("Expected topic: 'topic', got: '%s'", m.Topic())
	}
	if m.Qos() != 1 {
		t.Errorf("Expected QoS: 1, got: %d", m.Qos())
	}
	if !bytes.Equal([]byte{0x01, 0x02}, m.Payload()) {
		t.Errorf("Expected payload: [1, 2], got: %v", m.Payload())
	}
	if !m.Duplicate() {
		t.Errorf("Expected dup: true, got: %v", m.Duplicate())
	}
	if !m.Retained() {
		t.Errorf("Expected retain: true, got: %v", m.Retained())
	}
	if m.MessageID() != 0x1234 {
		t.Errorf("Expected ID: 1234, got: %x", m.MessageID())
	}
}
