package mqtt

import (
	"reflect"
	"testing"
)

func TestServeMux_HandleFunc(t *testing.T) {
	mux := &ServeMux{}

	if err := mux.HandleFunc("", func(*Message) {}); err != ErrInvalidTopicFilter {
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
