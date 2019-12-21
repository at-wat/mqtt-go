package mqtt

import (
	"context"
	"errors"
)

type protocolLevel byte

const (
	protocol311 protocolLevel = 0x04
)

type connectFlag byte

const (
	connectFlagCleanSession connectFlag = 0x02
	connectFlagWill                     = 0x04
	connectFlagWillQoS0                 = 0x00
	connectFlagWillQoS1                 = 0x08
	connectFlagWillQoS2                 = 0x10
	connectFlagWillRetain               = 0x20
	connectFlagPassword                 = 0x40
	connectFlagUserName                 = 0x80
)

const connectFlagWillQoSBit = 3

func (c *Client) Connect(ctx context.Context, clientID string, opts ...ConnectOption) error {
	o := &ConnectOptions{
		KeepAlive: 60,
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return err
		}
	}

	payload := packString(clientID)

	var flag byte
	if o.CleanSession {
		flag |= byte(connectFlagCleanSession)
	}
	if o.Will != nil {
		flag |= byte(connectFlagWill)
		switch o.Will.QoS {
		case QoS0:
			flag |= byte(connectFlagWillQoS0)
		case QoS1:
			flag |= byte(connectFlagWillQoS1)
		case QoS2:
			flag |= byte(connectFlagWillQoS2)
		default:
			panic("invalid QoS")
		}
		if o.Will.Retain {
			flag |= byte(connectFlagWillRetain)
		}
		payload = packString(o.Will.Topic)
		payload = packBytes(o.Will.Payload)
	}
	if o.UserName != "" {
		flag |= byte(connectFlagUserName)
		payload = packString(o.UserName)
	}
	if o.Password != "" {
		flag |= byte(connectFlagPassword)
		payload = packString(o.Password)
	}
	pkt := pack(
		byte(packetConnect),
		[]byte{
			0x00, 0x04, 0x4D, 0x51, 0x54, 0x54,
			byte(protocol311),
			byte(connectFlagCleanSession),
		},
		packUint16(o.KeepAlive),
		payload,
	)

	chConnAck := make(chan *ConnAck)
	c.mu.Lock()
	c.sig.chConnAck = chConnAck
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
	case connAck := <-chConnAck:
		if connAck.Code != ConnectionAccepted {
			return errors.New(connAck.Code.String())
		}
	}
	return nil
}
