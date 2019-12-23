package mqtt

import (
	"context"
	"errors"
	"io"
)

// ProtocolLevel represents MQTT protocol level.
type ProtocolLevel byte

// ProtocolLevel values.
const (
	ProtocolLevel3 ProtocolLevel = 0x03 // MQTT 3.1
	ProtocolLevel4 ProtocolLevel = 0x04 // MQTT 3.1.1 (default)
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
func (c *BaseClient) Connect(ctx context.Context, clientID string, opts ...ConnectOption) (sessionPresent bool, err error) {
	o := &ConnectOptions{
		ProtocolLevel: ProtocolLevel4,
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return false, err
		}
	}
	c.sig = &signaller{}
	c.connClosed = make(chan struct{})
	c.initID()

	go func() {
		err := c.serve()
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			if errConn := c.Close(); errConn != nil && err == nil {
				err = errConn
			}
		}
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		c.connStateUpdate(StateClosed)
		close(c.connClosed)
	}()
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
			byte(o.ProtocolLevel),
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
		return false, err
	}
	select {
	case <-c.connClosed:
		return false, ErrClosedTransport
	case <-ctx.Done():
		return false, ctx.Err()
	case connAck := <-chConnAck:
		if connAck.Code != ConnectionAccepted {
			return false, errors.New(connAck.Code.String())
		}
		c.connStateUpdate(StateActive)
		return connAck.SessionPresent, nil
	}
}

// ConnectOptions represents options for Connect.
type ConnectOptions struct {
	UserName      string
	Password      string
	CleanSession  bool
	KeepAlive     uint16
	Will          *Message
	ProtocolLevel ProtocolLevel
}

// ConnectOption sets option for Connect.
type ConnectOption func(*ConnectOptions) error

// WithUserNamePassword sets plain text auth information used in Connect.
func WithUserNamePassword(userName, password string) ConnectOption {
	return func(o *ConnectOptions) error {
		o.UserName = userName
		o.Password = password
		return nil
	}
}

// WithKeepAlive sets keep alive interval in seconds.
func WithKeepAlive(interval uint16) ConnectOption {
	return func(o *ConnectOptions) error {
		o.KeepAlive = interval
		return nil
	}
}

// WithCleanSession sets clean session flag.
func WithCleanSession(cleanSession bool) ConnectOption {
	return func(o *ConnectOptions) error {
		o.CleanSession = cleanSession
		return nil
	}
}

// WithWill sets will message.
func WithWill(will *Message) ConnectOption {
	return func(o *ConnectOptions) error {
		o.Will = will
		return nil
	}
}

// WithProtocolLevel sets protocol level.
func WithProtocolLevel(level ProtocolLevel) ConnectOption {
	return func(o *ConnectOptions) error {
		o.ProtocolLevel = level
		return nil
	}
}
