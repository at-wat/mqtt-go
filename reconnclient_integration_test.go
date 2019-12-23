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

			chReceived := make(chan *Message, 100)
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
			cli.Handle(HandlerFunc(func(msg *Message) {
				chReceived <- msg
			}))

			// Close underlying client.
			cli.(*reconnectClient).Client.(*RetryClient).Client.(ClientCloser).Close()

			if err := cli.Subscribe(ctx, Subscription{Topic: "test", QoS: QoS1}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			if err := cli.Publish(ctx, &Message{
				Topic:   "test",
				QoS:     QoS1,
				Retain:  true,
				Payload: []byte("message"),
			}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			time.Sleep(time.Second)

			select {
			case <-ctx.Done():
				t.Fatalf("Unexpected error: '%v'", ctx.Err())
			case <-chReceived:
			}
		})
	}

}
