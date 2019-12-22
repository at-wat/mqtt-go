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
	connectFlagWill         connectFlag = 0x04
	connectFlagWillQoS0     connectFlag = 0x00
	connectFlagWillQoS1     connectFlag = 0x08
	connectFlagWillQoS2     connectFlag = 0x10
	connectFlagWillRetain   connectFlag = 0x20
	connectFlagPassword     connectFlag = 0x40
	connectFlagUserName     connectFlag = 0x80
)

// Connect to the broker.
func (c *BaseClient) Connect(ctx context.Context, clientID string, opts ...ConnectOption) error {
	o := &ConnectOptions{
		KeepAlive: 60,
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return err
		}
	}
	c.sig = &signaller{}

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
		payload = append(payload, packString(o.Will.Topic)...)
		payload = append(payload, packBytes(o.Will.Payload)...)
	}
	if o.UserName != "" {
		flag |= byte(connectFlagUserName)
		payload = append(payload, packString(o.UserName)...)
	}
	if o.Password != "" {
		flag |= byte(connectFlagPassword)
		payload = append(payload, packString(o.Password)...)
	}
	pkt := pack(
		packetConnect.b(),
		[]byte{
			0x00, 0x04, 0x4D, 0x51, 0x54, 0x54,
			byte(protocol311),
			flag,
		},
		packUint16(o.KeepAlive),
		payload,
	)

	chConnAck := make(chan *pktConnAck, 1)
	c.mu.Lock()
	c.sig.chConnAck = chConnAck
	c.mu.Unlock()
	if err := c.write(pkt); err != nil {
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
		c.connStateUpdate(StateActive)
	}
	return nil
}