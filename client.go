package mqtt

import (
	"io"
	"sync"
	"time"
)

// BaseClient is an MQTT client.
type BaseClient struct {
	Transport   io.ReadWriteCloser
	Handler     Handler
	SendTimeout time.Duration
	RecvTimeout time.Duration
	ConnState   func(ConnState, error)

	sig        *signaller
	mu         sync.RWMutex
	connState  ConnState
	err        error
	connClosed chan struct{}
	muWrite    sync.Mutex
}

func (c *BaseClient) write(b []byte) error {
	l := len(b)
	c.muWrite.Lock()
	defer c.muWrite.Unlock()
	for i := 0; i < l; {
		n, err := c.Transport.Write(b[i : l-i])
		if err != nil {
			return err
		}
		i += n
	}
	return nil
}

type signaller struct {
	chConnAck  chan *pktConnAck
	chPingResp chan *pktPingResp
	chPubAck   map[uint16]chan *pktPubAck
	chPubRec   map[uint16]chan *pktPubRec
	chPubComp  map[uint16]chan *pktPubComp
	chSubAck   map[uint16]chan *pktSubAck
	chUnsubAck map[uint16]chan *pktUnsubAck
	mu         sync.RWMutex
}

func (s *signaller) ConnAck() chan *pktConnAck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.chConnAck
}
func (s *signaller) PingResp() chan *pktPingResp {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.chPingResp
}
func (s *signaller) PubAck(id uint16) (chan *pktPubAck, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.chPubAck == nil {
		return nil, false
	}
	defer delete(s.chPubAck, id)
	ch, ok := s.chPubAck[id]
	return ch, ok
}
func (s *signaller) PubRec(id uint16) (chan *pktPubRec, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.chPubRec == nil {
		return nil, false
	}
	defer delete(s.chPubRec, id)
	ch, ok := s.chPubRec[id]
	return ch, ok
}
func (s *signaller) PubComp(id uint16) (chan *pktPubComp, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.chPubComp == nil {
		return nil, false
	}
	defer delete(s.chPubComp, id)
	ch, ok := s.chPubComp[id]
	return ch, ok
}
func (s *signaller) SubAck(id uint16) (chan *pktSubAck, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.chSubAck == nil {
		return nil, false
	}
	defer delete(s.chSubAck, id)
	ch, ok := s.chSubAck[id]
	return ch, ok
}
func (s *signaller) UnsubAck(id uint16) (chan *pktUnsubAck, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.chUnsubAck == nil {
		return nil, false
	}
	defer delete(s.chUnsubAck, id)
	ch, ok := s.chUnsubAck[id]
	return ch, ok
}
