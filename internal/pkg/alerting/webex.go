package alerting

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// WebexAlerter sends alerts to Webex incoming webhooks.
type WebexAlerter struct {
	webhookURL string
	additional string
	client     *httpClient
}

// NewWebexAlerter creates a new WebexAlerter.
func NewWebexAlerter(webhookURL, proxyURL, additional string) *WebexAlerter {
	return &WebexAlerter{
		webhookURL: webhookURL,
		additional: additional,
		client:     newHTTPClient(proxyURL),
	}
}

// webexMessage represents a Webex incoming webhook message.
// The webhook accepts either a "text" or a "markdown" key, not both.
type webexMessage struct {
	Markdown string `json:"markdown"`
}

func (a *WebexAlerter) Send(ctx context.Context, message AlertMessage) error {
	msg := webexMessage{Markdown: a.formatMessage(message)}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling webex message: %w", err)
	}

	return a.client.post(ctx, a.webhookURL, body)
}

func (a *WebexAlerter) formatMessage(msg AlertMessage) string {
	text := fmt.Sprintf(
		"**Reloader triggered reload**\n\n"+
			"**Workload:** %s/%s (%s)\n\n"+
			"**Resource:** %s/%s (%s)\n\n"+
			"**Time:** %s",
		msg.WorkloadNamespace, msg.WorkloadName, msg.WorkloadKind,
		msg.ResourceNamespace, msg.ResourceName, msg.ResourceKind,
		msg.Timestamp.Format("2006-01-02 15:04:05 UTC"),
	)

	if a.additional != "" {
		text = escapeWebexMarkdown(a.additional) + "\n\n" + text
	}

	return text
}

// webexMarkdownEscaper backslash-escapes the characters that start bold,
// italic, code, link, heading, quote or mention formatting in Webex markdown.
var webexMarkdownEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	"*", `\*`,
	"_", `\_`,
	"[", `\[`,
	"]", `\]`,
	"<", `\<`,
	">", `\>`,
	"#", `\#`,
)

// escapeWebexMarkdown makes user supplied text render literally in a Webex
// markdown message. Kubernetes object names cannot contain these characters,
// so only free text such as ALERT_ADDITIONAL_INFO needs it.
func escapeWebexMarkdown(s string) string {
	return webexMarkdownEscaper.Replace(s)
}
