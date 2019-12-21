package mqtt

import (
	"context"
)

type publishFlag byte

const (
	publishFlagRetain  publishFlag = 0x01
	publishFlagQoS0                = 0x00
	publishFlagQoS1                = 0x02
	publishFlagQoS2                = 0x04
	publishFlagQoSMask             = 0x06
	publishFlagDup                 = 0x08
)

func (c *Client) Publish(ctx context.Context, message *Message) error {
	pktHeader := byte(packetPublish)
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
	if message.ID != 0 {
		panic("ID is set by user")
	}
	id := newID()

	if message.QoS != QoS0 {
		header = append(header, packUint16(id)...)
	}
	if message.Dup {
		panic("Dup flag is set by user")
	}
	pkt := pack(pktHeader, header, message.Payload)

	var chPubAck chan *PubAck
	var chPubRec chan *PubRec
	var chPubComp chan *PubComp
	switch message.QoS {
	case QoS1:
		chPubAck = make(chan *PubAck)
		c.mu.Lock()
		if c.sig.chPubAck == nil {
			c.sig.chPubAck = make(map[uint16]chan *PubAck)
		}
		c.sig.chPubAck[id] = chPubAck
		c.mu.Unlock()
	case QoS2:
		chPubRec = make(chan *PubRec)
		chPubComp = make(chan *PubComp)
		c.mu.Lock()
		if c.sig.chPubRec == nil {
			c.sig.chPubRec = make(map[uint16]chan *PubRec)
		}
		c.sig.chPubRec[id] = chPubRec
		if c.sig.chPubComp == nil {
			c.sig.chPubComp = make(map[uint16]chan *PubComp)
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
		pktPubRel := pack(packetPubRel|packetFromClient, packUint16(id))
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
