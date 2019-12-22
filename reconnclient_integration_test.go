// +build integration

package mqtt

import (
	"context"
	"crypto/tls"
	"testing"
	"time"
)

func TestIntegration_ReconnectClient(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cli := NewReconnectClient(
				ctx,
				&URLDialer{
					URL: url,
					Options: []DialOption{
						WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
					},
				},
				"ReconnectClient",
			)

			for i := 0; i < 10; i++ {
				if err := cli.Publish(ctx, &Message{
					Topic:   "test",
					QoS:     QoS1,
					Payload: []byte("message"),
				}); err != nil {
					t.Fatalf("Unexpected error: '%v'", err)
				}
			}
		})
	}

}
