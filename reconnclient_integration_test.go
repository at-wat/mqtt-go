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
			// Close underlying client.
			cli.(*reconnectClient).Client.(*RetryClient).Client.(ClientCloser).Close()

			if err := cli.Publish(ctx, &Message{
				Topic:   "test",
				QoS:     QoS1,
				Retain:  true,
				Payload: []byte("message"),
			}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			time.Sleep(2 * time.Second)

			chReceived := make(chan *Message, 100)
			cli.(*reconnectClient).Client.(*RetryClient).Client.(*BaseClient).Handler = HandlerFunc(func(msg *Message) {
				chReceived <- msg
			})
			if err := cli.Subscribe(ctx, Subscription{Topic: "test", QoS: QoS1}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			select {
			case <-ctx.Done():
				t.Fatalf("Unexpected error: '%v'", ctx.Err())
			case <-chReceived:
			}
		})
	}

}
