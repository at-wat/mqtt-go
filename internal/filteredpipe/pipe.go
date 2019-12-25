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

// Package filteredpipe provides pipes with interceptor for testing.
package filteredpipe

import (
	"bytes"
	"io"
	"sync"
)

// DetectAndClosePipe creates pair of filtered pipe.
// Handler is called on each Write and determine to close the connection.
func DetectAndClosePipe(h0, h1 func([]byte) bool) (io.ReadWriteCloser, io.ReadWriteCloser) {
	ch0 := make(chan []byte, 1000)
	ch1 := make(chan []byte, 1000)
	return &conn{
			rCh:     ch0,
			wCh:     ch1,
			handler: h0,
			closed:  make(chan struct{}),
		}, &conn{
			rCh:     ch1,
			wCh:     ch0,
			handler: h1,
			closed:  make(chan struct{}),
		}
}

type conn struct {
	rCh       chan []byte
	wCh       chan []byte
	handler   func([]byte) bool
	closed    chan struct{}
	closeOnce sync.Once
	remain    io.Reader
}

func (c *conn) Read(data []byte) (n int, err error) {
	if c.remain != nil {
		n, _ := c.remain.Read(data)
		if n == 0 {
			select {
			case <-c.closed:
				return 0, io.EOF
			default:
			}
			c.remain = nil
			return c.Read(data)
		}
		return n, nil
	}
	select {
	case d := <-c.rCh:
		c.remain = bytes.NewReader(d)
		for {
			select {
			case d := <-c.rCh:
				c.remain = io.MultiReader(c.remain, bytes.NewReader(d))
			case <-c.closed:
				return 0, io.EOF
			default:
				return c.Read(data)
			}
		}
	case <-c.closed:
		return 0, io.EOF
	}
}

func (c *conn) Write(data []byte) (n int, err error) {
	if c.handler(data) {
		c.closeOnce.Do(func() { close(c.closed) })
		return 0, io.ErrClosedPipe
	}
	select {
	case <-c.closed:
		return 0, io.ErrClosedPipe
	default:
	}
	cp := append([]byte{}, data...)
	select {
	case <-c.closed:
		return 0, io.ErrClosedPipe
	case c.wCh <- cp:
	}
	return len(cp), nil
}

func (c *conn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

// Connect two io.ReadWriteCloser.
func Connect(conn0, conn1 io.ReadWriteCloser) {
	go func() {
		_, _ = io.Copy(conn0, conn1)
		_ = conn0.Close()
		_ = conn1.Close()
	}()
	go func() {
		_, _ = io.Copy(conn1, conn0)
		_ = conn0.Close()
		_ = conn1.Close()
	}()
}
