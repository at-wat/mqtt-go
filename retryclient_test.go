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
	"context"
	"errors"
	"testing"
)

var _ Retryer = &RetryClient{} // RetryClient must implement Retryer.

func TestRetryClientPublish_MessageValidationError(t *testing.T) {
	cli := &RetryClient{
		cli: &BaseClient{
			MaxPayloadLen: 100,
		},
	}
	if err := cli.Publish(context.Background(), &Message{
		Payload: make([]byte, 101),
	}); !errors.Is(err, ErrPayloadLenExceeded) {
		t.Errorf("Publishing too large payload expected error: %v, got: %v",
			ErrPayloadLenExceeded, err,
		)
	}
}

func TestRetryClient_NilClient(t *testing.T) {
	cli := &RetryClient{}
	t.Run("Publish", func(t *testing.T) {
		if err := cli.Publish(context.Background(), &Message{
			Payload: make([]byte, 101),
		}); err != nil {
			t.Errorf("Unexpected error: '%v'", err)
		}
	})
	t.Run("Subscribe", func(t *testing.T) {
		if _, err := cli.Subscribe(context.Background(), Subscription{
			Topic: "test",
			QoS:   QoS1,
		}); err != nil {
			t.Errorf("Unexpected error: '%v'", err)
		}
	})
	t.Run("Unsubscribe", func(t *testing.T) {
		if err := cli.Unsubscribe(context.Background(), "test"); err != nil {
			t.Errorf("Unexpected error: '%v'", err)
		}
	})
}
