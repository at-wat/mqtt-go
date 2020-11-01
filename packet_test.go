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
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/at-wat/mqtt-go/internal/errs"
)

func TestRemainingLength(t *testing.T) {
	cases := []struct {
		n int
		b []byte
	}{
		{0, []byte{0x00}},
		{64, []byte{0x40}},
		{127, []byte{0x7F}},
		{128, []byte{0x80, 0x01}},
		{321, []byte{0xC1, 0x02}},
		{16383, []byte{0xFF, 0x7F}},
		{16384, []byte{0x80, 0x80, 0x01}},
		{100000, []byte{0xA0, 0x8D, 0x06}},
		{2097151, []byte{0xFF, 0xFF, 0x7F}},
		{2097152, []byte{0x80, 0x80, 0x80, 0x01}},
		{268435455, []byte{0xFF, 0xFF, 0xFF, 0x7F}},
	}

	for _, c := range cases {
		r := remainingLength(c.n)
		if !bytes.Equal(c.b, r) {
			t.Errorf("%d must be encoded to: %v, got: %v", c.n, c.b, r)
		}
	}
}

func TestPacketType(t *testing.T) {
	if packetConnect.String() != "CONNECT" {
		t.Errorf("Expected packetConnect.String(): CONNECT, got: %s", packetConnect.String())
	}
	if !strings.HasPrefix(packetType(0xFF).String(), "Unknown") {
		t.Errorf("Expected invlidPacketType.String(): Unknown..., got: %s",
			packetType(0xFF).String(),
		)
	}
}

func TestPacketParseError(t *testing.T) {
	cases := map[string]struct {
		flag     byte
		contents []byte
		pkt      interface{}
		err      error
	}{
		"ConnAck_InvalidFlag": {
			0x01, []byte{}, &pktConnAck{}, ErrInvalidPacket,
		},
		"ConnAck_ShortLength": {
			0x00, []byte{}, &pktConnAck{}, ErrInvalidPacketLength,
		},
		"PingResp_InvalidFlag": {
			0x01, []byte{}, &pktPingResp{}, ErrInvalidPacket,
		},
		"PubAck_InvalidFlag": {
			0x01, []byte{}, &pktPubAck{}, ErrInvalidPacket,
		},
		"PubAck_ShortLength": {
			0x00, []byte{}, &pktPubAck{}, ErrInvalidPacketLength,
		},
		"PubComp_InvalidFlag": {
			0x01, []byte{}, &pktPubComp{}, ErrInvalidPacket,
		},
		"PubComp_ShortLength": {
			0x00, []byte{}, &pktPubComp{}, ErrInvalidPacketLength,
		},
		"Publish_InvalidQoS": {
			0x06, []byte{}, &pktPublish{}, ErrInvalidPacket,
		},
		"Publish_ShortLength": {
			0x02, []byte{}, &pktPublish{}, ErrInvalidPacketLength,
		},
		"PubRec_InvalidFlag": {
			0x01, []byte{}, &pktPubRec{}, ErrInvalidPacket,
		},
		"PubRec_ShortLength": {
			0x00, []byte{}, &pktPubRec{}, ErrInvalidPacketLength,
		},
		"PubRel_InvalidFlag": {
			0x01, []byte{}, &pktPubRel{}, ErrInvalidPacket,
		},
		"PubRel_ShortLength": {
			0x02, []byte{}, &pktPubRel{}, ErrInvalidPacketLength,
		},
		"SubAck_InvalidFlag": {
			0x01, []byte{}, &pktSubAck{}, ErrInvalidPacket,
		},
		"UnsubAck_InvalidFlag": {
			0x01, []byte{}, &pktUnsubAck{}, ErrInvalidPacket,
		},
		"UnsubAck_ShortLength": {
			0x00, []byte{}, &pktUnsubAck{}, ErrInvalidPacketLength,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := reflect.ValueOf(c.pkt).MethodByName("Parse")
			if !f.IsValid() {
				t.Fatal("Packet struct doesn't have Parse method.")
			}
			ret := f.Call(
				[]reflect.Value{
					reflect.ValueOf(c.flag), reflect.ValueOf(c.contents),
				},
			)
			if len(ret) != 2 {
				t.Fatal("Invalid number of the return value of the Parse method.")
			}
			err, ok := ret[1].Interface().(error)
			if !ok {
				t.Fatal("Second return value of Parse is not error.")
			}
			if !errs.Is(err, c.err) {
				t.Errorf("Parse result is expected to be: '%v', got: '%v'", c.err, err)
			}
		})
	}
}

