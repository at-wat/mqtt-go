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
