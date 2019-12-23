package mqtt

import (
	"testing"
)

func TestPublish_ParseError(t *testing.T) {
	cases := []struct {
		flag     byte
		contents []byte
		err      error
	}{
		{0x00, []byte{0x00, 0x01, 0x61, 0x00, 0x00}, nil},
		{0x00, []byte{0x00, 0x01, 0x00, 0x00, 0x00}, ErrInvalidRune},
		{0x06, []byte{0x00, 0x01, 0x61, 0x00, 0x00}, ErrInvalidPacket},
		{0x00, []byte{0x00, 0x01}, ErrInvalidPacketLength},
		{0x00, []byte{0x00}, ErrInvalidPacketLength},
	}

	for _, c := range cases {
		_, err := (&pktPublish{}).parse(c.flag, c.contents)
		if err != c.err {
			t.Errorf("Parsing packet with flag=%x, contents=%v expected error: %v, got: %v",
				c.flag, c.contents,
				c.err, err,
			)
		}
	}
}
