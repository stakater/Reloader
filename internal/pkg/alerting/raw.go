package alerting

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RawAlerter sends alerts to a webhook as plain text (default) or structured JSON.
type RawAlerter struct {
	webhookURL string
	additional string
	structured bool
	client     *httpClient
}

// NewRawAlerter creates a new RawAlerter.
// If structured is true, sends JSON; otherwise sends plain text.
func NewRawAlerter(webhookURL, proxyURL, additional string, structured bool) *RawAlerter {
	return &RawAlerter{
		webhookURL: webhookURL,
		additional: additional,
		structured: structured,
		client:     newHTTPClient(proxyURL),
	}
}

// rawMessage is the JSON payload for structured raw webhook alerts.
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
	if a.structured {
		return a.sendStructured(ctx, message)
	}
	return a.sendPlainText(ctx, message)
}

func (a *RawAlerter) sendStructured(ctx context.Context, message AlertMessage) error {
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

func (a *RawAlerter) sendPlainText(ctx context.Context, message AlertMessage) error {
	text := a.formatMessage(message)
	// Strip markdown formatting for plain text
	text = strings.ReplaceAll(text, "*", "")
	return a.client.postText(ctx, a.webhookURL, text)
}

func (a *RawAlerter) formatMessage(msg AlertMessage) string {
	text := fmt.Sprintf(
		"Reloader triggered reload - Workload: %s/%s (%s), Resource: %s/%s (%s), Time: %s",
		msg.WorkloadNamespace, msg.WorkloadName, msg.WorkloadKind,
		msg.ResourceNamespace, msg.ResourceName, msg.ResourceKind,
		msg.Timestamp.Format("2006-01-02 15:04:05 UTC"),
	)

	if a.additional != "" {
		text = a.additional + " : " + text
	}

	return text
}
