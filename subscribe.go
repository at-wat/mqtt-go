package mqtt

import (
	"context"
	"errors"
)

var ErrInvalidSubAck = errors.New("invalid SUBACK")

type subscribeFlag byte

const (
	subscribeFlagQoS0    = 0x00
	subscribeFlagQoS1    = 0x01
	subscribeFlagQoS2    = 0x02
	subscribeFlagFailure = 0x80
)

func (c *Client) Subscribe(ctx context.Context, subs ...Subscription) error {
	pktHeader := byte(packetSubscribe | packetFromClient)

	id := newID()
	header := packUint16(id)

	var payload []byte
	for _, sub := range subs {
		payload = append(payload, packString(sub.Topic)...)

		var flag byte
		switch sub.QoS {
		case QoS0:
			flag |= byte(subscribeFlagQoS0)
		case QoS1:
			flag |= byte(subscribeFlagQoS1)
		case QoS2:
			flag |= byte(subscribeFlagQoS2)
		default:
			panic("invalid QoS")
		}
		payload = append(payload, flag)
	}
	pkt := pack(pktHeader, header, payload)

	var chSubAck chan *SubAck
	chSubAck = make(chan *SubAck)
	c.mu.Lock()
	if c.sig.chSubAck == nil {
		c.sig.chSubAck = make(map[uint16]chan *SubAck)
	}
	c.sig.chSubAck[id] = chSubAck
	c.mu.Unlock()

	_, err := c.Transport.Write(pkt)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case subAck := <-chSubAck:
		if len(subAck.Codes) != len(subs) {
			return ErrInvalidSubAck
		}
		for i := 0; i < len(subAck.Codes); i++ {
			subs[i].QoS = QoS(subAck.Codes[i])
		}
	}
	return nil
}
