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

func TestIntegration_RetryClient_Cancel(t *testing.T) {
	cliBase, err := Dial(urls["MQTT"], WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cliRecv, err := Dial(
		urls["MQTT"],
		WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	if _, err = cliRecv.Connect(ctx,
		"RetryClientCancelRecv",
	); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	chRecv := make(chan *Message)
	cliRecv.Handle(HandlerFunc(func(msg *Message) {
		chRecv <- msg
	}))
	if err := cliRecv.Subscribe(ctx, Subscription{Topic: "testCancel", QoS: QoS2}); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	var cli RetryClient
	cli.SetClient(ctx, cliBase)

	if _, err := cli.Connect(ctx, "RetryClientCancel"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx2, cancel2 := context.WithCancel(ctx)
	if err := cli.Publish(ctx2, &Message{
		Topic:   "testCancel",
		QoS:     QoS2,
		Payload: []byte("message"),
	}); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	// Job must be already queued now.
	cancel2()

	select {
	case <-chRecv:
	case <-time.After(time.Second):
		t.Error("Timeout")
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	if err := cliRecv.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}
