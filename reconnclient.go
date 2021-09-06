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
	"fmt"
	"strings"
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
// Once after establishing the connection, the retry loop is not affected by the context.
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

	var errDial, errConnect firstError

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
			if baseCli, err := c.dialer.DialContext(ctx); err == nil {
				optsCurr := append([]ConnectOption{}, opts...)
				optsCurr = append(optsCurr, WithCleanSession(clean))
				clean = false // Clean only first time.
				c.RetryClient.SetClient(ctx, baseCli)

				var ctxTimeout context.Context
				var cancel func()
				if c.options.Timeout == 0 {
					ctxTimeout, cancel = ctx, func() {}
				} else {
					ctxTimeout, cancel = context.WithTimeout(ctx, c.options.Timeout)
				}

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
								c.Client().SetErrorOnce(err)
								// The client should close the connection if PINGRESP is not returned.
								// MQTT 3.1.1 spec. 3.1.2.10
								baseCli.Close()
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
				} else if err != ctxTimeout.Err() {
					errConnect.Store(err) // Hold last connect error but avoid overwrote by context cancel.
				}
				cancel()
			} else if err != ctx.Err() {
				errDial.Store(err) // Hold last dial error but avoid overwrote by context cancel.
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
		var actualErrs []string
		if err := errDial.Load(); err != nil {
			actualErrs = append(actualErrs, fmt.Sprintf("dial: %v", err))
		}
		if err := errConnect.Load(); err != nil {
			actualErrs = append(actualErrs, fmt.Sprintf("connect: %v", err))
		}
		var errStr string
		if len(actualErrs) > 0 {
			errStr = fmt.Sprintf(" (%s)", strings.Join(actualErrs, ", "))
		}
		return false, wrapErrorf(ctx.Err(), "establishing first connection%s", errStr)
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
		return wrapError(ctx.Err(), "disconnecting")
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
