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
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/at-wat/mqtt-go/internal/filteredpipe"
)

func TestIntegration_ReconnectClient(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			chReceived := make(chan *Message, 100)
			cli, err := NewReconnectClient(
				&URLDialer{
					URL: url,
					Options: []DialOption{
						WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
					},
				},
				WithPingInterval(time.Second),
				WithTimeout(time.Second),
				WithReconnectWait(time.Second, 10*time.Second),
			)
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			_, err = cli.Connect(
				ctx,
				"ReconnectClient"+name,
				WithKeepAlive(10),
				WithCleanSession(true),
			)
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			cli.Handle(HandlerFunc(func(msg *Message) {
				chReceived <- msg
			}))

			// Close underlying client.
			time.Sleep(time.Millisecond)
			cli.(*reconnectClient).cli.Close()

			if _, err := cli.Subscribe(ctx, Subscription{Topic: "test", QoS: QoS1}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			if err := cli.Publish(ctx, &Message{
				Topic:   "test",
				QoS:     QoS1,
				Retain:  true,
				Payload: []byte("message"),
			}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			time.Sleep(time.Second)

			select {
			case <-ctx.Done():
				t.Fatalf("Unexpected error: '%v'", ctx.Err())
			case <-chReceived:
				cli.Disconnect(ctx)
			}
		})
	}
}

func newFilterBase(cbMsg func([]byte) bool) func([]byte) bool {
	var readBuf []byte
	return func(b []byte) (ret bool) {
		readBuf = append(readBuf, b...)
		ret = false
		for {
			if len(readBuf) == 0 {
				return
			}
			var length int
			for i := 1; i < 5; i++ {
				if i >= len(readBuf) {
					return
				}
				length = (length << 7) | (int(readBuf[i]) & 0x7F)
				if readBuf[i]&0x80 == 0 {
					length += i + 1
					break
				}
			}
			if length >= len(readBuf) {
				return
			}
			if cbMsg(readBuf[:length]) {
				ret = true
				return
			}
			readBuf = readBuf[length:]
		}
	}
}

func newCloseFilter(key byte, en bool) func([]byte) bool {
	return newFilterBase(func(msg []byte) bool {
		return en && msg[0]&0xF0 == key
	})
}

func TestIntegration_ReconnectClient_Resubscribe(t *testing.T) {
	for name, url := range urls {
		url := url
		t.Run(name, func(t *testing.T) {
			for dropName, dropCnt := range map[string]int32{
				"DropOnce":  1,
				"DropTwice": 2,
			} {
				dropCnt := dropCnt
				t.Run(dropName, func(t *testing.T) {
					cases := map[string]struct {
						out byte
						qos QoS
						in  byte
					}{
						"ConnAck":    {0x00, QoS1, 0x20},
						"Subscribe":  {0x80, QoS1, 0x00},
						"PublishOut": {0x30, QoS1, 0x00},
						"PubAck":     {0x00, QoS1, 0x40},
						"PubRec":     {0x00, QoS2, 0x50},
						"PubRel":     {0x60, QoS2, 0x00},
						"PubComp":    {0x00, QoS2, 0x70},
						"SubAck":     {0x00, QoS1, 0x90},
						"PublishIn":  {0x00, QoS1, 0x30},
					}
					for pktName, head := range cases {
						fIn, qos, fOut := head.in, head.qos, head.out
						t.Run("StopAt"+pktName, func(t *testing.T) {
							ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
							defer cancel()
							var dialCnt int32

							chReceived := make(chan *Message, 100)
							cli, err := NewReconnectClient(
								DialerFunc(func(ctx context.Context) (*BaseClient, error) {
									cli, err := DialContext(ctx, url,
										WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
									)
									if err != nil {
										return nil, err
									}
									cnt := atomic.AddInt32(&dialCnt, 1)
									ca, cb := filteredpipe.DetectAndClosePipe(
										newCloseFilter(fIn, cnt <= dropCnt),
										newCloseFilter(fOut, cnt <= dropCnt),
									)
									filteredpipe.Connect(ca, cli.Transport)
									cli.Transport = cb
									return cli, nil
								}),
								WithPingInterval(250*time.Millisecond),
								WithTimeout(250*time.Millisecond),
								WithReconnectWait(200*time.Millisecond, time.Second),
							)
							if err != nil {
								t.Fatalf("Unexpected error: '%v'", err)
							}
							_, err = cli.Connect(
								ctx,
								"ReconnectClient"+name+pktName,
							)
							if err != nil {
								t.Fatalf("Unexpected error: '%v'", err)
							}
							cli.Handle(HandlerFunc(func(msg *Message) {
								chReceived <- msg
							}))

							if err := cli.Publish(ctx, &Message{
								Topic:   "test/" + name + pktName,
								QoS:     qos,
								Retain:  true,
								Payload: []byte("message"),
							}); err != nil {
								t.Fatalf("Unexpected error: '%v'", err)
							}
							if _, err := cli.Subscribe(ctx, Subscription{
								Topic: "test/" + name + pktName,
								QoS:   qos,
							}); err != nil {
								t.Fatalf("Unexpected error: '%v'", err)
							}

							for {
								time.Sleep(50 * time.Millisecond)
								if cnt := atomic.LoadInt32(&dialCnt); cnt >= 2 {
									break
								}
							}

							select {
							case <-ctx.Done():
								t.Fatalf("Unexpected error: '%v'", ctx.Err())
							case <-chReceived:
							}
							cli.Disconnect(ctx)

							if cnt := atomic.LoadInt32(&dialCnt); cnt < 2 {
								t.Errorf("Must be dialed at least twice, dialed %d times", cnt)
							}
						})
					}
				})
			}
		})
	}
}

