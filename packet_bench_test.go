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
	"testing"
)

func BenchmarkPack(b *testing.B) {
	packets := map[string]interface{ Pack() []byte }{
		"Connect": &pktConnect{
			ProtocolLevel: ProtocolLevel4, ClientID: "client",
			UserName: "user", Password: "pass",
			Will: &Message{Topic: "topic", Payload: make([]byte, 128)},
		},
		"PubAck":      &pktPubAck{},
		"PubComp":     &pktPubComp{},
		"Publish":     &pktPublish{&Message{Topic: "topic", Payload: make([]byte, 128)}},
		"PubRec":      &pktPubRec{},
		"PubRel":      &pktPubRel{},
		"Subscribe":   &pktSubscribe{Subscriptions: []Subscription{{Topic: "topic"}}},
		"Unsubscribe": &pktUnsubscribe{Topics: []string{"topic"}},
	}
	for name, p := range packets {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = p.Pack()
			}
		})
	}
}
