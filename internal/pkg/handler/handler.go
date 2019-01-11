package handler

import (
	"github.com/stakater/Reloader/internal/pkg/util"
)

// ResourceHandler handles the creation and update of resources
type ResourceHandler interface {
	Handle() error
	GetConfig() (util.Config, string, string)
}
