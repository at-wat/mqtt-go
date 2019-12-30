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

package mqtt

import (
	"reflect"
	"testing"

	"github.com/at-wat/mqtt-go/internal/errs"
)

func TestServeMux_HandleFunc(t *testing.T) {
	mux := &ServeMux{}

	if err := mux.HandleFunc("", func(*Message) {}); !errs.Is(err, ErrInvalidTopicFilter) {
		t.Errorf("Expect error against invalid filter: %v, got: %v", ErrInvalidTopicFilter, err)
	}
	topic1 := []Message{}
	if err := mux.HandleFunc("test/+", func(m *Message) {
		topic1 = append(topic1, *m)
	}); err != nil {
		t.Errorf("Unexpect error: %v", err)
	}
	topic2 := []Message{}
	if err := mux.HandleFunc("test2", func(m *Message) {
		topic2 = append(topic2, *m)
	}); err != nil {
		t.Errorf("Unexpect error: %v", err)
	}

	mux.Serve(&Message{Topic: "test2", Payload: []byte{0x01}})
	mux.Serve(&Message{Topic: "test1", Payload: []byte{0x02}})
	mux.Serve(&Message{Topic: "test/1", Payload: []byte{0x03}})
	mux.Serve(&Message{Topic: "test/2", Payload: []byte{0x04}})

	expected1 := []Message{
		{Topic: "test/1", Payload: []byte{0x03}},
		{Topic: "test/2", Payload: []byte{0x04}},
	}
	expected2 := []Message{
		{Topic: "test2", Payload: []byte{0x01}},
	}

	if !reflect.DeepEqual(expected1, topic1) {
		t.Errorf("Expected: %v, \n    got: %v", expected1, topic1)
	}
	if !reflect.DeepEqual(expected2, topic2) {
		t.Errorf("Expected: %v, \n    got: %v", expected2, topic2)
	}
}
