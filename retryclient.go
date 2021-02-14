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
	"errors"
	"sync"
)

// ErrClosedClient means operation was requested on closed client.
var ErrClosedClient = errors.New("operation on closed client")

// RetryClient queues unacknowledged messages and retry on reconnect.
type RetryClient struct {
	cli ClientCloser

	retryQueue     []retryFn
	subEstablished subscriptions // acknoledged subscriptions
	mu             sync.Mutex
	handler        Handler
	chTask         chan struct{}
	taskQueue      []func(ctx context.Context, cli Client)
}

// Handle registers the message handler.
func (c *RetryClient) Handle(handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
	if c.cli != nil {
		c.cli.Handle(handler)
	}
}

// Publish tries to publish the message and immediately returns.
// If it is not acknowledged to be published, the message will be queued.
func (c *RetryClient) Publish(ctx context.Context, message *Message) error {
	c.mu.Lock()
	cli := c.cli
	c.mu.Unlock()

	if cli, ok := cli.(*BaseClient); ok {
		if err := cli.ValidateMessage(message); err != nil {
			return wrapError(err, "validating publishing message")
		}
	}
	return wrapError(c.pushTask(ctx, func(ctx context.Context, cli Client) {
		c.publish(ctx, false, cli, message)
	}), "retryclient: publishing")
}

// Subscribe tries to subscribe the topic and immediately return nil.
// If it is not acknowledged to be subscribed, the request will be queued.
func (c *RetryClient) Subscribe(ctx context.Context, subs ...Subscription) error {
	return wrapError(c.pushTask(ctx, func(ctx context.Context, cli Client) {
		c.subscribe(ctx, false, cli, subs...)
	}), "retryclient: subscribing")
}

// Unsubscribe tries to unsubscribe the topic and immediately return nil.
// If it is not acknowledged to be unsubscribed, the request will be queued.
func (c *RetryClient) Unsubscribe(ctx context.Context, topics ...string) error {
	return wrapError(c.pushTask(ctx, func(ctx context.Context, cli Client) {
		c.unsubscribe(ctx, false, cli, topics...)
	}), "retryclient: unsubscribing")
}

func (c *RetryClient) publish(ctx context.Context, retry bool, cli Client, message *Message) {
	publish := func(ctx context.Context, cli Client, message *Message) {
		if err := cli.Publish(ctx, message); err != nil {
			select {
			case <-ctx.Done():
				if !retry {
					// User cancelled; don't queue.
					return
				}
			default:
			}
			if retryErr, ok := err.(ErrorWithRetry); ok {
				c.retryQueue = append(c.retryQueue, retryErr.retry)
			}
		}
	}

	if len(c.retryQueue) == 0 {
		publish(ctx, cli, message)
		return
	}

	if message.QoS > QoS0 {
		copyMsg := *message
		c.retryQueue = append(c.retryQueue, func(ctx context.Context, cli *BaseClient) error {
			publish(ctx, cli, &copyMsg)
			return nil
		})
	}
	return
}

func (c *RetryClient) subscribe(ctx context.Context, retry bool, cli Client, subs ...Subscription) {
	subscribe := func(ctx context.Context, cli Client) {
		subscriptions(subs).applyTo(&c.subEstablished)
		if err := cli.Subscribe(ctx, subs...); err != nil {
			select {
			case <-ctx.Done():
				if !retry {
					// User cancelled; don't queue.
					return
				}
			default:
			}
			if retryErr, ok := err.(ErrorWithRetry); ok {
				c.retryQueue = append(c.retryQueue, retryErr.retry)
			}
		}
	}

	if len(c.retryQueue) == 0 {
		subscribe(ctx, cli)
		return
	}

	c.retryQueue = append(c.retryQueue, func(ctx context.Context, cli *BaseClient) error {
		subscribe(ctx, cli)
		return nil
	})
}

