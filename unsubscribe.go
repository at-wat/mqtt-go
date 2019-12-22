package mqtt

import (
	"context"
)

// Unsubscribe topics.
func (c *BaseClient) Unsubscribe(ctx context.Context, subs ...string) error {
	pktHeader := byte(packetUnsubscribe | packetFromClient)

	id := newID()
	header := packUint16(id)

	var payload []byte
	for _, sub := range subs {
		payload = append(payload, packString(sub)...)
	}
	pkt := pack(pktHeader, header, payload)

	chUnsubAck := make(chan *pktUnsubAck, 1)
	c.sig.mu.Lock()
	if c.sig.chUnsubAck == nil {
		c.sig.chUnsubAck = make(map[uint16]chan *pktUnsubAck)
	}
	c.sig.chUnsubAck[id] = chUnsubAck
	c.sig.mu.Unlock()

	if err := c.write(pkt); err != nil {
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
