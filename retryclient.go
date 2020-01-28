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
)

// RetryClient queues unacknowledged messages and retry on reconnect.
type RetryClient struct {
	Client

	pubQueue       []*Message       // unacknoledged messages
	subQueue       [][]Subscription // unacknoledged subscriptions
	unsubQueue     [][]string       // unacknoledged unsubscriptions
	subEstablished []Subscription   // acknoledged subscriptions
	mu             sync.Mutex
	muQueue        sync.Mutex
	handler        Handler
}

// Handle registers the message handler.
func (c *RetryClient) Handle(handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
	if c.Client != nil {
		c.Client.Handle(handler)
	}
}

// Publish tries to publish the message and immediately return nil.
// If it is not acknowledged to be published, the message will be queued.
func (c *RetryClient) Publish(ctx context.Context, message *Message) error {
	go func() {
		c.mu.Lock()
		cli := c.Client
		c.mu.Unlock()
		c.publish(ctx, false, cli, message)
	}()
	return nil
}

// Subscribe tries to subscribe the topic and immediately return nil.
// If it is not acknowledged to be subscribed, the request will be queued.
func (c *RetryClient) Subscribe(ctx context.Context, subs ...Subscription) error {
	go func() {
		c.mu.Lock()
		cli := c.Client
		c.mu.Unlock()
		c.subscribe(ctx, false, cli, subs...)
	}()
	return nil
}

// Unsubscribe tries to unsubscribe the topic and immediately return nil.
// If it is not acknowledged to be unsubscribed, the request will be queued.
func (c *RetryClient) Unsubscribe(ctx context.Context, topics ...string) error {
	go func() {
		c.mu.Lock()
		cli := c.Client
		c.mu.Unlock()
		c.unsubscribe(ctx, false, cli, topics...)
	}()
	return nil
}

func (c *RetryClient) publish(ctx context.Context, retry bool, cli Client, message *Message) {
	if err := cli.Publish(ctx, message); err != nil {
		select {
		case <-ctx.Done():
			if !retry {
				// User cancelled; don't queue.
				return
			}
		default:
		}
		if message.QoS > QoS0 {
			copyMsg := *message

			c.muQueue.Lock()
			copyMsg.Dup = true
			c.pubQueue = append(c.pubQueue, &copyMsg)
			c.muQueue.Unlock()
		}
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
		c.muQueue.Lock()
		c.subQueue = append(c.subQueue, subs)
		c.muQueue.Unlock()
	} else {
		c.appendEstablished(subs...)
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
		c.muQueue.Lock()
		c.unsubQueue = append(c.unsubQueue, topics)
		c.muQueue.Unlock()
	} else {
		c.removeEstablished(topics...)
	}
}

func (c *RetryClient) appendEstablished(subs ...Subscription) {
	c.muQueue.Lock()
	c.subEstablished = append(c.subEstablished, subs...)
	c.muQueue.Unlock()
}

func (c *RetryClient) removeEstablished(topics ...string) {
	c.muQueue.Lock()
	l := len(c.subEstablished)
	for _, topic := range topics {
		for i, e := range c.subEstablished {
			if e.Topic == topic {
				l--
				c.subEstablished[i] = c.subEstablished[l]
				break
			}
		}
	}
	c.subEstablished = c.subEstablished[:l]
	c.muQueue.Unlock()
}

// Disconnect from the broker.
func (c *RetryClient) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	cli := c.Client
	c.mu.Unlock()
	return cli.Disconnect(ctx)
}

// Ping to the broker.
func (c *RetryClient) Ping(ctx context.Context) error {
	c.mu.Lock()
	cli := c.Client
	c.mu.Unlock()
	return cli.Ping(ctx)
}

// SetClient sets the new Client.
// Call Retry() and Resubscribe() to process queued messages and subscriptions.
func (c *RetryClient) SetClient(ctx context.Context, cli Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Client = cli
}

// Connect to the broker.
func (c *RetryClient) Connect(ctx context.Context, clientID string, opts ...ConnectOption) (sessionPresent bool, err error) {
	c.mu.Lock()
	cli := c.Client
	cli.Handle(c.handler)
	c.mu.Unlock()

	present, err := cli.Connect(ctx, clientID, opts...)

	return present, err
}

// Resubscribe subscribes all established subscriptions.
func (c *RetryClient) Resubscribe(ctx context.Context) {
	c.muQueue.Lock()
	oldSubEstablished := append([]Subscription{}, c.subEstablished...)
	c.subEstablished = nil
	c.muQueue.Unlock()

	if len(oldSubEstablished) > 0 {
		c.mu.Lock()
		cli := c.Client
		c.mu.Unlock()
		for _, sub := range oldSubEstablished {
			c.subscribe(ctx, true, cli, sub)
		}
	}
}

// Retry all queued publish/subscribe requests.
func (c *RetryClient) Retry(ctx context.Context) {
	c.mu.Lock()
	cli := c.Client
	c.mu.Unlock()

	c.muQueue.Lock()
	oldPubQueue := append([]*Message{}, c.pubQueue...)
	oldSubQueue := append([][]Subscription{}, c.subQueue...)
	oldUnsubQueue := append([][]string{}, c.unsubQueue...)
	c.pubQueue = nil
	c.subQueue = nil
	c.unsubQueue = nil
	c.muQueue.Unlock()

	// Retry publish.
	go func() {
		for _, sub := range oldSubQueue {
			c.subscribe(ctx, true, cli, sub...)
		}
		for _, unsub := range oldUnsubQueue {
			c.unsubscribe(ctx, true, cli, unsub...)
		}
		for _, msg := range oldPubQueue {
			c.publish(ctx, true, cli, msg)
		}
	}()
}
