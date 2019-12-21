package mqtt

import (
	"context"
	"errors"
)

func (c *Client) Unsubscribe(ctx context.Context, subs ...string) error {
	pktHeader := byte(packetUnsubscribe | packetFromClient)

	id := newID()
	header := packUint16(id)

	var payload []byte
	for _, sub := range subs {
		payload = append(payload, packString(sub)...)
	}
	pkt := pack(pktHeader, header, payload)

	chUnsubAck := make(chan *pktUnsubAck)
	c.mu.Lock()
	if c.sig.chUnsubAck == nil {
		c.sig.chUnsubAck = make(map[uint16]chan *pktUnsubAck)
	}
	c.sig.chUnsubAck[id] = chUnsubAck
	c.mu.Unlock()

	_, err := c.Transport.Write(pkt)
	if err != nil {
		return err
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
