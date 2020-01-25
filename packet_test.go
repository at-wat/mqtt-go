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
	"io"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/at-wat/mqtt-go/internal/errs"
)

func TestRemainingLength(t *testing.T) {
	cases := []struct {
		n int
		b []byte
	}{
		{64, []byte{0x40}},
		{321, []byte{0xC1, 0x02}},
		{100000, []byte{0xA0, 0x8D, 0x06}},
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
		if err := cli.Subscribe(ctx, Subscription{Topic: "test"}); err != context.Canceled {
			t.Errorf("Expected error: '%v', got: '%v'", context.Canceled, err)
		}
	})
	t.Run("Unsubscribe", func(t *testing.T) {
		if err := cli.Unsubscribe(ctx, "test"); err != context.Canceled {
			t.Errorf("Expected error: '%v', got: '%v'", context.Canceled, err)
		}
	})
	t.Run("PingReq", func(t *testing.T) {
		if err := cli.Ping(ctx); err != context.Canceled {
			t.Errorf("Expected error: '%v', got: '%v'", context.Canceled, err)
		}
	})
	t.Run("PublishQoS1", func(t *testing.T) {
		if err := cli.Publish(ctx, &Message{QoS: QoS1}); err != context.Canceled {
			t.Errorf("Expected error: '%v', got: '%v'", context.Canceled, err)
		}
	})
	t.Run("PublishQoS2", func(t *testing.T) {
		if err := cli.Publish(ctx, &Message{QoS: QoS1}); err != context.Canceled {
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
