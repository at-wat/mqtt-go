package mqtt

import (
	"context"
)

type publishFlag byte

const (
	publishFlagRetain publishFlag = 0x01
	publishFlagQoS0               = 0x00
	publishFlagQoS1               = 0x02
	publishFlagQoS2               = 0x04
	publishFlagDup                = 0x08
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
	if message.QoS != QoS0 {
		header = append(header, packUint16(message.ID)...)
	}
	if message.Dup {
		panic("Dup flag is set by user")
	}
	pkt := pack(pktHeader, header, message.Payload)

	chPubAck := make(chan *PubAck)
	if message.QoS != QoS0 {
		c.mu.Lock()
		if c.sig.chPubAck == nil {
			c.sig.chPubAck = make(map[uint16]chan *PubAck)
		}
		c.sig.chPubAck[message.ID] = chPubAck
		c.mu.Unlock()
	}
	_, err := c.Transport.Write(pkt)
	if err != nil {
		return err
	}
	if message.QoS != QoS0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-chPubAck:
			return nil
		}
	}
	return nil
}