func (c *RetryClient) unsubscribe(ctx context.Context, retry bool, cli Client, topics ...string) {
	unsubscribe := func(ctx context.Context, cli Client) {
		unsubscriptions(topics).applyTo(&c.subEstablished)
		if err := cli.Unsubscribe(ctx, topics...); err != nil {
			select {
			case <-ctx.Done():
				if !retry {
					// User cancelled; don't queue.
					return
				}
			default:
			}
			if retryErr, ok := err.(ErrorWithRetry); ok {
				c.retryQueue = append(c.retryQueue, retryErr.retry)
			}
		}
	}

	if len(c.retryQueue) == 0 {
		unsubscribe(ctx, cli)
	}

	c.retryQueue = append(c.retryQueue, func(ctx context.Context, cli *BaseClient) error {
		unsubscribe(ctx, cli)
		return nil
	})
}

// Disconnect from the broker.
func (c *RetryClient) Disconnect(ctx context.Context) error {
	return wrapError(c.pushTask(ctx, func(ctx context.Context, cli Client) {
		cli.Disconnect(ctx)
	}), "retryclient: disconnecting")
}

// Ping to the broker.
func (c *RetryClient) Ping(ctx context.Context) error {
	c.mu.Lock()
	cli := c.cli
	c.mu.Unlock()
	return wrapError(cli.Ping(ctx), "retryclient: pinging")
}

// Client returns the base client.
func (c *RetryClient) Client() ClientCloser {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cli
}

// SetClient sets the new Client.
// Call Retry() and Resubscribe() to process queued messages and subscriptions.
func (c *RetryClient) SetClient(ctx context.Context, cli ClientCloser) {
	c.mu.Lock()
	c.cli = cli
	c.mu.Unlock()

	if c.chTask != nil {
		return
	}

	c.chTask = make(chan struct{}, 1)
	go func() {
		ctx := context.Background()
		for {
			c.mu.Lock()
			if len(c.taskQueue) == 0 {
				c.mu.Unlock()
				_, ok := <-c.chTask
				if !ok {
					return
				}
				continue
			}
			task := c.taskQueue[0]
			c.taskQueue = c.taskQueue[1:]
			cli := c.cli
			c.mu.Unlock()

			task(ctx, cli)
		}
	}()
}

func (c *RetryClient) pushTask(ctx context.Context, task func(ctx context.Context, cli Client)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case _, ok := <-c.chTask:
		if !ok {
			return ErrClosedClient
		}
	default:
	}

	c.taskQueue = append(c.taskQueue, task)
	select {
	case c.chTask <- struct{}{}:
	default:
	}
	return nil
}

// Connect to the broker.
func (c *RetryClient) Connect(ctx context.Context, clientID string, opts ...ConnectOption) (sessionPresent bool, err error) {
	c.mu.Lock()
	cli := c.cli
	cli.Handle(c.handler)
	c.mu.Unlock()

	present, err := cli.Connect(ctx, clientID, opts...)

	return present, wrapError(err, "retryclient: connecting")
}

// Resubscribe subscribes all established subscriptions.
func (c *RetryClient) Resubscribe(ctx context.Context) {
	c.pushTask(ctx, func(ctx context.Context, cli Client) {
		oldSubEstablished := append([]Subscription{}, c.subEstablished...)
		c.subEstablished = nil

		if len(oldSubEstablished) > 0 {
			for _, sub := range oldSubEstablished {
				c.subscribe(ctx, true, cli, sub)
			}
		}
	})
}

// Retry all queued publish/subscribe requests.
// Underlaying Client must be *BaseClient to retry.
func (c *RetryClient) Retry(ctx context.Context) {
	c.pushTask(ctx, func(ctx context.Context, cli Client) {
		oldRetryQueue := append([]retryFn{}, c.retryQueue...)
		c.retryQueue = nil

		baseCli := c.cli.(*BaseClient)

		for _, retry := range oldRetryQueue {
			retry(ctx, baseCli)
		}
	})
}
