package reload

import (
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"

	"github.com/stakater/Reloader/pkg/config"
)

func TestConsumerSpread(t *testing.T) {
	cfg := config.NewDefault()
	cfg.GlobalReloadSpread = ""
	h := NewThrottleHandler(cfg, logr.Discard())

	start := time.Now()
	h.WaitConsumerSpread(nil, "ns", "res", "Secret", "sha1")
	elapsed := time.Since(start)
	if elapsed > 20*time.Millisecond {
		t.Fatalf("expected no delay without spread, got %s", elapsed)
	}

	cfg.GlobalReloadSpread = "40ms"
	annotations := map[string]string{
		cfg.Annotations.ReloadSpread: "40ms",
	}

	var wg sync.WaitGroup
	staggerStart := time.Now()
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.WaitConsumerSpread(annotations, "ns", "res", "Secret", "sha1")
		}()
	}
	wg.Wait()
	if elapsed := time.Since(staggerStart); elapsed < 70*time.Millisecond {
		t.Fatalf("expected staggered delay, got %s", elapsed)
	}
	h.ResetSpread("ns", "Secret", "res", "sha1")
}

func TestAcquireReloadSlots(t *testing.T) {
	cfg := config.NewDefault()
	h := NewThrottleHandler(cfg, logr.Discard())

	cfg.MaxConcurrentReloads = 0
	h.AcquireSlot()
	h.ReleaseSlot()

	cfg.MaxConcurrentReloads = 1
	h2 := NewThrottleHandler(cfg, logr.Discard())
	done := make(chan struct{})
	go func() {
		h2.AcquireSlot()
		time.Sleep(30 * time.Millisecond)
		h2.ReleaseSlot()
		close(done)
	}()
	time.Sleep(5 * time.Millisecond)
	acquired := make(chan struct{})
	go func() {
		h2.AcquireSlot()
		close(acquired)
		h2.ReleaseSlot()
	}()
	select {
	case <-acquired:
		t.Fatal("second acquire should block until first releases")
	case <-time.After(10 * time.Millisecond):
	}
	<-done
	select {
	case <-acquired:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second acquire should proceed after release")
	}
}

func TestConsumerSpreadInvalidDuration(t *testing.T) {
	cfg := config.NewDefault()
	h := NewThrottleHandler(cfg, logr.Discard())
	annotations := map[string]string{
		cfg.Annotations.ReloadSpread: "not-a-duration",
	}
	start := time.Now()
	h.WaitConsumerSpread(annotations, "ns", "res", "Secret", "sha")
	if time.Since(start) > 10*time.Millisecond {
		t.Fatalf("invalid spread should not sleep")
	}
}