// Copyright 2020 The mqtt-go authors.
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

package mqtt

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestServeAsync(t *testing.T) {
	processed := make(chan byte, 10)
	var wg sync.WaitGroup

	s := &ServeAsync{HandlerFunc(func(m *Message) {
		time.Sleep(time.Duration(m.Payload[1]) * time.Millisecond)
		processed <- m.Payload[0]
		wg.Done()
	})}

	wg.Add(3)
	s.Serve(&Message{Payload: []byte{0x00, 0x10}})
	s.Serve(&Message{Payload: []byte{0x01, 0x08}})
	s.Serve(&Message{Payload: []byte{0x02, 0x18}})
	wg.Wait()
	close(processed)

	var order []byte
	for id := range processed {
		order = append(order, id)
	}

	expected := []byte{0x01, 0x00, 0x02}
	if !bytes.Equal(expected, order) {
		t.Errorf("Order of the process is expected to be: %v, got: %v", expected, order)
	}
}
