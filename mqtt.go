package mqtt

import (
	"context"
	"io"
	"sync"
	"time"
)

type QoS uint8

const (
	QoS0             QoS = 0x00
	QoS1             QoS = 0x01
	QoS2             QoS = 0x02
	SubscribeFailure QoS = 0x80
)

type Message struct {
	Topic   string
	ID      uint16
	QoS     QoS
	Retain  bool
	Dup     bool
	Payload []byte
}

type Subscription struct {
	Topic string
	QoS   QoS
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
	Connect(ctx context.Context, clientID string, opts ...ConnectOption) error
	Disconnect(ctx context.Context) error
	Publish(ctx context.Context, message *Message) error
	Subscribe(ctx context.Context, subs ...Subscription) error
	Unsubscribe(ctx context.Context, subs ...string) error
	Ping(ctx context.Context) error
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
	StateDisconnected
)

type Client struct {
	Transport   io.ReadWriteCloser
	Handler     Handler
	SendTimeout time.Duration
	RecvTimeout time.Duration
	ConnState   func(ConnState, error)

	sig        signaller
	mu         sync.RWMutex
	connState  ConnState
	err        error
	connClosed chan struct{}
}

type signaller struct {
	chConnAck  chan *pktConnAck
	chPingResp chan *pktPingResp
	chPubAck   map[uint16]chan *pktPubAck
	chPubRec   map[uint16]chan *pktPubRec
	chPubComp  map[uint16]chan *pktPubComp
	chSubAck   map[uint16]chan *pktSubAck
	chUnsubAck map[uint16]chan *pktUnsubAck
}

func (s signaller) Copy() signaller {
	var ret signaller
	ret.chConnAck = s.chConnAck
	ret.chPingResp = s.chPingResp
	ret.chPubAck = make(map[uint16]chan *pktPubAck)
	ret.chPubRec = make(map[uint16]chan *pktPubRec)
	ret.chPubComp = make(map[uint16]chan *pktPubComp)
	ret.chSubAck = make(map[uint16]chan *pktSubAck)
	ret.chUnsubAck = make(map[uint16]chan *pktUnsubAck)

	for k, v := range s.chPubAck {
		ret.chPubAck[k] = v
	}
	for k, v := range s.chPubRec {
		ret.chPubRec[k] = v
	}
	for k, v := range s.chPubComp {
		ret.chPubComp[k] = v
	}
	for k, v := range s.chSubAck {
		ret.chSubAck[k] = v
	}
	for k, v := range s.chUnsubAck {
		ret.chUnsubAck[k] = v
	}
	return ret
}
