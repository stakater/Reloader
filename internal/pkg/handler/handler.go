package handler

import "github.com/stakater/Reloader/pkg/common"

// ResourceHandler handles the creation and update of resources
type ResourceHandler interface {
	Handle() error
	GetConfig() (common.Config, string)
}
