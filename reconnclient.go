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
	"context"
	"sync"
	"time"
)

type reconnectClient struct {
	Client
}

// NewReconnectClient creates a MQTT client with re-connect/re-publish/re-subscribe features.
func NewReconnectClient(ctx context.Context, dialer Dialer, clientID string, opts ...ConnectOption) (Client, error) {
	rc := &RetryClient{}

	options := &ConnectOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	done := make(chan struct{})
	var doneOnce sync.Once
	go func() {
		clean := options.CleanSession
		reconnWaitBase := 100 * time.Millisecond
		reconnWaitMax := 10 * time.Second
		reconnWait := reconnWaitBase
		for {
			if c, err := dialer.Dial(); err == nil {
				optsCurr := append([]ConnectOption{}, opts...)
				optsCurr = append(optsCurr, WithCleanSession(clean))
				clean = false // Clean only first time.
				rc.SetClient(ctx, c)

				if present, err := rc.Connect(ctx, clientID, optsCurr...); err == nil {
					reconnWait = reconnWaitBase // Reset reconnect wait.
					doneOnce.Do(func() { close(done) })
					if present {
						rc.Resubscribe(ctx)
					}
					if options.KeepAlive > 0 {
						// Start keep alive.
						go func() {
							timeout := time.Duration(options.KeepAlive) * time.Second / 2
							if timeout < time.Second {
								timeout = time.Second
							}
							_ = KeepAlive(
								ctx, c,
								time.Duration(options.KeepAlive)*time.Second,
								timeout,
							)
						}()
					}
					select {
					case <-c.Done():
						if err := c.Err(); err == nil {
							// Disconnected as expected; don't restart.
							return
						}
					case <-ctx.Done():
						// User cancelled; don't restart.
						return
					}
				}
			}
			select {
			case <-time.After(reconnWait):
			case <-ctx.Done():
				// User cancelled; don't restart.
				return
			}
			reconnWait *= 2
			if reconnWait > reconnWaitMax {
				reconnWait = reconnWaitMax
			}
		}
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return rc, nil
}
