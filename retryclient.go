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

	pubQueue       []*Message    // unacknoledged messages
	subQueue       []subTask     // unacknoledged subscriptions
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

type subTask interface {
	applyTo(*subscriptions)
}

type subscriptions []Subscription

func (s subscriptions) applyTo(d *subscriptions) {
	*d = append(*d, s...)
}

type unsubscriptions []string

func (s unsubscriptions) applyTo(d *subscriptions) {
	l := len(*d)
	for _, topic := range s {
		for i, e := range *d {
			if e.Topic == topic {
				l--
				(*d)[i] = (*d)[l]
				break
			}
		}
	}
	*d = (*d)[:l]
}

// Publish tries to publish the message and immediately returns.
// If it is not acknowledged to be published, the message will be queued.
func (c *RetryClient) Publish(ctx context.Context, message *Message) error {
	if cli, ok := c.cli.(*BaseClient); ok {
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
	if len(c.pubQueue) == 0 {
		if err := cli.Publish(ctx, message); err != nil {
			select {
			case <-ctx.Done():
				if !retry {
					// User cancelled; don't queue.
					return
				}
			default:
			}
		} else {
			return
		}
	}
	if message.QoS > QoS0 {
		copyMsg := *message

		copyMsg.Dup = true
		c.pubQueue = append(c.pubQueue, &copyMsg)
	}
}

func (c *RetryClient) subscribe(ctx context.Context, retry bool, cli Client, subs ...Subscription) {
	if err := cli.Subscribe(ctx, subs...); err != nil {
		select {
		case <-ctx.Done():
			if !retry {
				// User cancelled; don't queue.
				return
			}
		default:
		}
		c.subQueue = append(c.subQueue, subscriptions(subs))
	} else {
		subscriptions(subs).applyTo(&c.subEstablished)
	}
}

func (c *RetryClient) unsubscribe(ctx context.Context, retry bool, cli Client, topics ...string) {
	if err := cli.Unsubscribe(ctx, topics...); err != nil {
		select {
		case <-ctx.Done():
			if !retry {
				// User cancelled; don't queue.
				return
			}
		default:
		}
		c.subQueue = append(c.subQueue, unsubscriptions(topics))
	} else {
		unsubscriptions(topics).applyTo(&c.subEstablished)
	}
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
	defer c.mu.Unlock()
	c.cli = cli

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
func (c *RetryClient) Retry(ctx context.Context) {
	c.pushTask(ctx, func(ctx context.Context, cli Client) {
		oldPubQueue := append([]*Message{}, c.pubQueue...)
		oldSubQueue := append([]subTask{}, c.subQueue...)
		c.pubQueue = nil
		c.subQueue = nil

		// Retry publish.
		for _, sub := range oldSubQueue {
			switch s := sub.(type) {
			case subscriptions:
				c.subscribe(ctx, true, cli, s...)
			case unsubscriptions:
				c.unsubscribe(ctx, true, cli, s...)
			}
		}
		for _, msg := range oldPubQueue {
			c.publish(ctx, true, cli, msg)
		}
	})
}
