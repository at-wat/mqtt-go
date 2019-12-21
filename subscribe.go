package mqtt

import (
	"context"
	"errors"
)

// ErrInvalidSubAck means that the incomming SUBACK packet is inconsistent with the request.
var ErrInvalidSubAck = errors.New("invalid SUBACK")

type subscribeFlag byte

const (
	subscribeFlagQoS0 subscribeFlag = 0x00
	subscribeFlagQoS1 subscribeFlag = 0x01
	subscribeFlagQoS2 subscribeFlag = 0x02
)

// Subscribe topics.
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

	chSubAck := make(chan *pktSubAck, 1)
	c.sig.mu.Lock()
	if c.sig.chSubAck == nil {
		c.sig.chSubAck = make(map[uint16]chan *pktSubAck)
	}
	c.sig.chSubAck[id] = chSubAck
	c.sig.mu.Unlock()

	if err := c.write(pkt); err != nil {
		return err
	}
	select {
	case <-c.connClosed:
		return ErrClosedTransport
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
