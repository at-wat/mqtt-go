// +build integration

package mqtt

import (
	"context"
	"crypto/tls"
	"testing"
	"time"
)

func TestIntegration_RetryClient(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			cliBase, err := Dial(url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var cli RetryClient
			cli.SetClient(ctx, cliBase)

			if _, err := cli.Connect(ctx, "RetryClient"+name); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Publish(ctx, &Message{
				Topic:   "test",
				QoS:     QoS1,
				Payload: []byte("message"),
			}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := cli.Disconnect(ctx); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
		})
	}

}
