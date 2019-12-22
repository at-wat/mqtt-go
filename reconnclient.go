package mqtt

import (
	"context"
	"sync"
	"time"
)

type reconnectClient struct {
	Client
	Dialer Dialer
}

// NewReconnectClient creates a MQTT client with re-connect/re-publish/re-subscribe features.
func NewReconnectClient(ctx context.Context, dialer Dialer, clientID string, opts ...ConnectOption) Client {
	rc := &RetryClient{}
	cli := &reconnectClient{
		Client: rc,
		Dialer: dialer,
	}
	done := make(chan struct{})
	var doneOnce sync.Once
	go func() {
		clean := true
		for {
			if c, err := dialer.Dial(); err == nil {
				rc.SetClient(ctx, c)

				optsCurr := append([]ConnectOption{}, opts...)
				optsCurr = append(optsCurr, WithCleanSession(clean))
				clean = false // Clean only first time.

				if present, err := rc.Connect(ctx, clientID, optsCurr...); err == nil {
					doneOnce.Do(func() { close(done) })
					if present {
						// Do resubscribe here.
					}
					// Start keep alive.
					go func() {
						_ = KeepAlive(ctx, c, time.Second, time.Second)
					}()
					select {
					case <-c.Done():
						if err := c.Err(); err == nil {
							return
						}
					case <-ctx.Done():
						return
					}
				}
			}
			time.Sleep(time.Second)
		}
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
	return cli
}
