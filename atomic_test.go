// Copyright 2021 The mqtt-go authors.
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
	"errors"
	"testing"
)

func TestFirstError(t *testing.T) {
	var fe firstError

	if err := fe.Load(); err != nil {
		t.Fatalf("Initial value must be 'nil', got '%v'", err)
	}

	errDummy0 := errors.New("dummy1")
	errDummy1 := errors.New("dummy1")

	fe.Store(errDummy0)

	if err := fe.Load(); err != errDummy0 {
		t.Fatalf("Expected '%v', got '%v'", errDummy0, err)
	}

	fe.Store(errDummy1)

	if err := fe.Load(); err != errDummy0 {
		t.Fatalf("Value is updated after first store. Expected '%v', got '%v'", errDummy0, err)
	}
}
