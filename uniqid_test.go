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
	"testing"
)

func TestBaseClient_IDIncrement(t *testing.T) {
	var id uint32 = 0xFFFF
	c := &BaseClient{idLast: &id}

	// 0 is reserved for auto, next to 0xFFFF is 1
	if id := c.newID(); id != 1 {
		t.Errorf("Expected newID: 1, got: %d", id)
	}

	if id := c.newID(); id != 2 {
		t.Errorf("Expected newID: 2, got: %d", id)
	}
}
