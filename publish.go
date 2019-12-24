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

type publishFlag byte

const (
	publishFlagRetain  publishFlag = 0x01
	publishFlagQoS0    publishFlag = 0x00
	publishFlagQoS1    publishFlag = 0x02
	publishFlagQoS2    publishFlag = 0x04
	publishFlagQoSMask publishFlag = 0x06
	publishFlagDup     publishFlag = 0x08
)

type pktPublish struct {
	Message *Message
}

func (p *pktPublish) parse(flag byte, contents []byte) (*pktPublish, error) {
	p.Message = &Message{
		Dup:    (publishFlag(flag) & publishFlagDup) != 0,
		Retain: (publishFlag(flag) & publishFlagRetain) != 0,
	}
	switch publishFlag(flag) & publishFlagQoSMask {
	case publishFlagQoS0:
		p.Message.QoS = QoS0
	case publishFlagQoS1:
		p.Message.QoS = QoS1
	case publishFlagQoS2:
		p.Message.QoS = QoS2
	default:
		return nil, ErrInvalidPacket
	}

	var n, nID int
	var err error
	n, p.Message.Topic, err = unpackString(contents)
	if err != nil {
		return nil, err
	}
	if p.Message.QoS != QoS0 {
		if len(contents)-n < 2 {
			return nil, ErrInvalidPacketLength
		}
		nID, p.Message.ID = unpackUint16(contents[n:])
	}
	p.Message.Payload = contents[n+nID:]

	return p, nil
}

func (p *pktPublish) pack() []byte {
	pktHeader := packetPublish.b()

	if p.Message.Retain {
		pktHeader |= byte(publishFlagRetain)
	}
	switch p.Message.QoS {
	case QoS0:
		pktHeader |= byte(publishFlagQoS0)
	case QoS1:
		pktHeader |= byte(publishFlagQoS1)
	case QoS2:
		pktHeader |= byte(publishFlagQoS2)
	default:
		panic("invalid QoS")
	}
	if p.Message.Dup {
		pktHeader |= byte(publishFlagDup)
	}

	header := packString(p.Message.Topic)
	if p.Message.QoS != QoS0 {
		header = append(header, packUint16(p.Message.ID)...)
	}

	return pack(
		pktHeader,
		header,
		p.Message.Payload,
	)
}

// Publish a message to the broker.
// ID field of the message is filled if zero.
func (c *BaseClient) Publish(ctx context.Context, message *Message) error {
	if message.ID == 0 {
		message.ID = c.newID()
	}

	var chPubAck chan *pktPubAck
	var chPubRec chan *pktPubRec
	var chPubComp chan *pktPubComp
	switch message.QoS {
	case QoS1:
		chPubAck = make(chan *pktPubAck, 1)
		c.sig.mu.Lock()
		if c.sig.chPubAck == nil {
			c.sig.chPubAck = make(map[uint16]chan *pktPubAck)
		}
		c.sig.chPubAck[message.ID] = chPubAck
		c.sig.mu.Unlock()
	case QoS2:
		chPubRec = make(chan *pktPubRec, 1)
		chPubComp = make(chan *pktPubComp, 1)
		c.sig.mu.Lock()
		if c.sig.chPubRec == nil {
			c.sig.chPubRec = make(map[uint16]chan *pktPubRec)
		}
		c.sig.chPubRec[message.ID] = chPubRec
		if c.sig.chPubComp == nil {
			c.sig.chPubComp = make(map[uint16]chan *pktPubComp)
		}
		c.sig.chPubComp[message.ID] = chPubComp
		c.sig.mu.Unlock()
	}

	pkt := (&pktPublish{Message: message}).pack()
	if err := c.write(pkt); err != nil {
		return err
	}
	switch message.QoS {
	case QoS1:
		select {
		case <-c.connClosed:
			return ErrClosedTransport
		case <-ctx.Done():
			return ctx.Err()
		case <-chPubAck:
		}
	case QoS2:
		select {
		case <-c.connClosed:
			return ErrClosedTransport
		case <-ctx.Done():
			return ctx.Err()
		case <-chPubRec:
		}
		pktPubRel := (&pktPubRel{ID: message.ID}).pack()
		if err := c.write(pktPubRel); err != nil {
			return err
		}
		select {
		case <-c.connClosed:
			return ErrClosedTransport
		case <-ctx.Done():
			return ctx.Err()
		case <-chPubComp:
		}
	}
	return nil
}
