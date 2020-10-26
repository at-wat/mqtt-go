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
	*RetryClient
	done         chan struct{}
	options      *ReconnectOptions
	dialer       Dialer
	disconnected chan struct{}
}

// NewReconnectClient creates a MQTT client with re-connect/re-publish/re-subscribe features.
func NewReconnectClient(dialer Dialer, opts ...ReconnectOption) (Client, error) {
	options := &ReconnectOptions{
		ReconnectWaitBase: time.Second,
		ReconnectWaitMax:  10 * time.Second,
	}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}
	return &reconnectClient{
		RetryClient:  &RetryClient{},
		done:         make(chan struct{}),
		disconnected: make(chan struct{}),
		options:      options,
		dialer:       dialer,
	}, nil
}

// Connect starts connection retry loop.
// The function returns after establishing a first connection, which can be canceled by the context.
// Once after establising the connection, the retry loop is not affected by the context.
func (c *reconnectClient) Connect(ctx context.Context, clientID string, opts ...ConnectOption) (bool, error) {
	connOptions := &ConnectOptions{
		CleanSession: true,
	}
	for _, opt := range opts {
		if err := opt(connOptions); err != nil {
			return false, err
		}
	}
	if c.options.PingInterval == time.Duration(0) {
		c.options.PingInterval = time.Duration(connOptions.KeepAlive) * time.Second
	}
	if c.options.Timeout == time.Duration(0) {
		c.options.Timeout = c.options.PingInterval
	}

	done := make(chan struct{})
	var doneOnce sync.Once
	var sessionPresent bool
	go func(ctx context.Context) {
		defer func() {
			close(c.done)
		}()
		clean := connOptions.CleanSession
		reconnWait := c.options.ReconnectWaitBase
		for {
			if baseCli, err := c.dialer.Dial(); err == nil {
				optsCurr := append([]ConnectOption{}, opts...)
				optsCurr = append(optsCurr, WithCleanSession(clean))
				clean = false // Clean only first time.
				c.RetryClient.SetClient(ctx, baseCli)

				ctxTimeout, cancel := context.WithTimeout(ctx, c.options.Timeout)
				if sessionPresent, err := c.RetryClient.Connect(ctxTimeout, clientID, optsCurr...); err == nil {
					cancel()

					reconnWait = c.options.ReconnectWaitBase // Reset reconnect wait.
					doneOnce.Do(func() {
						ctx = context.Background()
						close(done)
					})

					if !sessionPresent {
						c.RetryClient.Resubscribe(ctx)
					}
					c.RetryClient.Retry(ctx)

					if c.options.PingInterval > time.Duration(0) {
						// Start keep alive.
						go func() {
							if err := KeepAlive(
								ctx, baseCli,
								c.options.PingInterval,
								c.options.Timeout,
							); err != nil {
								if cli, ok := c.cli.(*BaseClient); ok {
									cli.SetErrorOnce(err)
								}
							}
						}()
					}
					select {
					case <-baseCli.Done():
						if err := baseCli.Err(); err == nil {
							// Disconnected as expected; don't restart.
							return
						}
					case <-ctx.Done():
						// User cancelled; don't restart.
						return
					case <-c.disconnected:
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
			case <-c.disconnected:
				return
			}
			reconnWait *= 2
			if reconnWait > c.options.ReconnectWaitMax {
				reconnWait = c.options.ReconnectWaitMax
			}
		}
	}(ctx)
	select {
	case <-done:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	return sessionPresent, nil
}

// Disconnect from the broker.
func (c *reconnectClient) Disconnect(ctx context.Context) error {
	close(c.disconnected)
	err := c.RetryClient.Disconnect(ctx)
	select {
	case <-c.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return err
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
