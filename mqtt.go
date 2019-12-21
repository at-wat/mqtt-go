package mqtt

import (
	"context"
	"fmt"
	"io"
	"time"
)

type QoS uint8

const (
	QoS0 QoS = 0
	QoS1     = 1
	QoS2     = 2
)

type Message struct {
	Topic   string
	ID      uint16
	QoS     QoS
	Retain  bool
	Message []byte
}

type Subscription struct {
	Topic string
	QoS   uint8
}

type ConnectOptions struct {
	UserName     string
	Password     string
	CleanSession bool
	KeepAlive    uint16
	Will         *Message
}

type ConnectOption func(*ConnectOptions) error

type CommandClient interface {
	Connect(ctx context.Context, clientID string, opts ...ConnectOption) (*ConnAck, error)
	Disconnect(ctx context.Context) error
	Publish(ctx context.Context, message *Message) error
	Subscribe(ctx context.Context, subs ...*Subscription) error
	Unsubscribe(ctx context.Context, subs ...string) error
}

type HandlerFunc func(*Message)

func (h HandlerFunc) Serve(message *Message) {
	h(message)
}

type Handler interface {
	Serve(*Message)
}

type ConnState int

const (
	StateNew ConnState = iota
	StateActive
	StateClosed
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

type ConnAck struct {
	SessionPresent bool
	Code           ConnectionReturnCode
}

type Client struct {
	Transport   io.ReadWriteCloser
	Handler     Handler
	SendTimeout time.Duration
	RecvTimeout time.Duration
	ConnState   func(ConnState)

	chConnAck chan *ConnAck
}
