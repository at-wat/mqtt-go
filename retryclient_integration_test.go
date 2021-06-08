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
	"sync/atomic"
	"testing"
	"time"

	"github.com/at-wat/mqtt-go/internal/filteredpipe"
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
	if _, err := cliRecv.Subscribe(ctx, Subscription{Topic: "testCancel", QoS: QoS2}); err != nil {
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

func TestIntegration_RetryClient_TaskQueue(t *testing.T) {
	cliBase, err := Dial(urls["MQTT"], WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var cli RetryClient
	var cnt int

	publish := func() {
		if err := cli.Publish(ctx, &Message{
			Topic:   "test/queue",
			QoS:     QoS0,
			Payload: []byte("message"),
		}); err != nil {
			t.Errorf("Unexpected error: '%v' (cnt=%d)", err, cnt)
			return
		}
		select {
		case <-ctx.Done():
			t.Errorf("Timeout (cnt=%d)", cnt)
		default:
		}
	}

	// Must be queued and done after Connect.
	for i := 0; i < 10; i++ {
		publish()
	}

	cli.SetClient(ctx, cliBase)

	time.Sleep(100 * time.Millisecond)

	if _, err := cli.Connect(ctx, "RetryClientQueue"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	ctxDone, done := context.WithCancel(context.Background())
	defer done()

	cli.Handle(HandlerFunc(func(msg *Message) {
		if err := cli.Publish(ctx, &Message{
			Topic:   "test/queue_response",
			QoS:     QoS1,
			Payload: []byte("message"),
		}); err != nil {
			t.Errorf("Unexpected error: '%v'", err)
			return
		}
		cnt++
		if cnt == 100 {
			done()
		}
	}))
	if _, err := cli.Subscribe(ctx, Subscription{Topic: "test/queue", QoS: QoS1}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	func() {
		for i := 0; i < 90; i++ {
			publish()
		}
	}()

	select {
	case <-ctx.Done():
		t.Errorf("Timeout (cnt=%d)", cnt)
	case <-ctxDone.Done():
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_RetryClient_RetryInitialRequest(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			topic := "test/RetryInitialReq" + name
			var sw int32

			cli, err := NewReconnectClient(
				DialerFunc(func() (*BaseClient, error) {
					cli, err := Dial(url,
						WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
					)
					if err != nil {
						return nil, err
					}
					ca, cb := filteredpipe.DetectAndClosePipe(
						newOnOffFilter(&sw),
						newOnOffFilter(&sw),
					)
					filteredpipe.Connect(ca, cli.Transport)
					cli.Transport = cb
					return cli, nil
				}),
				WithReconnectWait(50*time.Millisecond, 200*time.Millisecond),
				WithPingInterval(250*time.Millisecond),
				WithTimeout(250*time.Millisecond),
			)
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if _, err := cli.Subscribe(ctx, Subscription{Topic: topic, QoS: QoS1}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)

			// Disconnect
			atomic.StoreInt32(&sw, 1)
			go func() {
				time.Sleep(300 * time.Millisecond)
				// Connect
				atomic.StoreInt32(&sw, 0)
			}()

			if _, err := cli.Connect(ctx, "RetryInitialReq"+name); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := ctx.Err(); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			cli.Disconnect(ctx)
		})
	}
}
