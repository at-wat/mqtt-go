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

package filteredpipe

import (
	"io"
	"sync"
)

// DetectAndDropPipe creates pair of filtered pipe.
// Handler is called on each Write and determine to drop the remaining data.
func DetectAndDropPipe(h0, h1 func([]byte) bool) (io.ReadWriteCloser, io.ReadWriteCloser) {
	ch0 := make(chan []byte, 1000)
	ch1 := make(chan []byte, 1000)
	closed, fnClose := mewCloseCh()
	return &detectAndDropConn{
			baseFilterConn: &baseFilterConn{
				rCh:     ch0,
				wCh:     ch1,
				handler: h0,
				closed:  closed,
				fnClose: fnClose,
			},
			dropping: make(chan struct{}),
		}, &detectAndDropConn{
			baseFilterConn: &baseFilterConn{
				rCh:     ch1,
				wCh:     ch0,
				handler: h1,
				closed:  closed,
				fnClose: fnClose,
			},
			dropping: make(chan struct{}),
		}
}

type detectAndDropConn struct {
	*baseFilterConn
	dropping chan struct{}
	dropOnce sync.Once
}

func (c *detectAndDropConn) Write(data []byte) (n int, err error) {
	if c.handler(data) {
		c.dropOnce.Do(func() { close(c.dropping) })
	}
	select {
	case <-c.closed:
		return 0, io.ErrClosedPipe
	case <-c.dropping:
		return len(data), nil
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
