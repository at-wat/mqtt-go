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
	"reflect"
	"testing"
)

func TestAppendRemoveEstablished(t *testing.T) {
	c := &RetryClient{}

	c.appendEstablished(
		Subscription{Topic: "t1", QoS: QoS1},
		Subscription{Topic: "t2", QoS: QoS2},
		Subscription{Topic: "t3", QoS: QoS0},
	)

	expected1 := []Subscription{
		Subscription{Topic: "t1", QoS: QoS1},
		Subscription{Topic: "t2", QoS: QoS2},
		Subscription{Topic: "t3", QoS: QoS0},
	}
	if !reflect.DeepEqual(c.subEstablished, expected1) {
		t.Errorf("Expected established topic list:\n%v\ngot:\n%v", expected1, c.subEstablished)
	}

	c.removeEstablished("t1", "t3")

	expected2 := []Subscription{
		Subscription{Topic: "t2", QoS: QoS2},
	}
	if !reflect.DeepEqual(c.subEstablished, expected2) {
		t.Errorf("Expected established topic list:\n%v\ngot:\n%v", expected2, c.subEstablished)
	}
}
