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
	"time"
)

func TestToken(t *testing.T) {
	token := newToken()

	done := make(chan struct{})
	go func() {
		token.Wait()
		close(done)
	}()

	if token.WaitTimeout(time.Millisecond) {
		t.Errorf("Unreleased token should be timed out.")
	}
	select {
	case <-done:
		t.Errorf("Wait of unreleased token should not be returned.")
	case <-time.After(10 * time.Millisecond):
	}

	token.release()

	if !token.WaitTimeout(time.Millisecond) {
		t.Errorf("Released token should not be timed out.")
	}
	select {
	case <-done:
	case <-time.After(10 * time.Millisecond):
		t.Errorf("Wait of released token should be returned.")
	}
}
