package mqtt

import (
	"io"
)

func (c *Client) serve() error {
	defer func() {
		c.connStateUpdate(StateClosed)
		close(c.connClosed)
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
		// fmt.Printf("%s: %v\n", pktType, contents)

		c.mu.RLock()
		sig := c.sig.Copy()
		c.mu.RUnlock()

		switch pktType {
		case packetConnAck:
			select {
			case sig.chConnAck <- (&pktConnAck{}).parse(pktFlag, contents):
			default:
			}
		case packetPublish:
			publish := (&pktPublish{}).parse(pktFlag, contents)
			if c.Handler != nil {
				c.Handler.Serve(&publish.Message)
			}
			switch publish.Message.QoS {
			case QoS1:
				pktPubAck := pack(
					packetPubAck.b()|packetFromClient.b(),
					packUint16(publish.Message.ID),
				)
				if err := c.write(pktPubAck); err != nil {
					return err
				}
			case QoS2:
				pktPubRec := pack(
					packetPubRec.b()|packetFromClient.b(),
					packUint16(publish.Message.ID),
				)
				if err := c.write(pktPubRec); err != nil {
					return err
				}
			}
		case packetPubAck:
			if sig.chPubAck != nil {
				pubAck := (&pktPubAck{}).parse(pktFlag, contents)
				select {
				case sig.chPubAck[pubAck.ID] <- pubAck:
				default:
				}
			}
		case packetPubRec:
			if sig.chPubRec != nil {
				pubRec := (&pktPubRec{}).parse(pktFlag, contents)
				select {
				case sig.chPubRec[pubRec.ID] <- pubRec:
				default:
				}
			}
		case packetPubRel:
			pubRel := (&pktPubRel{}).parse(pktFlag, contents)
			pktPubComp := pack(
				packetPubComp.b()|packetFromClient.b(),
				packUint16(pubRel.ID),
			)
			if err := c.write(pktPubComp); err != nil {
				return err
			}
		case packetPubComp:
			if sig.chPubComp != nil {
				pubComp := (&pktPubComp{}).parse(pktFlag, contents)
				select {
				case sig.chPubComp[pubComp.ID] <- pubComp:
				default:
				}
			}
		case packetSubAck:
			if sig.chSubAck != nil {
				subAck := (&pktSubAck{}).parse(pktFlag, contents)
				select {
				case sig.chSubAck[subAck.ID] <- subAck:
				default:
				}
			}
		case packetUnsubAck:
			if sig.chUnsubAck != nil {
				unsubAck := (&pktUnsubAck{}).parse(pktFlag, contents)
				select {
				case sig.chUnsubAck[unsubAck.ID] <- unsubAck:
				default:
				}
			}
		case packetPingResp:
			pingResp := (&pktPingResp{}).parse(pktFlag, contents)
			select {
			case sig.chPingResp <- pingResp:
			default:
			}
		}
	}
}
