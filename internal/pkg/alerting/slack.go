package alerting

import (
	"context"
	"encoding/json"
	"fmt"
)

// SlackAlerter sends alerts to Slack webhooks.
type SlackAlerter struct {
	webhookURL string
	additional string
	client     *httpClient
}

// NewSlackAlerter creates a new SlackAlerter.
func NewSlackAlerter(webhookURL, proxyURL, additional string) *SlackAlerter {
	return &SlackAlerter{
		webhookURL: webhookURL,
		additional: additional,
		client:     newHTTPClient(proxyURL),
	}
}

type slackMessage struct {
	Text string `json:"text"`
}

func (a *SlackAlerter) Send(ctx context.Context, message AlertMessage) error {
	text := a.formatMessage(message)
	msg := slackMessage{Text: text}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling slack message: %w", err)
	}

	return a.client.post(ctx, a.webhookURL, body)
}

func (a *SlackAlerter) formatMessage(msg AlertMessage) string {
	text := fmt.Sprintf(
		"Reloader triggered reload\n"+
			"*Workload:* %s/%s (%s)\n"+
			"*Resource:* %s/%s (%s)\n"+
			"*Time:* %s",
		msg.WorkloadNamespace, msg.WorkloadName, msg.WorkloadKind,
		msg.ResourceNamespace, msg.ResourceName, msg.ResourceKind,
		msg.Timestamp.Format("2006-01-02 15:04:05 UTC"),
	)

	if a.additional != "" {
		text = a.additional + "\n" + text
	}

	return text
}
