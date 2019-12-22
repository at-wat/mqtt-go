// +build integration

package mqtt

import (
	"context"
	"crypto/tls"
	"testing"
	"time"
)

func TestIntegration_KeepAlive(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			cli, err := Dial(url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			if _, err := cli.Connect(ctx, "Client1",
				WithKeepAlive(1),
			); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			// Without keepalive, broker should disconnect on t=1.5s.
			if err := KeepAlive(ctx, cli, time.Second); err != context.DeadlineExceeded {
				t.Errorf("Expected error: '%v', got: '%v'", context.DeadlineExceeded, err)

				if err := cli.Disconnect(ctx); err != nil {
					t.Fatalf("Unexpected error: '%v'", err)
				}
			}
		})
	}

}
