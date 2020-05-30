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
	"context"
)

type pktUnsubscribe struct {
	ID     uint16
	Topics []string
}

func (p *pktUnsubscribe) Pack() []byte {
	payload := make([]byte, 0, packetBufferCap)
	for _, sub := range p.Topics {
		payload = appendString(payload, sub)
	}

	return pack(
		byte(packetUnsubscribe|packetFromClient),
		packUint16(p.ID),
		payload,
	)
}

// Unsubscribe topics.
func (c *BaseClient) Unsubscribe(ctx context.Context, subs ...string) error {
	c.muConnecting.RLock()
	defer c.muConnecting.RUnlock()

	id := c.newID()

	chUnsubAck := make(chan *pktUnsubAck, 1)
	c.sig.mu.Lock()
	if c.sig.chUnsubAck == nil {
		c.sig.chUnsubAck = make(map[uint16]chan *pktUnsubAck, 1)
	}
	c.sig.chUnsubAck[id] = chUnsubAck
	c.sig.mu.Unlock()

	pkt := (&pktUnsubscribe{ID: id, Topics: subs}).Pack()
	if err := c.write(pkt); err != nil {
		return wrapError(err, "sending UNSUBSCRIBE")
	}
	select {
	case <-c.connClosed:
		return ErrClosedTransport
	case <-ctx.Done():
		return ctx.Err()
	case <-chUnsubAck:
	}
	return nil
}
