package mqtt

import (
	"context"
	"sync"
)

// RetryClient queues unacknowledged messages and retry on reconnect.
type RetryClient struct {
	Client
	queue []*Message // unacknoledged messages
	mu    sync.Mutex
	muMsg sync.Mutex
}

// Publish tries to publish the message and immediately return nil.
// If it is not acknowledged to be published, the message will be queued and
// retried on the next connection.
func (c *RetryClient) Publish(ctx context.Context, message *Message) error {
	go func() {
		c.mu.Lock()
		cli := c.Client
		c.publish(ctx, cli, message)
		c.mu.Unlock()
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

				c.muMsg.Lock()
				copyMsg.Dup = true
				c.queue = append(c.queue, &copyMsg)
				c.muMsg.Unlock()
			}
		}
	}
}

// SetClient sets the new Client.
// If there are any queued messages, retry to publish them.
func (c *RetryClient) SetClient(ctx context.Context, cli Client) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Client = cli

	c.muMsg.Lock()
	oldQueue := append([]*Message{}, c.queue...)
	c.queue = nil
	c.muMsg.Unlock()

	// Retry publish.
	go func() {
		for _, msg := range oldQueue {
			c.publish(ctx, cli, msg)
		}
	}()

	return nil
}
