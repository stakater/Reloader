package handler

import (
	"time"

	"github.com/stakater/Reloader/pkg/common"
)

// ResourceHandler handles the creation and update of resources
type ResourceHandler interface {
	Handle() error
	GetConfig() (common.Config, string)
}

// TimedHandler is a handler that tracks when it was enqueued
type TimedHandler interface {
	GetEnqueueTime() time.Time
}
