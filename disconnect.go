package mqtt

import (
	"context"
)

// Disconnect from the broker.
func (c *Client) Disconnect(ctx context.Context) error {
	pkt := pack(
		packetDisconnect.b(),
		[]byte{},
		[]byte{},
	)
	c.connStateUpdate(StateDisconnected)
	if err := c.write(pkt); err != nil {
		return err
	}
	c.Transport.Close()
	return nil
}
