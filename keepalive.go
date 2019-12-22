package mqtt

import (
	"context"
	"errors"
	"time"
)

// ErrKeepAliveDisabled is returned if Runned on keep alive disabled connection.
var ErrKeepAliveDisabled = errors.New("keep alive disabled")

// ErrPingTimeout is returned on ping response timeout.
var ErrPingTimeout = errors.New("ping timeout")

// KeepAlive runs keep alive loop.
// It must be called after Connect.
func KeepAlive(ctx context.Context, cli *BaseClient, timeout time.Duration) error {
	if cli.connectOpts.KeepAlive == 0 {
		return ErrKeepAliveDisabled
	}
	ticker := time.NewTicker(time.Duration(cli.connectOpts.KeepAlive) * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		ctxTo, cancel := context.WithTimeout(ctx, timeout)
		if err := cli.Ping(ctxTo); err != nil {
			defer cancel()
			// The client should close the connection if PINGRESP is not returned.
			// MQTT 3.1.1 spec. 3.1.2.10
			cli.Close()

			select {
			case <-ctx.Done():
				// Parent context cancelled.
				return ctx.Err()
			default:
			}
			select {
			case <-ctxTo.Done():
				return ErrPingTimeout
			default:
			}
			return err
		}
		cancel()
	}
}
