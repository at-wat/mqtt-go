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
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/at-wat/mqtt-go/internal/filteredpipe"
)

func expectRetryStats(t *testing.T, expected, actual RetryStats) {
	t.Helper()
	if expected.QueuedTasks != actual.QueuedTasks {
		t.Errorf("Expected queued tasks: %d, actual: %d", expected.QueuedTasks, actual.QueuedTasks)
	}
	if expected.QueuedRetries != actual.QueuedRetries {
		t.Errorf("Expected queued retries: %d, actual: %d", expected.QueuedRetries, actual.QueuedRetries)
	}
	if expected.TotalTasks != actual.TotalTasks {
		t.Errorf("Expected total tasks: %d, actual: %d", expected.TotalTasks, actual.TotalTasks)
	}
	if expected.TotalRetries != actual.TotalRetries {
		t.Errorf("Expected total retries: %d, actual: %d", expected.TotalRetries, actual.TotalRetries)
	}
	if expected.CountSetClients != actual.CountSetClients {
		t.Errorf("Expected count of SetClient: %d, actual: %d", expected.CountSetClients, actual.CountSetClients)
	}
}

func TestIntegration_RetryClient(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cliBase, err := DialContext(ctx, url, WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

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

			time.Sleep(50 * time.Millisecond)
			expectRetryStats(t, RetryStats{
				TotalTasks:      1,
				CountSetClients: 1,
			}, cli.Stats())

			if err := cli.Disconnect(ctx); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
		})
	}
}