func TestPacketSendCancel(t *testing.T) {
	ca, cb := net.Pipe()
	defer ca.Close()
	cli := &BaseClient{Transport: ca}
	cli.init()

	go func() {
		for {
			b := make([]byte, 100)
			if _, err := cb.Read(b); err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("Subscribe", func(t *testing.T) {
		if err := cli.Subscribe(ctx, Subscription{Topic: "test"}); !errors.Is(err, context.Canceled) {
			t.Errorf("Expected error: '%v', got: '%v'", context.Canceled, err)
		}
	})
	t.Run("Unsubscribe", func(t *testing.T) {
		if err := cli.Unsubscribe(ctx, "test"); !errors.Is(err, context.Canceled) {
			t.Errorf("Expected error: '%v', got: '%v'", context.Canceled, err)
		}
	})
	t.Run("PingReq", func(t *testing.T) {
		if err := cli.Ping(ctx); !errors.Is(err, context.Canceled) {
			t.Errorf("Expected error: '%v', got: '%v'", context.Canceled, err)
		}
	})
	t.Run("PublishQoS1", func(t *testing.T) {
		if err := cli.Publish(ctx, &Message{QoS: QoS1}); !errors.Is(err, context.Canceled) {
			t.Errorf("Expected error: '%v', got: '%v'", context.Canceled, err)
		}
	})
	t.Run("PublishQoS2", func(t *testing.T) {
		if err := cli.Publish(ctx, &Message{QoS: QoS1}); !errors.Is(err, context.Canceled) {
			t.Errorf("Expected error: '%v', got: '%v'", context.Canceled, err)
		}
	})
}

func TestPacketSendError(t *testing.T) {
	ca, _ := net.Pipe()
	cli := &BaseClient{Transport: ca}
	cli.init()

	ctx := context.Background()
	ca.Close()

	t.Run("Subscribe", func(t *testing.T) {
		if err := cli.Subscribe(ctx, Subscription{Topic: "test"}); !errs.Is(err, io.ErrClosedPipe) {
			t.Errorf("Expected error: '%v', got: '%v'", io.ErrClosedPipe, err)
		}
	})
	t.Run("Unsubscribe", func(t *testing.T) {
		if err := cli.Unsubscribe(ctx, "test"); !errs.Is(err, io.ErrClosedPipe) {
			t.Errorf("Expected error: '%v', got: '%v'", io.ErrClosedPipe, err)
		}
	})
	t.Run("PingReq", func(t *testing.T) {
		if err := cli.Ping(ctx); !errs.Is(err, io.ErrClosedPipe) {
			t.Errorf("Expected error: '%v', got: '%v'", io.ErrClosedPipe, err)
		}
	})
	t.Run("Publish", func(t *testing.T) {
		if err := cli.Publish(ctx, &Message{}); !errs.Is(err, io.ErrClosedPipe) {
			t.Errorf("Expected error: '%v', got: '%v'", io.ErrClosedPipe, err)
		}
	})
}

func TestConnectionError(t *testing.T) {
	resps := [][]byte{
		{0x20, 0x02, 0x00, 0x00},
		{0x90, 0x03, 0x00, 0x01, 0x00},
		{0xB0, 0x02, 0x00, 0x02},
		{0xD0, 0x00},
		{},
		{0x40, 0x02, 0x00, 0x04},
		{0x50, 0x02, 0x00, 0x05},
		{0x70, 0x02, 0x00, 0x05},
		{},
	}
	reqs := []func(ctx context.Context, cli *BaseClient) error{
		func(ctx context.Context, cli *BaseClient) error {
			_, err := cli.Connect(ctx, "cli")
			cli.idLast = 0
			return err
		},
		func(ctx context.Context, cli *BaseClient) error {
			return cli.Subscribe(ctx, Subscription{Topic: "test"})
		},
		func(ctx context.Context, cli *BaseClient) error {
			return cli.Unsubscribe(ctx, "test")
		},
		func(ctx context.Context, cli *BaseClient) error {
			return cli.Ping(ctx)
		},
		func(ctx context.Context, cli *BaseClient) error {
			return cli.Publish(ctx, &Message{QoS: QoS0})
		},
		func(ctx context.Context, cli *BaseClient) error {
			return cli.Publish(ctx, &Message{QoS: QoS1})
		},
		func(ctx context.Context, cli *BaseClient) error {
			return cli.Publish(ctx, &Message{QoS: QoS2})
		},
		func(ctx context.Context, cli *BaseClient) error {
			return cli.Disconnect(ctx)
		},
	}

	cases := []struct {
		closeAt int
		errorAt int
	}{
		{0, 0}, // CONNECT
		{1, 1}, // SUBSCRIBE
		{2, 2}, // UNSUBSCRIBE
		{3, 3}, // PINGREQ
		{4, 4}, // PUBLISH QoS0
		{5, 5}, // PUBLISH QoS1
		{6, 6}, // PUBLISH QoS2 PUBREC
		{7, 6}, // PUBLISH QoS2 PUBCOMP
		{8, 7}, // DISCONNECT
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("CloseAt%dErrorAt%d", c.closeAt, c.errorAt), func(t *testing.T) {
			ca, cb := net.Pipe()
			cli := &BaseClient{Transport: ca}

			go func() {
				defer cb.Close()
				for i, resp := range resps {
					time.Sleep(10 * time.Millisecond)
					if i == c.closeAt {
						return
					}
					if _, _, _, err := readPacket(cb); err != nil {
						t.Errorf("Unexpected error: %v", err)
						return
					}
					if len(resp) > 0 {
						io.Copy(cb, bytes.NewReader(resp))
					}
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			for i, req := range reqs {
				err := req(ctx, cli)
				if i == c.errorAt {
					if !errs.Is(err, io.ErrClosedPipe) {
						t.Errorf("Expected error: '%v', got: '%v'", io.ErrClosedPipe, err)
					}
					break
				} else {
					if err != nil {
						t.Errorf("Unexpected error at %d: %v", i, err)
					}
				}
			}
		})
	}
}
