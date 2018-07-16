package handler

// ResourceHandler handles the creation and update of resources
type ResourceHandler interface {
	Handle() error
}
