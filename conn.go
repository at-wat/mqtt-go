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
	return nil
}