func TestIntegration_ReconnectClient_SessionPersistence(t *testing.T) {
	for name, url := range urls {
		url := url
		t.Run(name, func(t *testing.T) {
			for resubName, alwaysResub := range map[string]bool{
				"Always":       true,
				"IfNotPresent": false,
			} {
				alwaysResub := alwaysResub
				resubName := resubName
				t.Run(resubName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					var subCnt int32
					var dialCnt int32
					var actualConn io.ReadWriteCloser

					cli, err := NewReconnectClient(
						DialerFunc(func(ctx context.Context) (*BaseClient, error) {
							cli, err := DialContext(ctx, url,
								WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
							)
							if err != nil {
								return nil, err
							}
							atomic.AddInt32(&dialCnt, 1)
							ca, cb := filteredpipe.DetectAndClosePipe(
								newFilterBase(func([]byte) bool { return false }),
								newFilterBase(func(msg []byte) bool {
									if msg[0]&0xF0 == 0x80 {
										atomic.AddInt32(&subCnt, 1)
									}
									return false
								}),
							)
							filteredpipe.Connect(ca, cli.Transport)
							actualConn = cli.Transport
							cli.Transport = cb
							return cli, nil
						}),
						WithPingInterval(250*time.Millisecond),
						WithTimeout(250*time.Millisecond),
						WithReconnectWait(200*time.Millisecond, time.Second),
						WithAlwaysResubscribe(alwaysResub),
					)
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					chReceived := make(chan *Message, 100)
					cli.Handle(HandlerFunc(func(msg *Message) {
						chReceived <- msg
					}))
					_, err = cli.Connect(
						ctx,
						fmt.Sprintf("ReconnectClientSession%s-%d", name, time.Now().UnixNano()),
					)
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					topic := "test_session/" + name
					if _, err := cli.Subscribe(ctx, Subscription{
						Topic: topic,
						QoS:   QoS2,
					}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					if err := cli.Publish(ctx, &Message{
						Topic:  topic,
						QoS:    QoS2,
						Retain: true,
					}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					for {
						time.Sleep(50 * time.Millisecond)
						if cnt := atomic.LoadInt32(&dialCnt); cnt >= 1 {
							break
						}
					}
					select {
					case <-chReceived:
					case <-ctx.Done():
						t.Fatal("Timeout")
					}

					actualConn.Close()

					for {
						time.Sleep(50 * time.Millisecond)
						if cnt := atomic.LoadInt32(&dialCnt); cnt >= 2 {
							break
						}
					}

					if err := cli.Publish(ctx, &Message{
						Topic:  topic,
						QoS:    QoS2,
						Retain: true,
					}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					select {
					case <-chReceived:
					case <-ctx.Done():
						t.Fatal("Timeout")
					}

					cli.Disconnect(ctx)

					if cnt := atomic.LoadInt32(&dialCnt); cnt < 2 {
						t.Errorf("Must be dialed at least twice, dialed %d times", cnt)
					}
					if alwaysResub {
						if cnt := atomic.LoadInt32(&subCnt); cnt != 2 {
							t.Errorf("Must be subscribed twice, subscribed %d times", cnt)
						}
					} else {
						if cnt := atomic.LoadInt32(&subCnt); cnt != 1 {
							t.Errorf("Must be subscribed once, subscribed %d times", cnt)
						}
					}
				})
			}
		})
	}
}

func newOnOffFilter(sw *int32) func([]byte) bool {
	return func(b []byte) bool {
		s := atomic.LoadInt32(sw)
		return s != 0
	}
}

func TestIntegration_ReconnectClient_RetryPublish(t *testing.T) {
	for _, qos := range []QoS{QoS1, QoS2} {
		qos := qos
		qosStr := fmt.Sprintf("QoS%d", qos)
		t.Run(qosStr, func(t *testing.T) {
			for name, url := range urls {
				t.Run(name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()

					cliRecv, err := DialContext(
						ctx, url,
						WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
					)
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					if _, err = cliRecv.Connect(ctx,
						"RetryRecvClientPub"+qosStr+name,
					); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					topic := fmt.Sprintf("test/Retry_%s_%d", name, qos)

					if _, err := cliRecv.Subscribe(ctx, Subscription{
						Topic: topic,
						QoS:   qos,
					}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					var receivedCnt byte
					var mu sync.Mutex
					cliRecv.Handle(HandlerFunc(func(msg *Message) {
						mu.Lock()
						defer mu.Unlock()
						if msg.Payload[0] > receivedCnt {
							// Ignore retained messages.
							return
						}
						if qos == QoS1 && msg.Payload[0] != receivedCnt {
							// Allow duplication of QoS1 message.
							t.Log("QoS1 message duplication is ignored.")
							return
						}
						if msg.Payload[0] != receivedCnt {
							t.Errorf("%d-th message is expected to be %d, got %d", receivedCnt, receivedCnt, msg.Payload[0])
						}
						receivedCnt++
					}))

					var sw int32
					chConnected := make(chan struct{}, 1)

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
							cli.ConnState = func(s ConnState, err error) {
								if s == StateActive {
									chConnected <- struct{}{}
								}
							}
							return cli, nil
						}),
						WithPingInterval(250*time.Millisecond),
						WithTimeout(250*time.Millisecond),
						WithReconnectWait(200*time.Millisecond, time.Second),
					)
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					cli.Connect(ctx, "RetryClientPub"+name)

					select {
					case <-ctx.Done():
						t.Fatalf("Unexpected error: '%v'", ctx.Err())
					case <-chConnected:
					}

					for i := 0; i < 5; i++ {
						if err := cli.Publish(ctx, &Message{
							Topic:   topic,
							QoS:     qos,
							Retain:  true,
							Payload: []byte{byte(i)},
						}); err != nil {
							t.Fatalf("Unexpected error: '%v'", err)
						}
						time.Sleep(10 * time.Millisecond)
					}
					// Disconnect
					atomic.StoreInt32(&sw, 1)

					for i := 5; i < 10; i++ {
						if err := cli.Publish(ctx, &Message{
							Topic:   topic,
							QoS:     qos,
							Retain:  true,
							Payload: []byte{byte(i)},
						}); err != nil {
							t.Fatalf("Unexpected error: '%v'", err)
						}
						time.Sleep(10 * time.Millisecond)
					}
					// Connect
					atomic.StoreInt32(&sw, 0)
					select {
					case <-ctx.Done():
						t.Fatalf("Unexpected error: '%v'", ctx.Err())
					case <-chConnected:
					}
					for {
						time.Sleep(50 * time.Millisecond)
						mu.Lock()
						n := receivedCnt
						mu.Unlock()
						if n >= 10 {
							break
						}
					}

					cli.Disconnect(ctx)
					cliRecv.Disconnect(ctx)

					mu.Lock()
					defer mu.Unlock()
					switch qos {
					case QoS1:
						if receivedCnt < 10 {
							t.Errorf("Expected number of the messages: >=10, got: %d", receivedCnt)
						}
					case QoS2:
						if receivedCnt != 10 {
							t.Errorf("Expected number of the messages: 10, got: %d", receivedCnt)
						}
					}
				})
			}
		})
	}
}

