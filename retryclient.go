package mqtt

import (
	"context"
	"sync"
)

// RetryClient queues unacknowledged messages and retry on reconnect.
type RetryClient struct {
	Client

	pubQueue       []*Message     // unacknoledged messages
	subQueue       []Subscription // unacknoledged subscriptions
	subEstablished []Subscription // acknoledged subscriptions
	mu             sync.Mutex
	muQueue        sync.Mutex
	handler        Handler
}

// Handle registers the message handler.
func (c *RetryClient) Handle(handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
	c.Client.Handle(handler)
}

// Publish tries to publish the message and immediately return nil.
// If it is not acknowledged to be published, the message will be queued and
// retried on the next connection.
func (c *RetryClient) Publish(ctx context.Context, message *Message) error {
	go func() {
		c.mu.Lock()
		cli := c.Client
		c.mu.Unlock()
		c.publish(ctx, cli, message)
	}()
	return nil
}

// Subscribe tries to subscribe the topic and immediately return nil.
// If it is not acknowledged to be subscribed, the request will be queued and
// retried on the next connection.
func (c *RetryClient) Subscribe(ctx context.Context, subs ...Subscription) error {
	go func() {
		c.mu.Lock()
		cli := c.Client
		c.mu.Unlock()
		c.subscribe(ctx, cli, subs...)
	}()
	return nil
}

func (c *RetryClient) publish(ctx context.Context, cli Client, message *Message) {
	if err := cli.Publish(ctx, message); err != nil {
		select {
		case <-ctx.Done():
			// User cancelled; don't queue.
		default:
			if message.QoS > QoS0 {
				copyMsg := *message

				c.muQueue.Lock()
				copyMsg.Dup = true
				c.pubQueue = append(c.pubQueue, &copyMsg)
				c.muQueue.Unlock()
			}
		}
	}
}

func (c *RetryClient) subscribe(ctx context.Context, cli Client, subs ...Subscription) {
	if err := cli.Subscribe(ctx, subs...); err != nil {
		select {
		case <-ctx.Done():
			// User cancelled; don't queue.
		default:
			c.muQueue.Lock()
			c.subQueue = append(c.subQueue, subs...)
			c.muQueue.Unlock()
		}
	} else {
		c.muQueue.Lock()
		c.subEstablished = append(c.subEstablished, subs...)
		c.muQueue.Unlock()
	}
}

// SetClient sets the new Client.
// If there are any queued messages, retry to publish them.
func (c *RetryClient) SetClient(ctx context.Context, cli Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Client = cli
}

// Connect to the broker.
func (c *RetryClient) Connect(ctx context.Context, clientID string, opts ...ConnectOption) (sessionPresent bool, err error) {
	c.muQueue.Lock()
	oldPubQueue := append([]*Message{}, c.pubQueue...)
	oldSubQueue := append([]Subscription{}, c.subQueue...)
	c.pubQueue = nil
	c.subQueue = nil
	c.muQueue.Unlock()

	c.mu.Lock()
	cli := c.Client
	cli.Handle(c.handler)
	c.mu.Unlock()

	present, err := cli.Connect(ctx, clientID, opts...)

	// Retry publish.
	go func() {
		if len(oldSubQueue) > 0 {
			c.subscribe(ctx, cli, oldSubQueue...)
		}
		for _, msg := range oldPubQueue {
			c.publish(ctx, cli, msg)
		}
	}()

	return present, err
}

// Resubscribe subscribes all established subscriptions.
func (c *RetryClient) Resubscribe(ctx context.Context) error {
	if len(c.subEstablished) > 0 {
		c.mu.Lock()
		cli := c.Client
		c.mu.Unlock()
		c.subscribe(ctx, cli, c.subEstablished...)
	}
	return nil
}
