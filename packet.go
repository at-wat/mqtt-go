package mqtt

import (
	"fmt"
)

type packetType byte

const (
	packetConnect     packetType = 0x10
	packetConnAck                = 0x20
	packetPublish                = 0x30
	packetPubAck                 = 0x40
	packetPubRec                 = 0x50
	packetPubRel                 = 0x60
	packetPubComp                = 0x70
	packetSubscribe              = 0x80
	packetSubAck                 = 0x90
	packetUnsubscribe            = 0xA0
	packetUnsubAck               = 0xB0
	packetPingReq                = 0xC0
	packetPingResp               = 0xD0
	packetDisconnect             = 0xE0
)

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
			byte(n>>7) | 0x80,
			byte(n) & 0x7F,
		}
	case n <= 0x7FFFFF:
		return []byte{
			byte(n>>14) | 0x80,
			byte(n>>7) | 0x80,
			byte(n) & 0x7F,
		}
	case n <= 0x7FFFFFFF:
		return []byte{
			byte(n>>21) | 0x80,
			byte(n>>14) | 0x80,
			byte(n>>7) | 0x80,
			byte(n) & 0x7F,
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
