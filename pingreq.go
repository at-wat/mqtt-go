package mqtt

import (
	"context"
)

// Ping to the broker.
func (c *BaseClient) Ping(ctx context.Context) error {
	pkt := pack(packetPingReq.b())

	chPingResp := make(chan *pktPingResp, 1)
	c.mu.Lock()
	c.sig.chPingResp = chPingResp
	c.mu.Unlock()

	if err := c.write(pkt); err != nil {
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