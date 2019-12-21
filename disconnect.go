package mqtt

import (
	"context"
)

func (c *Client) Disconnect(ctx context.Context) error {
	pkt := pack(
		byte(packetDisconnect),
		[]byte{},
		[]byte{},
	)
	_, err := c.Transport.Write(pkt)
	if err != nil {
		return err
	}
	c.connStateUpdate(StateDisconnected)
	return nil
}
