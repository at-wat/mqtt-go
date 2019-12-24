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
	"net/url"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func TestNotConnected(t *testing.T) {
	cli := NewClient(&paho.ClientOptions{Servers: []*url.URL{{}}})

	if ok := cli.IsConnected(); ok {
		t.Error("IsConnected must return false on disconnected client.")
	}
	if ok := cli.IsConnectionOpen(); ok {
		t.Error("IsConnectionOpen must return false on disconnected client.")
	}
	t.Run("Publish", func(t *testing.T) {
		token := cli.Publish("a", 0, false, []byte{})
		if ok := token.WaitTimeout(time.Second); !ok {
			t.Fatal("Timeout")
		}
		if token.Error() != ErrNotConnected {
			t.Errorf("'%v' must be returned on disconnected client, got: '%v'",
				ErrNotConnected, token.Error(),
			)
		}
	})
	t.Run("Subscribe", func(t *testing.T) {
		token := cli.Subscribe("a", 0, func(paho.Client, paho.Message) {})
		if ok := token.WaitTimeout(time.Second); !ok {
			t.Fatal("Timeout")
		}
		if token.Error() != ErrNotConnected {
			t.Errorf("'%v' must be returned on disconnected client, got: '%v'",
				ErrNotConnected, token.Error(),
			)
		}
	})
	t.Run("SubscribeMultiple", func(t *testing.T) {
		token := cli.SubscribeMultiple(
			map[string]byte{"a": 0},
			func(paho.Client, paho.Message) {},
		)
		if ok := token.WaitTimeout(time.Second); !ok {
			t.Fatal("Timeout")
		}
		if token.Error() != ErrNotConnected {
			t.Errorf("'%v' must be returned on disconnected client, got: '%v'",
				ErrNotConnected, token.Error(),
			)
		}
	})
	t.Run("Unsubscribe", func(t *testing.T) {
		token := cli.Unsubscribe("a")
		if ok := token.WaitTimeout(time.Second); !ok {
			t.Fatal("Timeout")
		}
		if token.Error() != ErrNotConnected {
			t.Errorf("'%v' must be returned on disconnected client, got: '%v'",
				ErrNotConnected, token.Error(),
			)
		}
	})
}
