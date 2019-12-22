package mqtt

import (
	"testing"
)

func TestNewTopicFilter(t *testing.T) {
	cases := []struct {
		filter string
		valid  bool
	}{
		// examples from MQTT 3.1.1 spec document
		{"", false},
		{"sport/#", true},
		{"#", true},
		{"sport/tennis/#", true},
		{"sport/tennis#", false},
		{"sport/tennis/#/ranking", false},
		{"+", true},
		{"+/tennis/#", true},
		{"sport+", false},
		{"sport/+/player1", true},
		{"+/+", true},
		{"/+", true},
	}
	for _, c := range cases {
		if _, err := newTopicFilter(c.filter); (err == nil) != c.valid {
			if c.valid {
				t.Errorf("'%s' should be valid", c.filter)
			} else {
				t.Errorf("'%s' should not be valid", c.filter)
			}
		}
	}
}

func TestTopicFilterMatch(t *testing.T) {
	cases := []struct {
		filter string
		topic  string
		match  bool
	}{
		// examples from MQTT 3.1.1 spec document
		{"sport/tennis/player1", "sport/tennis/player1", true},
		{"sport/tennis", "sport/tennis/player1", false},
		{"sport/tennis/player1", "sport/tennis", false},
		{"sport/tennis/player1/#", "sport/tennis/player1", true},
		{"sport/tennis/player1/#", "sport/tennis/player1/ranking", true},
		{"sport/tennis/player1/#", "sport/tennis/player1/score/wimbledon", true},
		{"sport/#", "sport", true},
		{"#", "sport", true},
		{"#", "sport/tennis", true},
		{"sport/tennis/+", "sport/tennis/player1", true},
		{"sport/tennis/+", "sport/tennis/player2", true},
		{"sport/tennis/+", "sport/tennis/player/2", false},
		{"sport/+", "sport", false},
		{"sport/+", "sport/", true},
		{"+/+", "/finance", true},
		{"/+", "/finance", true},
		{"+", "/finance", false},
	}
	for _, c := range cases {
		f, err := newTopicFilter(c.filter)
		if err != nil {
			t.Fatalf("Failed to validate '%s': %v", c.filter, err)
		}
		if f.Match(c.topic) != c.match {
			if c.match {
				t.Errorf("'%s' should match '%s'", c.filter, c.topic)
			} else {
				t.Errorf("'%s' should not match '%s'", c.filter, c.topic)
			}
		}
	}
}
