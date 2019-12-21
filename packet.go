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

type protocolLevel byte

const (
	protocol311 protocolLevel = 0x04
)

type connectFlag byte

const (
	flagCleanSession connectFlag = 0x02
	flagWill                     = 0x04
	flagWillQoS0                 = 0x00
	flagWillQoS1                 = 0x08
	flagWillQoS2                 = 0x10
	flagWillRetain               = 0x20
	flagPassword                 = 0x40
	flagUserName                 = 0x80
)

const flagWillQoSBit = 3

func pack(packetType byte, header []byte, payload []byte) []byte {
	pkt := []byte{packetType}
	nHeader := len(header)
	if nHeader > 0xFF {
		panic("fixed header length overflow")
	}
	pkt = append(pkt, remainingLength(len(header)+len(payload))...)
	pkt = append(pkt, header...)
	pkt = append(pkt, payload...)
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
	n := len(s)
	if n > 0xFFFF {
		panic("string length overflow")
	}
	ret := []byte{byte(n >> 8), byte(n)}
	ret = append(ret, []byte(s)...)
	return ret
}

func (p *ConnAck) parse(flag byte, contents []byte) *ConnAck {
	return &ConnAck{
		SessionPresent: (contents[0]&0x01 != 0),
		Code:           ConnectionReturnCode(contents[1]),
	}
}
