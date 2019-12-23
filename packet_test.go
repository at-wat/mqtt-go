package mqtt

import (
	"bytes"
	"testing"
)

func TestRemainingLength(t *testing.T) {
	cases := []struct {
		n int
		b []byte
	}{
		{64, []byte{0x40}},
		{321, []byte{0xC1, 0x02}},
		{268435455, []byte{0xFF, 0xFF, 0xFF, 0x7F}},
	}

	for _, c := range cases {
		r := remainingLength(c.n)
		if !bytes.Equal(c.b, r) {
			t.Errorf("%d must be encoded to: %v, got: %v", c.n, c.b, r)
		}
	}
}
