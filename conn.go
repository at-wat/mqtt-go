package mqtt

import (
	"errors"
	"fmt"
	"net"
	"net/url"

	"golang.org/x/net/websocket"
)

var ErrUnsupportedProtocol = errors.New("unsupported protocol")
var ErrClosedTransport = errors.New("read/write on closed transport")

func Dial(urlStr string) (*Client, error) {
	c := &Client{}
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "tcp", "mqtt":
		conn, err := net.Dial("tcp", u.Host)
		if err != nil {
			return nil, err
		}
		c.Transport = conn
	case "ws":
		wsc, err := websocket.NewConfig(
			fmt.Sprintf("ws://%s:%s%s", u.Hostname(), u.Port(), u.EscapedPath()), "ws://")
		if err != nil {
			return nil, err
		}
		wsc.Protocol = append(wsc.Protocol, "mqtt")
		ws, err := websocket.DialConfig(wsc)
		if err != nil {
			return nil, err
		}
		var p net.Conn
		p, c.Transport = net.Pipe()
		go func() {
			for {
				var b []byte
				if err := websocket.Message.Receive(ws, &b); err != nil {
					p.Close()
					return
				}
				if _, err := p.Write(b); err != nil {
					return
				}
			}
		}()
		go func() {
			b := make([]byte, 1024)
			for {
				n, err := p.Read(b)
				if err != nil {
					return
				}
				if err := websocket.Message.Send(ws, b[:n]); err != nil {
					p.Close()
					return
				}
			}
		}()
	default:
		return nil, ErrUnsupportedProtocol
	}
	c.connStateUpdate(StateActive)
	c.connClosed = make(chan struct{})

	go func() {
		err := c.serve()
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
	}()

	return c, nil
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
