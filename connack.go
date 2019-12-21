package mqtt

import (
	"fmt"
)

type ConnectionReturnCode byte

const (
	ConnectionAccepted          ConnectionReturnCode = 0
	UnacceptableProtocolVersion                      = 1
	IdentifierRejected                               = 2
	ServerUnavailable                                = 3
	BadUserNameOrPassword                            = 4
	NotAuthorized                                    = 5
)

func (c ConnectionReturnCode) String() string {
	switch c {
	case ConnectionAccepted:
		return "ConnectionAccepted"
	case UnacceptableProtocolVersion:
		return "Connection Refused, unacceptable protocol version"
	case IdentifierRejected:
		return "Connection Refused, identifier rejected"
	case ServerUnavailable:
		return "Connection Refused, Server unavailable"
	case BadUserNameOrPassword:
		return "Connection Refused, bad user name or password"
	case NotAuthorized:
		return "Connection Refused, not authorized"
	}
	return fmt.Sprintf("Unknown ConnectionReturnCode %x", int(c))
}

type pktConnAck struct {
	SessionPresent bool
	Code           ConnectionReturnCode
}

func (p *pktConnAck) parse(flag byte, contents []byte) *pktConnAck {
	return &pktConnAck{
		SessionPresent: (contents[0]&0x01 != 0),
		Code:           ConnectionReturnCode(contents[1]),
	}
}