func TestIntegration_ReconnectClient_RetrySubscribe(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			cliSend, err := DialContext(
				ctx, url,
				WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
			)
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			if _, err = cliSend.Connect(ctx,
				"RetrySendClientSub"+name,
			); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}

			var sw int32
			chConnected := make(chan struct{}, 1)

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
					cli.ConnState = func(s ConnState, err error) {
						if s == StateActive {
							chConnected <- struct{}{}
						}
					}
					return cli, nil
				}),
				WithPingInterval(250*time.Millisecond),
				WithTimeout(250*time.Millisecond),
				WithReconnectWait(200*time.Millisecond, time.Second),
			)
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			var received bool
			var mu sync.Mutex
			cli.Handle(HandlerFunc(func(msg *Message) {
				mu.Lock()
				defer mu.Unlock()
				if msg.Payload[0] != 0 {
					t.Errorf("Message byte is expected to be 0, got %d", msg.Payload[0])
				} else {
					received = true
				}
			}))

			cli.Connect(ctx, "RetryClientSub"+name)

			select {
			case <-ctx.Done():
				t.Fatalf("Unexpected error: '%v'", ctx.Err())
			case <-chConnected:
			}

			topic := "test/RetrySub" + name

			// Disconnect
			atomic.StoreInt32(&sw, 1)
			// Try subscribe
			cli.Subscribe(ctx, Subscription{Topic: topic, QoS: QoS1})
			time.Sleep(100 * time.Millisecond)
			// Connect
			atomic.StoreInt32(&sw, 0)
			select {
			case <-ctx.Done():
				t.Fatalf("Unexpected error: '%v'", ctx.Err())
			case <-chConnected:
			}

			time.Sleep(50 * time.Millisecond)
			if err := cliSend.Publish(ctx, &Message{
				Topic:   topic,
				QoS:     QoS0,
				Retain:  false,
				Payload: []byte{0},
			}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			time.Sleep(50 * time.Millisecond)

			// Disconnect
			atomic.StoreInt32(&sw, 1)
			// Try unsubscribe
			cli.Unsubscribe(ctx, topic)
			time.Sleep(50 * time.Millisecond)
			// Connect
			atomic.StoreInt32(&sw, 0)
			select {
			case <-ctx.Done():
				t.Fatalf("Unexpected error: '%v'", ctx.Err())
			case <-chConnected:
			}

			time.Sleep(50 * time.Millisecond)
			if err := cliSend.Publish(ctx, &Message{
				Topic:   topic,
				QoS:     QoS0,
				Retain:  false,
				Payload: []byte{1},
			}); err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			time.Sleep(50 * time.Millisecond)

			cli.Disconnect(ctx)
			cliSend.Disconnect(ctx)

			mu.Lock()
			defer mu.Unlock()
			if !received {
				t.Error("Expected to receive one message, but not received")
			}
		})
	}
}

