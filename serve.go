package mqtt

import (
	"errors"
	"io"
)

// ErrInvalidPacket means that an invalid message is arrived from the broker.
var ErrInvalidPacket = errors.New("invalid packet")

func (c *BaseClient) serve() error {
	defer func() {
		close(c.connClosed)
	}()
	r := c.Transport
	subBuffer := make(map[uint16]*Message)
	for {
		pktTypeBytes := make([]byte, 1)
		if _, err := io.ReadFull(r, pktTypeBytes); err != nil {
			return err
		}
		pktType := packetType(pktTypeBytes[0] & 0xF0)
		pktFlag := pktTypeBytes[0] & 0x0F
		var remainingLength int
		for {
			b := make([]byte, 1)
			if _, err := io.ReadFull(r, b); err != nil {
				return err
			}
			remainingLength = (remainingLength << 7) | (int(b[0]) & 0x7F)
			if !(b[0]&0x80 != 0) {
				break
			}
		}
		contents := make([]byte, remainingLength)
		if _, err := io.ReadFull(r, contents); err != nil {
			return err
		}
		// fmt.Printf("%s: %v\n", pktType, contents)

		switch pktType {
		case packetConnAck:
			select {
			case c.sig.ConnAck() <- (&pktConnAck{}).parse(pktFlag, contents):
			default:
			}
		case packetPublish:
			publish := (&pktPublish{}).parse(pktFlag, contents)
			switch publish.Message.QoS {
			case QoS0:
				c.mu.RLock()
				handler := c.handler
				c.mu.RUnlock()
				if handler != nil {
					handler.Serve(&publish.Message)
				}
			case QoS1:
				// Ownership of the message is now transferred to the receiver.
				c.mu.RLock()
				handler := c.handler
				c.mu.RUnlock()
				if handler != nil {
					handler.Serve(&publish.Message)
				}
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
				subBuffer[publish.Message.ID] = &publish.Message
			}
		case packetPubAck:
			pubAck := (&pktPubAck{}).parse(pktFlag, contents)
			if ch, ok := c.sig.PubAck(pubAck.ID); ok {
				select {
				case ch <- pubAck:
				default:
				}
			}
		case packetPubRec:
			pubRec := (&pktPubRec{}).parse(pktFlag, contents)
			if ch, ok := c.sig.PubRec(pubRec.ID); ok {
				select {
				case ch <- pubRec:
				default:
				}
			}
		case packetPubRel:
			pubRel := (&pktPubRel{}).parse(pktFlag, contents)
			if msg, ok := subBuffer[pubRel.ID]; ok {
				// Ownership of the message is now transferred to the receiver.
				c.mu.RLock()
				handler := c.handler
				c.mu.RUnlock()
				if handler != nil {
					handler.Serve(msg)
				}
				delete(subBuffer, pubRel.ID)
			}

			pktPubComp := pack(
				packetPubComp.b()|packetFromClient.b(),
				packUint16(pubRel.ID),
			)
			if err := c.write(pktPubComp); err != nil {
				return err
			}
		case packetPubComp:
			pubComp := (&pktPubComp{}).parse(pktFlag, contents)
			if ch, ok := c.sig.PubComp(pubComp.ID); ok {
				select {
				case ch <- pubComp:
				default:
				}
			}
		case packetSubAck:
			subAck := (&pktSubAck{}).parse(pktFlag, contents)
			if ch, ok := c.sig.SubAck(subAck.ID); ok {
				select {
				case ch <- subAck:
				default:
				}
			}
		case packetUnsubAck:
			unsubAck := (&pktUnsubAck{}).parse(pktFlag, contents)
			if ch, ok := c.sig.UnsubAck(unsubAck.ID); ok {
				select {
				case ch <- unsubAck:
				default:
				}
			}
		case packetPingResp:
			pingResp := (&pktPingResp{}).parse(pktFlag, contents)
			select {
			case c.sig.PingResp() <- pingResp:
			default:
			}
		default:
			// must close connection if the client encounted protocol violation.
			return ErrInvalidPacket
		}
	}
}
