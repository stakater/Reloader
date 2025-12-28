package alerting

import (
	"context"
	"encoding/json"
	"fmt"
)

// GChatAlerter sends alerts to Google Chat webhooks.
type GChatAlerter struct {
	webhookURL string
	additional string
	client     *httpClient
}

// NewGChatAlerter creates a new GChatAlerter.
func NewGChatAlerter(webhookURL, proxyURL, additional string) *GChatAlerter {
	return &GChatAlerter{
		webhookURL: webhookURL,
		additional: additional,
		client:     newHTTPClient(proxyURL),
	}
}

// gchatMessage represents a Google Chat message.
type gchatMessage struct {
	Text  string      `json:"text,omitempty"`
	Cards []gchatCard `json:"cards,omitempty"`
}

type gchatCard struct {
	Header   gchatHeader    `json:"header"`
	Sections []gchatSection `json:"sections"`
}

type gchatHeader struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

type gchatSection struct {
	Widgets []gchatWidget `json:"widgets"`
}

type gchatWidget struct {
	KeyValue *gchatKeyValue `json:"keyValue,omitempty"`
}

type gchatKeyValue struct {
	TopLabel string `json:"topLabel"`
	Content  string `json:"content"`
}

func (a *GChatAlerter) Send(ctx context.Context, message AlertMessage) error {
	msg := a.buildMessage(message)

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling gchat message: %w", err)
	}

	return a.client.post(ctx, a.webhookURL, body)
}

func (a *GChatAlerter) buildMessage(msg AlertMessage) gchatMessage {
	widgets := []gchatWidget{
		{KeyValue: &gchatKeyValue{TopLabel: "Workload", Content: fmt.Sprintf("%s/%s (%s)", msg.WorkloadNamespace, msg.WorkloadName, msg.WorkloadKind)}},
		{KeyValue: &gchatKeyValue{TopLabel: "Resource", Content: fmt.Sprintf("%s/%s (%s)", msg.ResourceNamespace, msg.ResourceName, msg.ResourceKind)}},
		{KeyValue: &gchatKeyValue{TopLabel: "Time", Content: msg.Timestamp.Format("2006-01-02 15:04:05 UTC")}},
	}

	subtitle := ""
	if a.additional != "" {
		subtitle = a.additional
	}

	return gchatMessage{
		Cards: []gchatCard{
			{
				Header: gchatHeader{
					Title:    "Reloader triggered reload",
					Subtitle: subtitle,
				},
				Sections: []gchatSection{
					{Widgets: widgets},
				},
			},
		},
	}
}
