// Copyright 2019 The mqtt-go authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mqtt

import (
	"io"
)

func (c *BaseClient) serve() error {
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
		for shift := uint(0); ; shift += 7 {
			b := make([]byte, 1)
			if _, err := io.ReadFull(r, b); err != nil {
				return err
			}
			remainingLength |= (int(b[0]) & 0x7F) << shift
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
			connAck, err := (&pktConnAck{}).parse(pktFlag, contents)
			if err != nil {
				// Client must close connection if packet is invalid.
				return err
			}
			select {
			case c.sig.ConnAck() <- connAck:
			default:
			}
		case packetPublish:
			publish, err := (&pktPublish{}).parse(pktFlag, contents)
			if err != nil {
				return err
			}
			switch publish.Message.QoS {
			case QoS0:
				c.mu.RLock()
				handler := c.handler
				c.mu.RUnlock()
				if handler != nil {
					handler.Serve(publish.Message)
				}
			case QoS1:
				// Ownership of the message is now transferred to the receiver.
				c.mu.RLock()
				handler := c.handler
				c.mu.RUnlock()
				if handler != nil {
					handler.Serve(publish.Message)
				}
				pktPubAck := (&pktPubAck{ID: publish.Message.ID}).pack()
				if err := c.write(pktPubAck); err != nil {
					return err
				}
			case QoS2:
				pktPubRec := (&pktPubRec{ID: publish.Message.ID}).pack()
				if err := c.write(pktPubRec); err != nil {
					return err
				}
				subBuffer[publish.Message.ID] = publish.Message
			}
		case packetPubAck:
			pubAck, err := (&pktPubAck{}).parse(pktFlag, contents)
			if err != nil {
				return err
			}
			if ch, ok := c.sig.PubAck(pubAck.ID); ok {
				select {
				case ch <- pubAck:
				default:
				}
			}
		case packetPubRec:
			pubRec, err := (&pktPubRec{}).parse(pktFlag, contents)
			if err != nil {
				return err
			}
			if ch, ok := c.sig.PubRec(pubRec.ID); ok {
				select {
				case ch <- pubRec:
				default:
				}
			}
		case packetPubRel:
			pubRel, err := (&pktPubRel{}).parse(pktFlag, contents)
			if err != nil {
				return err
			}
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

			pktPubComp := (&pktPubComp{ID: pubRel.ID}).pack()
			if err := c.write(pktPubComp); err != nil {
				return err
			}
		case packetPubComp:
			pubComp, err := (&pktPubComp{}).parse(pktFlag, contents)
			if err != nil {
				return err
			}
			if ch, ok := c.sig.PubComp(pubComp.ID); ok {
				select {
				case ch <- pubComp:
				default:
				}
			}
		case packetSubAck:
			subAck, err := (&pktSubAck{}).parse(pktFlag, contents)
			if err != nil {
				return err
			}
			if ch, ok := c.sig.SubAck(subAck.ID); ok {
				select {
				case ch <- subAck:
				default:
				}
			}
		case packetUnsubAck:
			unsubAck, err := (&pktUnsubAck{}).parse(pktFlag, contents)
			if err != nil {
				return err
			}
			if ch, ok := c.sig.UnsubAck(unsubAck.ID); ok {
				select {
				case ch <- unsubAck:
				default:
				}
			}
		case packetPingResp:
			pingResp, err := (&pktPingResp{}).parse(pktFlag, contents)
			if err != nil {
				return err
			}
			select {
			case c.sig.PingResp() <- pingResp:
			default:
			}
		default:
			// must close connection if the client encountered protocol violation.
			return wrapErrorf(ErrInvalidPacket, "serving incoming packet %x", int(pktType))
		}
	}
}
