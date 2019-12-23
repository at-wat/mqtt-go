package mqtt

import (
	"errors"
	"fmt"
)

// ErrInvalidRune means that the string has a rune not allowed in MQTT.
var ErrInvalidRune = errors.New("invalid rune in UTF-8 string")

// ErrInvalidPacket means that an invalid message is arrived from the broker.
var ErrInvalidPacket = errors.New("invalid packet")

// ErrInvalidPacketLength means that an invalid length of the message is arrived.
var ErrInvalidPacketLength = errors.New("invalid packet length")

type packetType byte

const (
	packetConnect     packetType = 0x10
	packetConnAck     packetType = 0x20
	packetPublish     packetType = 0x30
	packetPubAck      packetType = 0x40
	packetPubRec      packetType = 0x50
	packetPubRel      packetType = 0x60
	packetPubComp     packetType = 0x70
	packetSubscribe   packetType = 0x80
	packetSubAck      packetType = 0x90
	packetUnsubscribe packetType = 0xA0
	packetUnsubAck    packetType = 0xB0
	packetPingReq     packetType = 0xC0
	packetPingResp    packetType = 0xD0
	packetDisconnect  packetType = 0xE0
	packetFromClient  packetType = 0x02
)

func (t packetType) b() byte {
	return byte(t)
}

func (t packetType) String() string {
	switch t {
	case packetConnect:
		return "CONNECT"
	case packetConnAck:
		return "CONNACK"
	case packetPublish:
		return "PUBLISH"
	case packetPubAck:
		return "PUBACK"
	case packetPubRec:
		return "PUBREC"
	case packetPubRel:
		return "PUBREL"
	case packetPubComp:
		return "PUBCOMP"
	case packetSubscribe:
		return "SUBSCRIBE"
	case packetSubAck:
		return "SUBACK"
	case packetUnsubscribe:
		return "UNSUBSCRIBE"
	case packetUnsubAck:
		return "UNSUBACK"
	case packetPingReq:
		return "PINGREQ"
	case packetPingResp:
		return "PINGRESP"
	case packetDisconnect:
		return "DISCONNECT"
	}
	return fmt.Sprintf("Unknown packet type %x", int(t))
}

func pack(packetType byte, contents ...[]byte) []byte {
	pkt := []byte{packetType}
	var n int
	for _, c := range contents {
		n += len(c)
	}
	pkt = append(pkt, remainingLength(n)...)
	for _, c := range contents {
		pkt = append(pkt, c...)
	}
	return pkt
}

func remainingLength(n int) []byte {
	switch {
	case n <= 0x7F:
		return []byte{byte(n)}
	case n <= 0x7FFF:
		return []byte{
			byte(n) | 0x80,
			byte(n>>7) & 0x7F,
		}
	case n <= 0x7FFFFF:
		return []byte{
			byte(n) | 0x80,
			byte(n>>7) | 0x80,
			byte(n>>14) & 0x7F,
		}
	case n <= 0x7FFFFFFF:
		return []byte{
			byte(n) | 0x80,
			byte(n>>7) | 0x80,
			byte(n>>14) | 0x80,
			byte(n>>21) & 0x7F,
		}
	}
	panic("remaining length overflow")
}

func packString(s string) []byte {
	return packBytes([]byte(s))
}

func packBytes(s []byte) []byte {
	n := len(s)
	if n > 0xFFFF {
		panic("string length overflow")
	}
	ret := packUint16(uint16(n))
	ret = append(ret, s...)
	return ret
}

func packUint16(v uint16) []byte {
	return []byte{
		byte(v >> 8),
		byte(v),
	}
}

func unpackUint16(b []byte) (int, uint16) {
	return 2, uint16(b[0])<<8 | uint16(b[1])
}

func unpackString(b []byte) (int, string, error) {
	nHeader, n := unpackUint16(b)
	if int(n)+nHeader > len(b) {
		return 0, "", ErrInvalidPacketLength
	}

	// Validate UTF-8 runes according to MQTT-1.5.3-1 and MQTT-1.5.3-2.
	rs := []rune(string(b[nHeader : int(n)+nHeader]))
	for _, r := range rs {
		if r == 0x0000 || (0xD800 <= r && r <= 0xDFFF) {
			return 0, "", ErrInvalidRune
		}
	}
	return int(n) + nHeader, string(rs), nil
}
