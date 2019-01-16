package constants

// Result is a status for deployment update
type Result int

const (
	// Updated is returned when environment variable is created/updated
	Updated Result = 1 + iota
	// NotUpdated is returned when environment variable is found but had value equals to the new value
	NotUpdated
	// NoEnvFound is returned when no environment variable is found
	NoEnvFound
	// UpdateFailed is returned when environment variable is found but failed to update
	UpdateFailed
)
