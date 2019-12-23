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
func NewReconnectClient(ctx context.Context, dialer Dialer, clientID string, opts ...ConnectOption) (Client, error) {
	rc := &RetryClient{}

	options := &ConnectOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	done := make(chan struct{})
	var doneOnce sync.Once
	go func() {
		clean := options.CleanSession
		reconnWaitBase := 100 * time.Millisecond
		reconnWaitMax := 10 * time.Second
		reconnWait := reconnWaitBase
		for {
			if c, err := dialer.Dial(); err == nil {
				optsCurr := append([]ConnectOption{}, opts...)
				optsCurr = append(optsCurr, WithCleanSession(clean))
				clean = false // Clean only first time.
				rc.SetClient(ctx, c)

				if present, err := rc.Connect(ctx, clientID, optsCurr...); err == nil {
					reconnWait = reconnWaitBase // Reset reconnect wait.
					doneOnce.Do(func() { close(done) })
					if present {
						rc.Resubscribe(ctx)
					}
					if options.KeepAlive > 0 {
						// Start keep alive.
						go func() {
							timeout := time.Duration(options.KeepAlive) * time.Second / 2
							if timeout < time.Second {
								timeout = time.Second
							}
							_ = KeepAlive(
								ctx, c,
								time.Duration(options.KeepAlive)*time.Second,
								timeout,
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
		return nil, ctx.Err()
	}
	return rc, nil
}
