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
	"errors"
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

// ErrPayloadLenExceeded means the payload length is too large.
var ErrPayloadLenExceeded = errors.New("payload length exceeded")

// ErrInvalidQoS means the QoS value is not allowed.
var ErrInvalidQoS = errors.New("invalid QoS")

type pktPublish struct {
	Message *Message
}

func (p *pktPublish) Parse(flag byte, contents []byte) (*pktPublish, error) {
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
		return nil, wrapErrorf(
			ErrInvalidPacket, "parsing PUBLISH: QoS %d", int(publishFlag(flag)&publishFlagQoSMask),
		)
	}

	var n, nID int
	var err error
	n, p.Message.Topic, err = unpackString(contents)
	if err != nil {
		return nil, err
	}
	if p.Message.QoS != QoS0 {
		if len(contents)-n < 2 {
			return nil, wrapError(ErrInvalidPacketLength, "parsing PUBLISH")
		}
		nID, p.Message.ID = unpackUint16(contents[n:])
	}
	p.Message.Payload = contents[n+nID:]

	return p, nil
}

func (p *pktPublish) Pack() []byte {
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

	header := make([]byte, 0, packetBufferCap)
	header = appendString(header, p.Message.Topic)
	if p.Message.QoS != QoS0 {
		header = appendUint16(header, p.Message.ID)
	}

	return pack(
		pktHeader,
		header,
		p.Message.Payload,
	)
}

// ValidateMessage validates given message.
func (c *BaseClient) ValidateMessage(message *Message) error {
	if c.MaxPayloadLen != 0 && len(message.Payload) >= c.MaxPayloadLen {
		return wrapErrorf(ErrPayloadLenExceeded, "payload length %d", len(message.Payload))
	}
	if message.QoS > QoS2 {
		return wrapErrorf(ErrInvalidQoS, "QoS%d", int(message.QoS))
	}
	return nil
}

// Publish a message to the broker.
// ID field of the message is filled if zero.
func (c *BaseClient) Publish(ctx context.Context, message *Message) error {
	if err := c.ValidateMessage(message); err != nil {
		return err
	}

	return publishImpl(ctx, c, message)
}

func publishImpl(ctx context.Context, c *BaseClient, message *Message) error {
	c.muConnecting.RLock()
	defer c.muConnecting.RUnlock()

	if message.ID == 0 {
		message.ID = c.newID()
	}

	var chPubAck chan *pktPubAck
	var chPubRec chan *pktPubRec
	switch message.QoS {
	case QoS1:
		chPubAck = make(chan *pktPubAck, 1)
		c.sig.mu.Lock()
		if c.sig.chPubAck == nil {
			c.sig.chPubAck = make(map[uint16]chan *pktPubAck, 1)
		}
		c.sig.chPubAck[message.ID] = chPubAck
		c.sig.mu.Unlock()
	case QoS2:
		chPubRec = make(chan *pktPubRec, 1)
		c.sig.mu.Lock()
		if c.sig.chPubRec == nil {
			c.sig.chPubRec = make(map[uint16]chan *pktPubRec, 1)
		}
		c.sig.chPubRec[message.ID] = chPubRec
		c.sig.mu.Unlock()
	}

	retryPublish := func(ctx context.Context, cli *BaseClient) error {
		return publishImpl(ctx, cli, message)
	}

	println("sending PUBLISH")
	pkt := (&pktPublish{Message: message}).Pack()
	if err := c.write(pkt); err != nil {
		if message.QoS > QoS0 {
			println("retry PUBLISH queuing")
			return wrapErrorWithRetry(err, retryPublish, "sending PUBLISH")
		}
		println("retry PUBLISH aborted")
		return wrapError(err, "sending PUBLISH")
	}
	switch message.QoS {
	case QoS1:
		select {
		case <-c.connClosed:
			println("retry PUBLISH queuing")
			return wrapErrorWithRetry(ErrClosedTransport, retryPublish, "waiting PUBACK")
		case <-ctx.Done():
			println("retry PUBLISH queuing")
			return wrapErrorWithRetry(ctx.Err(), retryPublish, "waiting PUBACK")
		case <-chPubAck:
			println("PUBLISH done")
		}
	case QoS2:
		select {
		case <-c.connClosed:
			println("retry PUBLISH queuing")
			return wrapErrorWithRetry(ErrClosedTransport, retryPublish, "waiting PUBREC")
		case <-ctx.Done():
			println("retry PUBLISH queuing")
			return wrapErrorWithRetry(ctx.Err(), retryPublish, "waiting PUBREC")
		case <-chPubRec:
		}

		var retryPublish2 func(ctx context.Context, cli *BaseClient) error
		retryPublish2 = func(ctx context.Context, cli *BaseClient) error {
			chPubComp := make(chan *pktPubComp, 1)
			c.sig.mu.Lock()
			if c.sig.chPubComp == nil {
				c.sig.chPubComp = make(map[uint16]chan *pktPubComp, 1)
			}
			c.sig.chPubComp[message.ID] = chPubComp
			c.sig.mu.Unlock()

			println("sending PUBREL")
			pktPubRel := (&pktPubRel{ID: message.ID}).Pack()
			if err := c.write(pktPubRel); err != nil {
				println("retry PUBREL queuing")
				return wrapErrorWithRetry(err, retryPublish2, "sending PUBREL")
			}
			select {
			case <-c.connClosed:
				println("retry PUBREL queuing")
				return wrapErrorWithRetry(ErrClosedTransport, retryPublish2, "waiting PUBCOMP")
			case <-ctx.Done():
				println("retry PUBREL queuing")
				return wrapErrorWithRetry(ctx.Err(), retryPublish2, "waiting PUBCOMP")
			case <-chPubComp:
				println("PUBREL done")
			}
			return nil
		}
		return retryPublish2(ctx, c)
	}
	return nil
}
