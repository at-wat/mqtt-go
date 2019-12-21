package mqtt

import (
	"context"
	"errors"
)

var ErrInvalidUnsubAck = errors.New("invalid SUBACK")

func (c *Client) Unsubscribe(ctx context.Context, subs ...string) error {
	pktHeader := byte(packetUnsubscribe | packetFromClient)

	id := newID()
	header := packUint16(id)

	var payload []byte
	for _, sub := range subs {
		payload = append(payload, packString(sub)...)
	}
	pkt := pack(pktHeader, header, payload)

	var chUnsubAck chan *UnsubAck
	chUnsubAck = make(chan *UnsubAck)
	c.mu.Lock()
	if c.sig.chUnsubAck == nil {
		c.sig.chUnsubAck = make(map[uint16]chan *UnsubAck)
	}
	c.sig.chUnsubAck[id] = chUnsubAck
	c.mu.Unlock()

	_, err := c.Transport.Write(pkt)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-chUnsubAck:
	}
	return nil
}
