package mqtt

import (
	"fmt"
	"io"
)

func (c *Client) Serve() error {
	for {
		pktTypeBytes := make([]byte, 1)
		if _, err := io.ReadFull(c.Transport, pktTypeBytes); err != nil {
			return err
		}
		pktType := packetType(pktTypeBytes[0] & 0xF0)
		pktFlag := pktTypeBytes[0] & 0x0F
		var remainingLength int
		for {
			b := make([]byte, 1)
			if _, err := io.ReadFull(c.Transport, b); err != nil {
				return err
			}
			remainingLength = (remainingLength << 7) | (int(b[0]) & 0x7F)
			if !(b[0]&0x80 != 0) {
				break
			}
		}
		contents := make([]byte, remainingLength)
		if _, err := io.ReadFull(c.Transport, contents); err != nil {
			return err
		}
		fmt.Printf("%s: %v\n", pktType, contents)

		c.mu.RLock()
		sig := c.sig
		c.mu.RUnlock()

		switch pktType {
		case packetConnAck:
			select {
			case sig.chConnAck <- (&ConnAck{}).parse(pktFlag, contents):
			}
		case packetPubAck:
			if sig.chPubAck != nil {
				pubAck := (&PubAck{}).parse(pktFlag, contents)
				select {
				case sig.chPubAck[pubAck.ID] <- pubAck:
				}
			}
		}
	}
}

func (c *Client) Stop() error {
	return c.Transport.Close()
}
