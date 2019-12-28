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
	"strings"
	"testing"
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
