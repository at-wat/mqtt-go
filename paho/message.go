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
	"github.com/at-wat/mqtt-go"
	paho "github.com/eclipse/paho.mqtt.golang"
)

func (c *pahoWrapper) wrapMessageHandler(h paho.MessageHandler) mqtt.Handler {
	return mqtt.HandlerFunc(func(m *mqtt.Message) {
		h(c, wrapMessage(m))
	})
}

func wrapMessage(msg *mqtt.Message) paho.Message {
	return &wrappedMessage{msg}
}

type wrappedMessage struct {
	*mqtt.Message
}

func (m *wrappedMessage) Duplicate() bool {
	return m.Message.Dup
}

func (m *wrappedMessage) Qos() byte {
	return byte(m.Message.QoS)
}

func (m *wrappedMessage) Retained() bool {
	return m.Message.Retain
}

func (m *wrappedMessage) Topic() string {
	return m.Message.Topic
}

func (m *wrappedMessage) MessageID() uint16 {
	return m.Message.ID
}

func (m *wrappedMessage) Payload() []byte {
	return m.Message.Payload
}

func (m *wrappedMessage) Ack() {
}
