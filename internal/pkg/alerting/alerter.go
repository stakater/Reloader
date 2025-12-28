package alerting

import (
	"context"
	"time"

	"github.com/stakater/Reloader/internal/pkg/config"
)

// AlertMessage contains the details of a reload event to be sent as an alert.
type AlertMessage struct {
	WorkloadKind      string
	WorkloadName      string
	WorkloadNamespace string
	ResourceKind      string
	ResourceName      string
	ResourceNamespace string
	Timestamp         time.Time
}

// Alerter is the interface for sending reload notifications.
type Alerter interface {
	Send(ctx context.Context, message AlertMessage) error
}

// NewAlerter creates an Alerter based on the configuration.
// Returns a NoOpAlerter if alerting is disabled.
func NewAlerter(cfg *config.Config) Alerter {
	alertCfg := cfg.Alerting
	if !alertCfg.Enabled || alertCfg.WebhookURL == "" {
		return &NoOpAlerter{}
	}

	switch alertCfg.Sink {
	case "slack":
		return NewSlackAlerter(alertCfg.WebhookURL, alertCfg.Proxy, alertCfg.Additional)
	case "teams":
		return NewTeamsAlerter(alertCfg.WebhookURL, alertCfg.Proxy, alertCfg.Additional)
	case "gchat":
		return NewGChatAlerter(alertCfg.WebhookURL, alertCfg.Proxy, alertCfg.Additional)
	default:
		return NewRawAlerter(alertCfg.WebhookURL, alertCfg.Proxy, alertCfg.Additional, alertCfg.Structured)
	}
}

// NoOpAlerter is an Alerter that does nothing.
type NoOpAlerter struct{}

func (a *NoOpAlerter) Send(ctx context.Context, message AlertMessage) error {
	return nil
}
