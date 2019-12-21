package mqtt

import (
	"context"
)

func (c *Client) Ping(ctx context.Context) error {
	pktHeader := byte(packetPingReq)
	pkt := pack(pktHeader)

	var chPingResp chan *PingResp
	chPingResp = make(chan *PingResp)
	c.mu.Lock()
	c.sig.chPingResp = chPingResp
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
	case <-chPingResp:
	}
	return nil
}
