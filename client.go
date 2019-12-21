package mqtt

import (
	"io"
	"sync"
	"time"
)

// Client is an MQTT client.
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
