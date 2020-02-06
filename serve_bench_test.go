package mqtt

import (
	"bytes"
	"fmt"
	"testing"
)

func BenchmarkReadPacket(b *testing.B) {
	for _, l := range []int{16, 256, 65536} {
		b.Run(fmt.Sprintf("%dBytes", l), func(b *testing.B) {
			data := (&pktPublish{Message: &Message{
				Topic:   "topicString",
				Payload: make([]byte, l),
			}}).pack()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, _, err := readPacket(bytes.NewReader(data))
				if err != nil {
					b.Fatalf("Unexpected error: '%v'", err)
				}
			}
		})
	}
}
