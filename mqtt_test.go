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

// Package mqtt is a thread safe and context controlled MQTT 3.1.1 client library.
package mqtt

import (
	"testing"
)

func TestConnState(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		if s := StateNew.String(); s != "New" {
			t.Errorf("StateNew.String() should be 'New' but got '%s'", s)
		}
	})
	t.Run("Invalid", func(t *testing.T) {
		if s := ConnState(-1).String(); s != "Unknown" {
			t.Errorf("StateNew.String() for invalid state should be 'Unknown' but got '%s'", s)
		}
	})
}
