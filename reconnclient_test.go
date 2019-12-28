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
	"testing"
	"time"
)

func TestReconnectClient_DefaultOptions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	keepAlive := 11

	cli, err := NewReconnectClient(
		&URLDialer{URL: "mqtt://localhost:1883"},
	)
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	_, err = cli.Connect(ctx,
		"ReconnectClient",
		WithKeepAlive(uint16(keepAlive)),
	)
	reconnCli := cli.(*reconnectClient)
	if int(reconnCli.options.Timeout/time.Second) != keepAlive {
		t.Errorf("Default Timeout should be same as KeepAlice, expected: %d, got %d",
			11, reconnCli.options.Timeout/time.Second,
		)
	}
	if int(reconnCli.options.PingInterval/time.Second) != keepAlive {
		t.Errorf("Default PingInterval should be same as KeepAlice, expected: %d, got %d",
			11, reconnCli.options.PingInterval/time.Second,
		)
	}
}
