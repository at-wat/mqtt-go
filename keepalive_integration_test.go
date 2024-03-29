//go:build integration
// +build integration

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
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"
)

func TestIntegration_KeepAlive(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			cli, err := DialContext(ctx, url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if _, err := cli.Connect(ctx, "KeepAliveClient"+name,
				WithKeepAlive(1),
			); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			// Without keepalive, broker should disconnect on t=1.5s.
			if err := KeepAlive(
				ctx, cli,
				time.Second,
				500*time.Millisecond,
			); !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Expected error: '%v', got: '%v'", context.DeadlineExceeded, err)

				if err := cli.Disconnect(ctx); err != nil {
					t.Fatalf("Unexpected error: '%v'", err)
				}
			}

			if c := cli.Stats().CountPingError; c != 1 {
				t.Errorf("CountPingError must be 1, got %v", c)
			}
		})
	}
}
