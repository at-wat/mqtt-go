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
	"errors"
	"regexp"
	"testing"
)

type dummyError struct {
	Err error
}

func (e *dummyError) Error() string {
	return e.Err.Error()
}

func TestError(t *testing.T) {
	errBase := errors.New("an error")
	errOther := errors.New("an another error")
	errChained := wrapErrorf(errBase, "info")
	t.Log(`wrapErrorf(errBase, "info"):`, errChained)
	errChained2 := wrapError(errBase, "info")
	t.Log(`wrapError(errBase, "info"):`, errChained2)
	errDoubleChained := wrapErrorf(errChained, "info")
	t.Log(`wrapErrorf(errChained, "info"):`, errDoubleChained)
	errChainedNil := wrapErrorf(nil, "info")
	errChainedOther := wrapErrorf(errOther, "info")
	err112Chained := wrapErrorf(&dummyError{errBase}, "info")
	err112Nil := wrapErrorf(&dummyError{nil}, "info")
	errStrRegex := regexp.MustCompile(`^info \[error_test\.go:[0-9]+\]: an error$`)

	t.Run("ErrorsIs", func(t *testing.T) {
		if !errors.Is(errChained, errBase) {
			t.Errorf("Wrapped error '%v' doesn't chain '%v'", errChained, errBase)
		}
		if !errors.Is(errChained2, errBase) {
			t.Errorf("Wrapped error '%v' doesn't chain '%v'", errChained2, errBase)
		}
	})

	t.Run("Is", func(t *testing.T) {
		if !errChained.(*Error).Is(errChained) {
			t.Errorf("Wrapped error '%v' doesn't match with its-self", errChained)
		}
		if !errChained.(*Error).Is(errBase) {
			t.Errorf("Wrapped error '%v' doesn't match with '%v'", errChained, errBase)
		}
		if !errChained2.(*Error).Is(errChained2) {
			t.Errorf("Wrapped error '%v' doesn't match with its-self", errChained2)
		}
		if !errChained2.(*Error).Is(errBase) {
			t.Errorf("Wrapped error '%v' doesn't match with '%v'", errChained2, errBase)
		}
		if !errDoubleChained.(*Error).Is(errBase) {
			t.Errorf("Wrapped error '%v' doesn't match with '%v'", errDoubleChained, errBase)
		}
		if !err112Chained.(*Error).Is(errBase) {
			t.Errorf("Wrapped error '%v' doesn't match with '%v'",
				err112Chained, errBase)
		}
		if !(&Error{}).Is(nil) {
			t.Errorf("Wrapped nil error doesn't match with 'nil'")
		}
		if errChainedNil != nil {
			t.Errorf("Nil chained error '%v' doesn't match with 'nil'", errChainedNil)
		}

		if errChainedOther.(*Error).Is(errBase) {
			t.Errorf("Wrapped error '%v' unexpectedly matched with '%v'",
				errChainedOther, errBase)
		}
		if err112Nil.(*Error).Is(errBase) {
			t.Errorf("Wrapped error '%v' unexpectedly matched with '%v'",
				errChainedOther, errBase)
		}
		if errChained.(*Error).Is(nil) {
			t.Errorf("Wrapped error '%v' unexpectedly matched with 'nil'",
				errChained)
		}
		if (&Error{}).Is(errBase) {
			t.Errorf("Wrapped nil error unexpectedly matched with '%v'",
				errBase)
		}
	})

	if !errStrRegex.MatchString(errChained.Error()) {
		t.Errorf("Error string expected regexp: '%s', got: '%s'", errStrRegex, errChained.Error())
	}
	if errChained.(*Error).Unwrap() != errBase {
		t.Errorf("Unwrapped error expected: %s, got: %s", errBase, errChained.(*Error).Unwrap())
	}
	if !errStrRegex.MatchString(errChained2.Error()) {
		t.Errorf("Error string expected regexp: '%s', got: '%s'", errStrRegex, errChained2.Error())
	}
	if errChained2.(*Error).Unwrap() != errBase {
		t.Errorf("Unwrapped error expected: %s, got: %s", errBase, errChained2.(*Error).Unwrap())
	}
}
