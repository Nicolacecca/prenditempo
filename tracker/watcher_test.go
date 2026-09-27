package tracker

import (
	"testing"
	"time"
)

func TestWatcherStopIsNonBlocking(t *testing.T) {
	watcher := NewTimeWatcher()
	watcher.Start(3600)
	done := make(chan struct{})
	go func() {
		watcher.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop è rimasto bloccato")
	}
}
