package alerting

import (
	"context"
	"encoding/json"
	"fmt"
)

// TeamsAlerter sends alerts to Microsoft Teams webhooks.
type TeamsAlerter struct {
	webhookURL string
	additional string
	client     *httpClient
}

// NewTeamsAlerter creates a new TeamsAlerter.
func NewTeamsAlerter(webhookURL, proxyURL, additional string) *TeamsAlerter {
	return &TeamsAlerter{
		webhookURL: webhookURL,
		additional: additional,
		client:     newHTTPClient(proxyURL),
	}
}

// teamsMessage represents a Microsoft Teams message card.
type teamsMessage struct {
	Type       string         `json:"@type"`
	Context    string         `json:"@context"`
	ThemeColor string         `json:"themeColor"`
	Summary    string         `json:"summary"`
	Sections   []teamsSection `json:"sections"`
}

type teamsSection struct {
	ActivityTitle    string      `json:"activityTitle"`
	ActivitySubtitle string      `json:"activitySubtitle,omitempty"`
	Facts            []teamsFact `json:"facts"`
}

type teamsFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (a *TeamsAlerter) Send(ctx context.Context, message AlertMessage) error {
	msg := a.buildMessage(message)

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling teams message: %w", err)
	}

	return a.client.post(ctx, a.webhookURL, body)
}

func (a *TeamsAlerter) buildMessage(msg AlertMessage) teamsMessage {
	facts := []teamsFact{
		{Name: "Workload", Value: fmt.Sprintf("%s/%s (%s)", msg.WorkloadNamespace, msg.WorkloadName, msg.WorkloadKind)},
		{Name: "Resource", Value: fmt.Sprintf("%s/%s (%s)", msg.ResourceNamespace, msg.ResourceName, msg.ResourceKind)},
		{Name: "Time", Value: msg.Timestamp.Format("2006-01-02 15:04:05 UTC")},
	}

	subtitle := ""
	if a.additional != "" {
		subtitle = a.additional
	}

	return teamsMessage{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: "0076D7",
		Summary:    "Reloader triggered reload",
		Sections: []teamsSection{
			{
				ActivityTitle:    "Reloader triggered reload",
				ActivitySubtitle: subtitle,
				Facts:            facts,
			},
		},
	}
}
