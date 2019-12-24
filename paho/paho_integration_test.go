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
	"bytes"
	"net/url"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func TestIntegration_PublishSubscribe(t *testing.T) {
	opts := paho.NewClientOptions()
	server, err := url.Parse("mqtt://localhost:1883")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	opts.Servers = []*url.URL{server}
	opts.AutoReconnect = false
	opts.ClientID = "PahoWrapper"
	opts.KeepAlive = 0

	cli := NewClient(opts)
	token := cli.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("Connect timeout")
	}
	msg := make(chan paho.Message, 100)
	token = cli.Subscribe("paho", 1, func(c paho.Client, m paho.Message) {
		msg <- m
	})
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("Subscribe timeout")
	}
	token = cli.Publish("paho", 1, false, []byte{0x12})
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("Publish timeout")
	}

	select {
	case m := <-msg:
		if m.Topic() != "paho" {
			t.Errorf("Expected topic: 'topic', got: %s", m.Topic())
		}
		if !bytes.Equal(m.Payload(), []byte{0x12}) {
			t.Errorf("Expected payload: [18], got: %v", m.Payload())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Message timeout")
	}
	cli.Disconnect(10)
}
