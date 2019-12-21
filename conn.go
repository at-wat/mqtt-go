package mqtt

import (
	"errors"
	"net"
	"net/url"
)

var ErrUnsupportedProtocol = errors.New("unsupported protocol")

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
	return nil
}
