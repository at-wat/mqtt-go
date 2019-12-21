package mqtt

import (
	"context"
)

func (c *Client) Connect(ctx context.Context, clientID string, opts ...ConnectOption) (*ConnAck, error) {
	o := &ConnectOptions{
		KeepAlive: 60,
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}

	payload := packString(clientID)

	var flag byte
	if o.CleanSession {
		flag |= byte(flagCleanSession)
	}
	if o.Will != nil {
		flag |= byte(flagWill)
		switch o.Will.QoS {
		case QoS0:
			flag |= byte(flagWillQoS0)
		case QoS1:
			flag |= byte(flagWillQoS1)
		case QoS2:
			flag |= byte(flagWillQoS2)
		default:
			panic("invalid QoS")
		}
		if o.Will.Retain {
			flag |= byte(flagWillRetain)
		}
		payload = packString(o.Will.Topic)
		payload = packString(string(o.Will.Message))
	}
	if o.UserName != "" {
		flag |= byte(flagUserName)
		payload = packString(o.UserName)
	}
	if o.Password != "" {
		flag |= byte(flagPassword)
		payload = packString(o.Password)
	}
	pkt := pack(
		byte(packetConnect),
		[]byte{
			0x00, 0x04, 0x4D, 0x51, 0x54, 0x54,
			byte(protocol311),
			byte(flagCleanSession),
			byte(o.KeepAlive >> 8), byte(o.KeepAlive),
		},
		payload,
	)
	c.chConnAck = make(chan *ConnAck)
	_, err := c.Transport.Write(pkt)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ack := <-c.chConnAck:
		return ack, nil
	}
}

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
	return nil
}
