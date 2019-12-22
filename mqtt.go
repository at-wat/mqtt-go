package mqtt

import (
	"context"
)

// QoS represents quality of service level.
type QoS uint8

// QoS values.
const (
	QoS0             QoS = 0x00 // At most once delivery
	QoS1             QoS = 0x01 // At least once delivery
	QoS2             QoS = 0x02 // Exactly once delivery
	SubscribeFailure QoS = 0x80 // Rejected to subscribe
)

// Message represents MQTT message.
type Message struct {
	Topic   string
	ID      uint16
	QoS     QoS
	Retain  bool
	Dup     bool
	Payload []byte
}

// Subscription represents MQTT subscription target.
type Subscription struct {
	Topic string
	QoS   QoS
}

// ConnectOptions represents options for Connect.
type ConnectOptions struct {
	UserName     string
	Password     string
	CleanSession bool
	KeepAlive    uint16
	Will         *Message
}

// ConnectOption sets option for Connect.
type ConnectOption func(*ConnectOptions) error

// Client is the interface of MQTT client.
type Client interface {
	Connect(ctx context.Context, clientID string, opts ...ConnectOption) (sessionPresent bool, err error)
	Disconnect(ctx context.Context) error
	Publish(ctx context.Context, message *Message) error
	Subscribe(ctx context.Context, subs ...Subscription) error
	Unsubscribe(ctx context.Context, subs ...string) error
	Ping(ctx context.Context) error
}

// Closer is the interface of connection closer.
type Closer interface {
	Close() error
	Done() <-chan struct{}
	Err() error
}

// ClientCloser groups Client and Closer interface
type ClientCloser interface {
	Client
	Closer
}

// HandlerFunc type is an adapter to use functions as MQTT message handler.
type HandlerFunc func(*Message)

// Serve calls h(message).
func (h HandlerFunc) Serve(message *Message) {
	h(message)
}

// Handler receives an MQTT message.
type Handler interface {
	Serve(*Message)
}

// ConnState represents the status of MQTT connection.
type ConnState int

// ConnState values.
const (
	StateNew          ConnState = iota // initial state
	StateActive                        // connected to the broker
	StateClosed                        // connection is unexpectedly closed
	StateDisconnected                  // connection is expectedly closed
)