func TestIntegration_RetryClient_Cancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cliBase, err := DialContext(ctx, urls["MQTT"], WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	cliRecv, err := DialContext(
		ctx, urls["MQTT"],
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
	type pubTiming string
	const (
		pubBeforeSetClient pubTiming = "BeforeSetClient"
		pubBeforeConnect   pubTiming = "BeforeConnect"
		pubAfterConnect    pubTiming = "AfterConnect"
	)
	pubTimings := []pubTiming{
		pubBeforeSetClient, pubBeforeConnect, pubAfterConnect,
	}

	for _, withWait := range []bool{true, false} {
		name := "WithoutWait"
		if withWait {
			name = "WithWait"
		}
		withWait := withWait
		t.Run(name, func(t *testing.T) {
			for _, pubAt := range pubTimings {
				pubAt := pubAt
				t.Run(string(pubAt), func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					ctxDone, done := context.WithCancel(context.Background())
					defer done()

					var cnt int
					const expectedCount = 100

					cliRecv, err := DialContext(ctx, urls["MQTT"], WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					if _, err := cliRecv.Connect(ctx, "RetryClientQueueRecv"); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					if _, err := cliRecv.Subscribe(ctx, Subscription{Topic: "test/queue", QoS: QoS1}); err != nil {
						t.Fatal(err)
					}
					cliRecv.Handle(HandlerFunc(func(*Message) {
						cnt++
						if cnt == expectedCount {
							done()
						}
					}))

					cliBase, err := DialContext(ctx, urls["MQTT"], WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					var cli RetryClient
					publish := func() {
						for i := 0; i < expectedCount; i++ {
							if err := cli.Publish(ctx, &Message{
								Topic:   "test/queue",
								QoS:     QoS1,
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
					}

					if pubAt == pubBeforeSetClient {
						publish()
					}
					if withWait {
						time.Sleep(50 * time.Millisecond)
					}

					cli.SetClient(ctx, cliBase)

					if withWait {
						time.Sleep(50 * time.Millisecond)
					}
					// Ensure there is no deadlock when SetClient before Connect.
					cli.SetClient(ctx, cliBase)

					if pubAt == pubBeforeConnect {
						publish()
					}
					if withWait {
						time.Sleep(50 * time.Millisecond)

						if pubAt == pubBeforeSetClient || pubAt == pubBeforeConnect {
							expectRetryStats(t, RetryStats{
								QueuedTasks:     100,
								CountSetClients: 2,
							}, cli.Stats())
						}
					}

					if _, err := cli.Connect(ctx, "RetryClientQueue"); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					if pubAt == pubAfterConnect {
						publish()
					}

					select {
					case <-ctx.Done():
						t.Errorf("Timeout (cnt=%d)", cnt)
					case <-ctxDone.Done():
					}

					expectRetryStats(t, RetryStats{
						TotalTasks:      100,
						CountSetClients: 2,
					}, cli.Stats())

					if err := cli.Disconnect(ctx); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
				})
			}
		})
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
				DialerFunc(func(ctx context.Context) (*BaseClient, error) {
					cli, err := DialContext(ctx, url,
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

			expectRetryStats(t, RetryStats{
				QueuedTasks: 1,
			}, cli.Stats())

			// Disconnect
			atomic.StoreInt32(&sw, 1)
			go func() {
				time.Sleep(300 * time.Millisecond)

				if name == "MQTT" || name == "MQTTs" {
					// Mosquitto WebSocket sometimes requires extra time to connect
					// and retry number may be increased.
					expectRetryStats(t, RetryStats{
						TotalTasks:      1, // first try to subscribe (failed)
						QueuedRetries:   1,
						CountSetClients: 3,
					}, cli.Stats())
				}

				// Connect
				atomic.StoreInt32(&sw, 0)
			}()

			if _, err := cli.Connect(ctx, "RetryInitialReq"+name); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if err := ctx.Err(); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			if name == "MQTT" || name == "MQTTs" {
				// Mosquitto WebSocket sometimes requires extra time to connect
				// and retry number may be increased.
				time.Sleep(50 * time.Millisecond)
				stats := cli.Stats()
				if stats.QueuedTasks != 0 {
					t.Errorf("Expected no queued tasks, actual: %d", stats.QueuedTasks)
				}
				if stats.QueuedRetries != 0 {
					t.Errorf("Expected no queued retries, actual: %d", stats.QueuedRetries)
				}
				if stats.TotalTasks < 2 {
					t.Errorf("Expected total tasks: at least 2, actual: %d", stats.TotalTasks)
				}
				if stats.TotalRetries < 1 {
					t.Errorf("Expected total retries: at least 1, actual: %d", stats.TotalRetries)
				}
			}

			cli.Disconnect(ctx)
		})
	}
}

func baseCliRemovePacket(ctx context.Context, t *testing.T, packetPass func([]byte) bool) *BaseClient {
	t.Helper()
	cliBase, err := DialContext(ctx, urls["MQTT"], WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	ca, cb := net.Pipe()
	connOrig := cliBase.Transport
	cliBase.Transport = cb
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ca.Read(buf)
			if err != nil {
				return
			}
			if !packetPass(buf[:n]) {
				continue
			}
			if _, err := connOrig.Write(buf[:n]); err != nil {
				return
			}
		}
	}()
	go func() {
		io.Copy(ca, connOrig)
	}()
	return cliBase
}

func TestIntegration_RetryClient_ResponseTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cli := &RetryClient{
		ResponseTimeout: 50 * time.Millisecond,
	}
	cli.SetClient(ctx, baseCliRemovePacket(ctx, t, func(b []byte) bool {
		return b[0] != byte(packetPublish)|byte(publishFlagQoS1)
	}))

	if _, err := cli.Connect(ctx, "RetryClientTimeout"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Publish(ctx, &Message{
		Topic:   "test/ResponseTimeout",
		QoS:     QoS1,
		Payload: []byte("message"),
	}); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Timeout")
	case <-cli.Client().Done():
		// Client must be closed due to response timeout.
	}
	expectRetryStats(t, RetryStats{
		QueuedRetries:   1,
		TotalTasks:      1,
		CountSetClients: 1,
	}, cli.Stats())

	cli.SetClient(ctx, baseCliRemovePacket(ctx, t, func([]byte) bool {
		return true
	}))
	cli.Retry(ctx)
	if _, err := cli.Connect(ctx, "RetryClientTimeout"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	time.Sleep(150 * time.Millisecond)
	expectRetryStats(t, RetryStats{
		TotalRetries:    1,
		TotalTasks:      2,
		CountSetClients: 2,
	}, cli.Stats())

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}

func TestIntegration_RetryClient_DirectlyPublishQoS0(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cli := &RetryClient{
		DirectlyPublishQoS0: true,
	}

	publishQoS0Msg := make(chan struct{})
	cli.SetClient(ctx, baseCliRemovePacket(ctx, t, func(b []byte) bool {
		if b[0] == byte(packetPublish) {
			close(publishQoS0Msg)
		}
		return b[0] != byte(packetPublish)|byte(publishFlagQoS1)
	}))

	if _, err := cli.Connect(ctx, "RetryClientDirectlyPublishQoS0"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if err := cli.Publish(ctx, &Message{
		Topic:   "test/DirectlyPublishQoS0",
		QoS:     QoS1,
		Payload: []byte("message"),
	}); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	time.Sleep(150 * time.Millisecond)
	expectRetryStats(t, RetryStats{
		TotalRetries:    0,
		TotalTasks:      1,
		CountSetClients: 1,
	}, cli.Stats())

	if err := cli.Publish(ctx, &Message{
		Topic:   "test/DirectlyPublishQoS0",
		QoS:     QoS0,
		Payload: []byte("message"),
	}); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	select {
	case <-publishQoS0Msg:
	case <-time.After(150 * time.Millisecond):
		t.Fatal("Timeout")
	}
	expectRetryStats(t, RetryStats{
		TotalRetries:    0,
		TotalTasks:      1,
		CountSetClients: 1,
	}, cli.Stats())
}
