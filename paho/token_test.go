package mqtt

import (
	"testing"
	"time"
)

func TestToken(t *testing.T) {
	token := newToken()

	done := make(chan struct{})
	go func() {
		token.Wait()
		close(done)
	}()

	if token.WaitTimeout(time.Millisecond) {
		t.Errorf("Unreleased token should be timed out.")
	}
	select {
	case <-done:
		t.Errorf("Wait of unreleased token should not be returned.")
	case <-time.After(10 * time.Millisecond):
	}

	token.release()

	if !token.WaitTimeout(time.Millisecond) {
		t.Errorf("Released token should not be timed out.")
	}
	select {
	case <-done:
	case <-time.After(10 * time.Millisecond):
		t.Errorf("Wait of released token should be returned.")
	}
}