func TestIntegration_ReconnectClient_Ping(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			chConnected := make(chan struct{}, 1)
			cli, err := NewReconnectClient(
				&URLDialer{
					URL: url,
					Options: []DialOption{
						WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
						WithConnStateHandler(func(s ConnState, err error) {
							if s == StateActive {
								chConnected <- struct{}{}
							}
						}),
					},
				},
				WithPingInterval(250*time.Millisecond),
				WithTimeout(250*time.Millisecond),
				WithReconnectWait(200*time.Millisecond, time.Second),
			)
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			cli.Connect(ctx, "RetryClientPing"+name)

			select {
			case <-ctx.Done():
				t.Fatalf("Unexpected error: '%v'", ctx.Err())
			case <-chConnected:
			}

			if err := cli.Ping(ctx); err != nil {
				t.Errorf("Unexpected error: '%v'", err)
			}
			cli.Disconnect(ctx)
		})
	}
}

func TestIntegration_ReconnectClient_Context(t *testing.T) {
	t.Run("CancelAfterConnect", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cli, err := NewReconnectClient(
			&URLDialer{
				URL: urls["MQTT"],
				Options: []DialOption{
					WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
				},
			},
			WithPingInterval(250*time.Millisecond),
			WithTimeout(250*time.Millisecond),
			WithReconnectWait(200*time.Millisecond, time.Second),
		)
		if err != nil {
			t.Fatalf("Unexpected error: '%v'", err)
		}
		if _, err := cli.Connect(ctx, "RetryClientContext1"); err != nil {
			t.Fatalf("Unexpected error: '%v'", err)
		}

		cancel() // Once connected, connection must be kept

		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()

		if err := cli.Ping(ctx2); err != nil {
			t.Errorf("Unexpected error: '%v'", err)
		}
		cli.Disconnect(ctx2)
	})
	t.Run("CancelBeforeConnect", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		cli, err := NewReconnectClient(
			&URLDialer{URL: "mqtt://localhost:65535"},
		)
		if err != nil {
			t.Fatalf("Unexpected error: '%v'", err)
		}
		if _, err := cli.Connect(ctx, "RetryClientContext2"); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Rxpected error: '%v', got: '%v'", context.DeadlineExceeded, err)
		}
	})
}

