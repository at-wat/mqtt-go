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

// NewReconnectClient creates a MQTT client with re-connect/re-publish/re-subscribe features.
func NewReconnectClient(ctx context.Context, dialer Dialer, clientID string, opts ...ReconnectOption) (Client, error) {
	rc := &RetryClient{}

	options := &ReconnectOptions{
		ReconnectWaitBase: time.Second,
		ReconnectWaitMax:  10 * time.Second,
	}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}
	connOptions := &ConnectOptions{
		CleanSession: true,
	}
	for _, opt := range options.ConnectOptions {
		if err := opt(connOptions); err != nil {
			return nil, err
		}
	}
	if options.PingInterval == time.Duration(0) {
		options.PingInterval = time.Duration(connOptions.KeepAlive) * time.Second
	}
	if options.Timeout == time.Duration(0) {
		options.Timeout = options.PingInterval
	}

	done := make(chan struct{})
	var doneOnce sync.Once
	go func() {
		clean := connOptions.CleanSession
		reconnWait := options.ReconnectWaitBase
		for {
			if c, err := dialer.Dial(); err == nil {
				optsCurr := append([]ConnectOption{}, options.ConnectOptions...)
				optsCurr = append(optsCurr, WithCleanSession(clean))
				clean = false // Clean only first time.
				rc.SetClient(ctx, c)

				ctxTimeout, cancel := context.WithTimeout(ctx, options.Timeout)
				if present, err := rc.Connect(ctxTimeout, clientID, optsCurr...); err == nil {
					cancel()
					reconnWait = options.ReconnectWaitBase // Reset reconnect wait.
					if !present {
						rc.Resubscribe(ctx)
					}
					doneOnce.Do(func() { close(done) })
					if options.PingInterval > time.Duration(0) {
						// Start keep alive.
						go func() {
							_ = KeepAlive(
								ctx, c,
								options.PingInterval,
								options.Timeout,
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
				cancel()
			}
			select {
			case <-time.After(reconnWait):
			case <-ctx.Done():
				// User cancelled; don't restart.
				return
			}
			reconnWait *= 2
			if reconnWait > options.ReconnectWaitMax {
				reconnWait = options.ReconnectWaitMax
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

// ReconnectOptions represents options for Connect.
type ReconnectOptions struct {
	ConnectOptions    []ConnectOption
	Timeout           time.Duration
	ReconnectWaitBase time.Duration
	ReconnectWaitMax  time.Duration
	PingInterval      time.Duration
}

// ReconnectOption sets option for Connect.
type ReconnectOption func(*ReconnectOptions) error

// WithConnectOption sets ConnectOption to ReconnectClient.
func WithConnectOption(connOpts ...ConnectOption) ReconnectOption {
	return func(o *ReconnectOptions) error {
		o.ConnectOptions = connOpts
		return nil
	}
}

// WithTimeout sets timeout duration of server response.
// Default value is PingInterval.
func WithTimeout(timeout time.Duration) ReconnectOption {
	return func(o *ReconnectOptions) error {
		o.Timeout = timeout
		return nil
	}
}

// WithReconnectWait sets parameters of incremental reconnect wait.
func WithReconnectWait(base, max time.Duration) ReconnectOption {
	return func(o *ReconnectOptions) error {
		o.ReconnectWaitBase = base
		o.ReconnectWaitMax = max
		return nil
	}
}

// WithPingInterval sets ping request interval.
// Default value is KeepAlive value set by ConnectOption.
func WithPingInterval(interval time.Duration) ReconnectOption {
	return func(o *ReconnectOptions) error {
		o.PingInterval = interval
		return nil
	}
}
