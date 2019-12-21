package mqtt

import (
	"fmt"
	"io"
)

func (c *Client) Serve() error {
	defer func() {
	}()
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
		case packetPubRec:
			if sig.chPubRec != nil {
				pubRec := (&PubRec{}).parse(pktFlag, contents)
				select {
				case sig.chPubRec[pubRec.ID] <- pubRec:
				}
			}
		case packetPubComp:
			if sig.chPubComp != nil {
				pubComp := (&PubComp{}).parse(pktFlag, contents)
				select {
				case sig.chPubComp[pubComp.ID] <- pubComp:
				}
			}
		case packetSubAck:
			if sig.chSubAck != nil {
				subAck := (&SubAck{}).parse(pktFlag, contents)
				select {
				case sig.chSubAck[subAck.ID] <- subAck:
				}
			}
		}
	}
}

func (c *Client) Stop() error {
	return c.Transport.Close()
}
