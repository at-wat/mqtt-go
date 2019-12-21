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

func (c *Client) Publish(ctx context.Context, message *Message) error {
	pktHeader := packetPublish.b()
	header := packString(message.Topic)

	if message.Retain {
		pktHeader |= byte(publishFlagRetain)
	}
	switch message.QoS {
	case QoS0:
		pktHeader |= byte(publishFlagQoS0)
	case QoS1:
		pktHeader |= byte(publishFlagQoS1)
	case QoS2:
		pktHeader |= byte(publishFlagQoS2)
	default:
		panic("invalid QoS")
	}
	if message.Dup {
		pktHeader |= byte(publishFlagDup)
	}
	if message.ID != 0 {
		panic("ID is set by user")
	}
	id := newID()

	if message.QoS != QoS0 {
		header = append(header, packUint16(id)...)
	}
	pkt := pack(pktHeader, header, message.Payload)

	var chPubAck chan *pktPubAck
	var chPubRec chan *pktPubRec
	var chPubComp chan *pktPubComp
	switch message.QoS {
	case QoS1:
		chPubAck = make(chan *pktPubAck)
		c.mu.Lock()
		if c.sig.chPubAck == nil {
			c.sig.chPubAck = make(map[uint16]chan *pktPubAck)
		}
		c.sig.chPubAck[id] = chPubAck
		c.mu.Unlock()
	case QoS2:
		chPubRec = make(chan *pktPubRec)
		chPubComp = make(chan *pktPubComp)
		c.mu.Lock()
		if c.sig.chPubRec == nil {
			c.sig.chPubRec = make(map[uint16]chan *pktPubRec)
		}
		c.sig.chPubRec[id] = chPubRec
		if c.sig.chPubComp == nil {
			c.sig.chPubComp = make(map[uint16]chan *pktPubComp)
		}
		c.sig.chPubComp[id] = chPubComp
		c.mu.Unlock()
	}

	_, err := c.Transport.Write(pkt)
	if err != nil {
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
		pktPubRel := pack(packetPubRel.b()|packetFromClient.b(), packUint16(id))
		_, err := c.Transport.Write(pktPubRel)
		if err != nil {
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

type Publish struct {
	Message
}

func (p *Publish) parse(flag byte, contents []byte) *Publish {
	p.Message.Dup = (publishFlag(flag) & publishFlagDup) != 0
	p.Message.Retain = (publishFlag(flag) & publishFlagRetain) != 0
	switch publishFlag(flag) & publishFlagQoSMask {
	case publishFlagQoS0:
		p.Message.QoS = QoS0
	case publishFlagQoS1:
		p.Message.QoS = QoS1
	case publishFlagQoS2:
		p.Message.QoS = QoS2
	}

	var n, nID int
	n, p.Message.Topic = unpackString(contents)
	nID, p.Message.ID = unpackUint16(contents[n:])
	p.Message.Payload = contents[n+nID:]

	return p
}