func TestIntegration_ReconnectClient_KeepAliveError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chErr := make(chan error)

	cli, err := NewReconnectClient(
		DialerFunc(func(ctx context.Context) (*BaseClient, error) {
			cli, err := DialContext(ctx, urls["MQTT"],
				WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
			)
			if err != nil {
				return nil, err
			}
			ca, cb := filteredpipe.DetectAndDropPipe(
				newCloseFilter(byte(packetPingResp), true),
				func([]byte) bool { return false },
			)
			filteredpipe.Connect(ca, cli.Transport)
			cli.Transport = cb
			cli.ConnState = func(s ConnState, err error) {
				if err != nil {
					chErr <- err
				}
			}
			return cli, nil
		}),
		WithPingInterval(100*time.Millisecond),
		WithTimeout(100*time.Millisecond),
		WithReconnectWait(100*time.Millisecond, 500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	if _, err := cli.Connect(ctx, "RetryClientKeepAliveError", WithKeepAlive(60)); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	select {
	case err := <-chErr:
		if !errors.Is(err, ErrPingTimeout) {
			t.Errorf("Expected error '%v', got '%v'", ErrPingTimeout, err)
		}
	case <-ctx.Done():
		t.Fatal("Timeout")
	}

	cli.Disconnect(ctx)
}

func TestIntegration_ReconnectClient_RepeatedDisconnect(t *testing.T) {
	const testCount = 128
	for _, qos := range []QoS{QoS1, QoS2} {
		qos := qos
		qosStr := fmt.Sprintf("QoS%d", qos)
		t.Run(qosStr, func(t *testing.T) {
			for name, url := range urls {
				url := url
				t.Run(name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					cliRaw, err := DialContext(
						ctx, url,
						WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
					)
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					if _, err = cliRaw.Connect(ctx,
						"ReconnectClient2Raw"+name+qosStr,
						WithCleanSession(true),
					); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					cli, err := NewReconnectClient(
						&URLDialer{
							URL: url,
							Options: []DialOption{
								WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
							},
						},
						WithPingInterval(time.Second),
						WithTimeout(time.Second),
						WithReconnectWait(10*time.Millisecond, 10*time.Millisecond),
					)
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					_, err = cli.Connect(
						ctx,
						"ReconnectClient2"+name+qosStr,
						WithKeepAlive(10),
						WithCleanSession(true),
					)
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					topic := fmt.Sprintf("test_%d_%s", qos, name)

					var mu sync.Mutex
					received := make(map[byte]int)
					cliRaw.Handle(HandlerFunc(func(msg *Message) {
						mu.Lock()
						defer mu.Unlock()
						received[msg.Payload[0]]++
					}))
					if _, err := cliRaw.Subscribe(ctx, Subscription{Topic: topic, QoS: qos}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}

					go func() {
						cli := cli.(*reconnectClient)
						for {
							cli.mu.Lock()
							c := cli.cli
							cli.mu.Unlock()

							select {
							case <-time.After(100 * time.Millisecond):
							case <-ctx.Done():
								return
							}
							// Close underlying client.
							c.Close()
						}
					}()

					for i := 0; i < testCount; i++ {
						if err := cli.Publish(ctx, &Message{
							Topic:   topic,
							QoS:     qos,
							Payload: []byte{byte(i)},
						}); err != nil {
							t.Fatalf("Unexpected error: '%v'", err)
						}
						time.Sleep(10 * time.Millisecond)
					}

					time.Sleep(500 * time.Millisecond)

					mu.Lock()
					defer mu.Unlock()
					for i := 0; i < testCount; i++ {
						switch qos {
						case QoS1:
							if received[byte(i)] < 1 {
								t.Errorf("Expected number of received packets #%d: >=%d, got: %d", i, 1, received[byte(i)])
							}
						case QoS2:
							if received[byte(i)] != 1 {
								t.Errorf("Expected number of received packets #%d: %d, got: %d", i, 1, received[byte(i)])
							}
						}
					}
					cli.Disconnect(ctx)
					cliRaw.Disconnect(ctx)
				})
			}
		})
	}
}
