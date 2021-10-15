//go:build integration
// +build integration

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
	"context"
	"testing"
	"time"
)

func TestIntegration_WithTLSCertFiles(t *testing.T) {
	// Overwrite default port to avoid using privileged port during test.
	defaultPorts["ws"] = 9001
	defaultPorts["wss"] = 9443

	cases := map[string]struct {
		opt         DialOption
		expectError bool
	}{
		"NoCA": {
			WithTLSCertFiles(
				"localhost", "unexist-file", "integration/test.crt", "integration/test.key",
			),
			true,
		},
		"NoCert": {
			WithTLSCertFiles(
				"localhost", "integration/ca.crt", "unexist-file", "integration/test.key",
			),
			true,
		},
		"NoKey": {
			WithTLSCertFiles(
				"localhost", "integration/ca.crt", "integration/test.crt", "unexist-file",
			),
			true,
		},
		"Valid": {
			WithTLSCertFiles(
				"localhost", "integration/ca.crt", "integration/test.crt", "integration/test.key",
			),
			false,
		},
	}
	test := func(t *testing.T, urls map[string]string) {
		for name, c := range cases {
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				cli, err := DialContext(ctx, urls["MQTTs"], c.opt)

				if err != nil {
					if c.expectError {
						return
					}
					t.Fatal(err)
				}
				defer cli.Close()

				if c.expectError {
					t.Fatal("Expected error but succeeded")
				}

				if _, err := cli.Connect(ctx, "TestConnTLS", WithCleanSession(true)); err != nil {
					t.Error(err)
				}
				if err := cli.Disconnect(ctx); err != nil {
					t.Error(err)
				}
			})
		}
	}
	t.Run("WithPort", func(t *testing.T) {
		test(t, urls)
	})
	t.Run("WithoutPort", func(t *testing.T) {
		test(t, urlsWithoutPort)
	})
}
