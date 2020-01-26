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
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDialOptionError(t *testing.T) {
	errExpected := errors.New("an error")
	optErr := func() DialOption {
		return func(o *DialOptions) error {
			return errExpected
		}
	}

	if _, err := Dial("mqtt://localhost:1883", optErr()); err != errExpected {
		t.Errorf("Expected error: '%v', got: '%v'", errExpected, err)
	}
}

func TestDialOption_WithDialer(t *testing.T) {
	if _, err := Dial("mqtt://localhost:1884",
		WithDialer(&net.Dialer{Timeout: time.Nanosecond}),
	); !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: '%v'", err)
	}
}
