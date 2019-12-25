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

func TestIntegration_ReconnectClient(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			chReceived := make(chan *Message, 100)
			cli, err := NewReconnectClient(
				ctx,
				&URLDialer{
					URL: url,
					Options: []DialOption{
						WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
					},
				},
				"ReconnectClient"+name,
				WithConnectOption(
					WithKeepAlive(10),
					WithCleanSession(true),
				),
				WithPingInterval(time.Second),
				WithTimeout(time.Second),
				WithReconnectWait(time.Second, 10*time.Second),
			)
			if err != nil {
				t.Fatalf("Unexpected error: '%v'", err)
			}
			cli.Handle(HandlerFunc(func(msg *Message) {
				chReceived <- msg
			}))

			// Close underlying client.
			time.Sleep(time.Millisecond)
			cli.(*RetryClient).Client.(ClientCloser).Close()

			if err := cli.Subscribe(ctx, Subscription{Topic: "test", QoS: QoS1}); err != nil {
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

func newCloseFilter(key byte, en bool) func([]byte) bool {
	var readBuf []byte
	return func(b []byte) (ret bool) {
		readBuf = append(readBuf, b...)
		ret = false
		for {
			if len(readBuf) == 0 {
				return
			}
			if readBuf[0]&0xF0 == key {
				ret = en
			}
			var length int
			for i := 1; i < 6; i++ {
				if i >= len(readBuf) {
					return
				}
				length = (length << 7) | (int(readBuf[i]) & 0x7F)
				if !(readBuf[i]&0x80 != 0) {
					length += i + 1
					break
				}
			}
			if length >= len(readBuf) {
				return
			}
			readBuf = readBuf[length:]
		}
	}
}

func TestIntegration_ReconnectClient_Resubscribe(t *testing.T) {
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			cases := map[string]struct {
				out byte
				in  byte
			}{
				"ConnAck":    {0x00, 0x20},
				"Subscribe":  {0x80, 0x00},
				"PublishOut": {0x30, 0x00},
				"PubAck":     {0x00, 0x40},
				"SubAck":     {0x00, 0x90},
				"PublishIn":  {0x00, 0x30},
			}
			for pktName, head := range cases {
				fIn, fOut := head.in, head.out
				t.Run("StopAt"+pktName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					var dialCnt int32

					chReceived := make(chan *Message, 100)
					cli, err := NewReconnectClient(
						ctx,
						DialerFunc(func() (ClientCloser, error) {
							cnt := atomic.AddInt32(&dialCnt, 1)
							cli, err := Dial(url,
								WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
							)
							if err != nil {
								return nil, err
							}
							ca, cb := filteredpipe.DetectAndClosePipe(
								newCloseFilter(fIn, cnt == 1),
								newCloseFilter(fOut, cnt == 1),
							)
							filteredpipe.Connect(ca, cli.Transport)
							cli.Transport = cb
							return cli, nil
						}),
						"ReconnectClient"+name+pktName,
						WithPingInterval(250*time.Millisecond),
						WithTimeout(100*time.Millisecond),
						WithReconnectWait(100*time.Millisecond, time.Second),
					)
					if err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					cli.Handle(HandlerFunc(func(msg *Message) {
						chReceived <- msg
					}))

					if err := cli.Publish(ctx, &Message{
						Topic:   "test/" + name,
						QoS:     QoS1,
						Retain:  true,
						Payload: []byte("message"),
					}); err != nil {
						t.Fatalf("Unexpected error: '%v'", err)
					}
					if err := cli.Subscribe(ctx, Subscription{
						Topic: "test/" + name,
						QoS:   QoS1,
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

					cnt := atomic.LoadInt32(&dialCnt)
					if cnt != 2 {
						t.Errorf("Must be dialled twice, dialled %d times", cnt)
					}
				})
			}
		})
	}
}
