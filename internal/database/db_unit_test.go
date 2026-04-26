package database

import (
	"context"
	"testing"
	"time"
)

func TestMonitorConnectionPoolStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := (&DB{}).MonitorConnectionPool(ctx)

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("MonitorConnectionPool did not stop after context cancellation")
	}
}
