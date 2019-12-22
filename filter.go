package mqtt

import (
	"errors"
	"strings"
)

// ErrInvalidTopicFilter means that the topic filter string is invalid.
var ErrInvalidTopicFilter = errors.New("invalid topic filter")

type topicFilter []string

func newTopicFilter(filter string) (topicFilter, error) {
	if len(filter) == 0 {
		return nil, ErrInvalidTopicFilter
	}
	tf := strings.Split(filter, "/")

	// Validate according to MQTT 3.1.1 spec. 4.7.1
	for i, f := range tf {
		if strings.Contains(f, "+") {
			if len(f) != 1 {
				return nil, ErrInvalidTopicFilter
			}
		}
		if strings.Contains(f, "#") {
			if len(f) != 1 || i != len(tf)-1 {
				return nil, ErrInvalidTopicFilter
			}
		}
	}
	return tf, nil
}

func (f topicFilter) Match(topic string) bool {
	ts := strings.Split(topic, "/")

	var i int
	for i = 0; i < len(f); i++ {
		t := f[i]
		if t == "#" {
			return true
		}
		if i >= len(ts) {
			return false
		}
		if t != "+" && t != ts[i] {
			return false
		}
	}
	return i == len(ts)
}
