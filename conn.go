package mqtt

import (
	"errors"
	"net"
	"net/url"
)

var ErrUnsupportedProtocol = errors.New("unsupported protocol")
var ErrClosedTransport = errors.New("read/write on closed transport")

func (c *Client) Dial(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "tcp", "mqtt":
		conn, err := net.Dial("tcp", u.Host)
		if err != nil {
			return err
		}
		c.Transport = conn
	default:
		return ErrUnsupportedProtocol
	}
	c.connStateUpdate(StateActive)
	c.connClosed = make(chan struct{})

	go func() {
		err := c.serve()
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
	}()

	return nil
}

func (c *Client) connStateUpdate(newState ConnState) {
	c.mu.Lock()
	lastState := c.connState
	if c.connState != StateDisconnected {
		c.connState = newState
	}
	state := c.connState
	err := c.err
	c.mu.Unlock()

	if c.ConnState != nil && lastState != state {
		c.ConnState(state, err)
	}
}
