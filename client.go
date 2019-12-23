// Copyright 2019 The mqtt-go authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mqtt

import (
	"io"
	"sync"
)

// BaseClient is a low layer MQTT client.
// Zero values with valid underlying Transport is a valid BaseClient.
type BaseClient struct {
	// Transport is an underlying connection. Typically net.Conn.
	Transport io.ReadWriteCloser
	// ConnState is called if the connection state is changed.
	ConnState func(ConnState, error)

	handler    Handler
	sig        *signaller
	mu         sync.RWMutex
	connState  ConnState
	err        error
	connClosed chan struct{}
	muWrite    sync.Mutex
	idLast     uint32
}

// Handle registers the message handler.
func (c *BaseClient) Handle(handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
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
