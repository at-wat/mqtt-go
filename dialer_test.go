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
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

var (
	urls = map[string]string{
		"MQTT":       "mqtt://localhost:1883",
		"MQTTs":      "mqtts://localhost:8883",
		"WebSocket":  "ws://localhost:9001",
		"WebSockets": "wss://localhost:9443",
	}
)

func TestDialOptionError(t *testing.T) {
	errExpected := errors.New("an error")
	optErr := func() DialOption {
		return func(o *DialOptions) error {
			return errExpected
		}
	}

	if _, err := DialContext(
		context.TODO(), "mqtt://localhost:1883", optErr(),
	); err != errExpected {
		t.Errorf("Expected error: '%v', got: '%v'", errExpected, err)
	}
}

func TestDialOption_WithDialer(t *testing.T) {
	if _, err := DialContext(
		context.TODO(), "mqtt://localhost:1884",
		WithDialer(&net.Dialer{Timeout: time.Nanosecond}),
	); !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: '%v'", err)
	}
}

func TestDial_UnsupportedProtocol(t *testing.T) {
	if _, err := DialContext(
		context.TODO(), "unknown://localhost:1884",
	); !errors.Is(err, ErrUnsupportedProtocol) {
		t.Errorf("Expected error: '%v', got: '%v'", ErrUnsupportedProtocol, err)
	}
}

func TestDial_InvalidURL(t *testing.T) {
	if _, err := DialContext(
		context.TODO(), "://localhost",
	); !strings.Contains(err.Error(), "missing protocol scheme") {
		t.Errorf("Expected error: 'missing protocol scheme', got: '%v'", err)
	}
}

func TestDial_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			if _, err := DialContext(
				ctx, url,
			); !strings.Contains(err.Error(), "dial tcp: operation was canceled") {
				t.Errorf("Expected error: '%v', got: '%v'", context.DeadlineExceeded, err)
			}
		})
	}
}

type oldDialerImpl struct{}

func (d oldDialerImpl) Dial() (*BaseClient, error) {
	return &BaseClient{}, nil
}

func oldDialer() NoContextDialerIface {
	return &oldDialerImpl{}
}

func ExampleNoContextDialer() {
	d := oldDialer()
	cli, err := NewReconnectClient(&NoContextDialer{d})
	if err != nil {
		fmt.Println("error:", err.Error())
		return
	}
	cli.Handle(HandlerFunc(func(*Message) {}))
	fmt.Println("ok")

	// output: ok
}

func ExampleBaseClientStoreDialer() {
	dialer := &BaseClientStoreDialer{Dialer: &URLDialer{URL: "mqtt://localhost:1883"}}
	cli, err := NewReconnectClient(dialer)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := cli.Connect(ctx, "TestClient"); err != nil {
		panic(err)
	}
	defer cli.Disconnect(context.Background())

	// Publish asynchronously
	if err := cli.Publish(
		ctx, &Message{Topic: "test", QoS: QoS1, Payload: []byte("async")},
	); err != nil {
		panic(err)
	}

	// Publish synchronously
	if err := dialer.BaseClient().Publish(
		ctx, &Message{Topic: "test", QoS: QoS1, Payload: []byte("sync")},
	); err != nil {
		panic(err)
	}
}
