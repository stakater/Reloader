package alerting

import (
	"context"
	"encoding/json"
	"fmt"
)

// RawAlerter sends alerts as raw JSON to a webhook.
type RawAlerter struct {
	webhookURL string
	additional string
	client     *httpClient
}

// NewRawAlerter creates a new RawAlerter.
func NewRawAlerter(webhookURL, proxyURL, additional string) *RawAlerter {
	return &RawAlerter{
		webhookURL: webhookURL,
		additional: additional,
		client:     newHTTPClient(proxyURL),
	}
}

// rawMessage is the JSON payload for raw webhook alerts.
type rawMessage struct {
	Event             string `json:"event"`
	WorkloadKind      string `json:"workloadKind"`
	WorkloadName      string `json:"workloadName"`
	WorkloadNamespace string `json:"workloadNamespace"`
	ResourceKind      string `json:"resourceKind"`
	ResourceName      string `json:"resourceName"`
	ResourceNamespace string `json:"resourceNamespace"`
	Timestamp         string `json:"timestamp"`
	Additional        string `json:"additional,omitempty"`
}

func (a *RawAlerter) Send(ctx context.Context, message AlertMessage) error {
	msg := rawMessage{
		Event:             "reload",
		WorkloadKind:      message.WorkloadKind,
		WorkloadName:      message.WorkloadName,
		WorkloadNamespace: message.WorkloadNamespace,
		ResourceKind:      message.ResourceKind,
		ResourceName:      message.ResourceName,
		ResourceNamespace: message.ResourceNamespace,
		Timestamp:         message.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		Additional:        a.additional,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling raw message: %w", err)
	}

	return a.client.post(ctx, a.webhookURL, body)
}
