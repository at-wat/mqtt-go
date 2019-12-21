// +build integration

package mqtt

import (
	"context"
	"testing"
)

func TestIntegration_Connect(t *testing.T) {
	cli := &Client{}
	if err := cli.Dial("mqtt://localhost:1883"); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
	go cli.Serve()

	ctx := context.Background()
	ack, err := cli.Connect(ctx, "Client1")
	if err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}

	if ack.SessionPresent {
		t.Errorf("SessionPresent flag was set to non CleanSession connection")
	}
	if ack.Code != ConnectionAccepted {
		t.Errorf("Connection was not accepted: %s", ack.Code)
	}

	if err := cli.Disconnect(ctx); err != nil {
		t.Fatalf("Unexpected error: '%v'", err)
	}
}
