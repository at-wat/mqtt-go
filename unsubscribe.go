package mqtt

import (
	"context"
	"errors"
)

var ErrInvalidUnsubAck = errors.New("invalid SUBACK")

func (c *Client) Unsubscribe(ctx context.Context, topics []string) error {
	pktHeader := byte(packetUnsubscribe | packetFromClient)

	id := newID()
	header := packUint16(id)

	var payload []byte
	for _, topic := range topics {
		payload = append(payload, packString(topic)...)
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
