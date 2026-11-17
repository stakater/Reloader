package reload

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"

	"github.com/stakater/Reloader/pkg/config"
)

// ThrottleHandler limits concurrent reload patches and spreads rollouts across
// dependents in one namespace for a single config trigger.
type ThrottleHandler struct {
	cfg *config.Config
	log logr.Logger

	reloadSlots     chan struct{}
	reloadSlotsOnce sync.Once
	spreadCounters  sync.Map
}

// NewThrottleHandler creates a ThrottleHandler.
func NewThrottleHandler(cfg *config.Config, log logr.Logger) *ThrottleHandler {
	return &ThrottleHandler{cfg: cfg, log: log}
}

func (h *ThrottleHandler) initReloadSlots() {
	h.reloadSlotsOnce.Do(func() {
		if h.cfg.MaxConcurrentReloads > 0 {
			h.reloadSlots = make(chan struct{}, h.cfg.MaxConcurrentReloads)
		}
	})
}

// AcquireSlot blocks until a cluster-wide reload slot is available.
func (h *ThrottleHandler) AcquireSlot() {
	if h.cfg.MaxConcurrentReloads <= 0 {
		return
	}
	h.initReloadSlots()
	h.log.V(1).Info("waiting for reload slot", "maxConcurrent", h.cfg.MaxConcurrentReloads)
	h.reloadSlots <- struct{}{}
}

// ReleaseSlot frees a reload slot acquired with AcquireSlot.
func (h *ThrottleHandler) ReleaseSlot() {
	if h.cfg.MaxConcurrentReloads <= 0 || h.reloadSlots == nil {
		return
	}
	<-h.reloadSlots
}

// WaitConsumerSpread staggers reloads of workloads for one config change in a namespace.
func (h *ThrottleHandler) WaitConsumerSpread(
	resourceAnnotations map[string]string,
	namespace, resourceName, resourceType, hash string,
) {
	spreadValue := ""
	if resourceAnnotations != nil {
		spreadValue = resourceAnnotations[h.cfg.Annotations.ReloadSpread]
	}
	if spreadValue == "" {
		spreadValue = h.cfg.GlobalReloadSpread
	}
	if spreadValue == "" {
		return
	}

	spread, err := time.ParseDuration(spreadValue)
	if err != nil || spread <= 0 {
		h.log.Info(
			"invalid reload spread duration",
			"value", spreadValue,
			"namespace", namespace,
			"resource", resourceName,
			"error", err,
		)
		return
	}

	key := spreadCounterKey(namespace, resourceType, resourceName, hash)
	counter, _ := h.spreadCounters.LoadOrStore(key, new(int32))
	idx := atomic.AddInt32(counter.(*int32), 1) - 1
	if idx > 0 {
		delay := time.Duration(idx) * spread
		h.log.Info(
			"spreading reload for consumers",
			"resourceType", resourceType,
			"resource", resourceName,
			"namespace", namespace,
			"delay", delay.String(),
			"consumerIndex", idx+1,
		)
		time.Sleep(delay)
	}
}

// ResetSpread clears spread counters after all workloads were considered for one trigger.
func (h *ThrottleHandler) ResetSpread(namespace, resourceType, resourceName, hash string) {
	h.spreadCounters.Delete(spreadCounterKey(namespace, resourceType, resourceName, hash))
}

func spreadCounterKey(namespace, resourceType, resourceName, hash string) string {
	return fmt.Sprintf("%s/%s/%s/%s", namespace, resourceType, resourceName, hash)
}