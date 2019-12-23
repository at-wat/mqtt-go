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
func NewReconnectClient(ctx context.Context, dialer Dialer, clientID string, opts ...ConnectOption) Client {
	rc := &RetryClient{}
	cli := &reconnectClient{
		Client: rc,
	}
	done := make(chan struct{})
	var doneOnce sync.Once
	go func() {
		clean := true
		reconnWaitBase := 50 * time.Millisecond
		reconnWaitMax := 10 * time.Second
		reconnWait := reconnWaitBase
		for {
			if c, err := dialer.Dial(); err == nil {
				optsCurr := append([]ConnectOption{}, opts...)
				optsCurr = append(optsCurr, WithCleanSession(clean))
				clean = false               // Clean only first time.
				reconnWait = reconnWaitBase // Reset reconnect wait.
				rc.SetClient(ctx, c)

				if present, err := rc.Connect(ctx, clientID, optsCurr...); err == nil {
					doneOnce.Do(func() { close(done) })
					if present {
						rc.Resubscribe(ctx)
					}
					// Start keep alive.
					go func() {
						_ = KeepAlive(ctx, c, time.Second, time.Second)
					}()
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
	}
	return cli
}
