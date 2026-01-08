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

// slackMessage represents a Slack webhook message.
type slackMessage struct {
	Username        string            `json:"username,omitempty"`
	IconEmoji       string            `json:"icon_emoji,omitempty"`
	IconURL         string            `json:"icon_url,omitempty"`
	Channel         string            `json:"channel,omitempty"`
	ThreadTimestamp string            `json:"thread_ts,omitempty"`
	Text            string            `json:"text,omitempty"`
	Attachments     []slackAttachment `json:"attachments,omitempty"`
	Parse           string            `json:"parse,omitempty"`
	ResponseType    string            `json:"response_type,omitempty"`
	ReplaceOriginal bool              `json:"replace_original,omitempty"`
	DeleteOriginal  bool              `json:"delete_original,omitempty"`
	ReplyBroadcast  bool              `json:"reply_broadcast,omitempty"`
}

// slackAttachment represents a Slack message attachment.
type slackAttachment struct {
	Color    string `json:"color,omitempty"`
	Fallback string `json:"fallback,omitempty"`

	CallbackID string `json:"callback_id,omitempty"`
	ID         int    `json:"id,omitempty"`

	AuthorID      string `json:"author_id,omitempty"`
	AuthorName    string `json:"author_name,omitempty"`
	AuthorSubname string `json:"author_subname,omitempty"`
	AuthorLink    string `json:"author_link,omitempty"`
	AuthorIcon    string `json:"author_icon,omitempty"`

	Title     string `json:"title,omitempty"`
	TitleLink string `json:"title_link,omitempty"`
	Pretext   string `json:"pretext,omitempty"`
	Text      string `json:"text,omitempty"`

	ImageURL string `json:"image_url,omitempty"`
	ThumbURL string `json:"thumb_url,omitempty"`

	ServiceName string `json:"service_name,omitempty"`
	ServiceIcon string `json:"service_icon,omitempty"`
	FromURL     string `json:"from_url,omitempty"`
	OriginalURL string `json:"original_url,omitempty"`

	Fields     []slackField `json:"fields,omitempty"`
	MarkdownIn []string     `json:"mrkdwn_in,omitempty"`

	Footer     string `json:"footer,omitempty"`
	FooterIcon string `json:"footer_icon,omitempty"`

	Actions []slackAction `json:"actions,omitempty"`
}

// slackField represents a field in a Slack attachment.
type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// slackAction represents an action button in a Slack attachment.
type slackAction struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	URL   string `json:"url"`
	Style string `json:"style"`
}

func (a *SlackAlerter) Send(ctx context.Context, message AlertMessage) error {
	text := a.formatMessage(message)
	msg := slackMessage{
		Attachments: []slackAttachment{
			{
				Text:       text,
				Color:      "good",
				AuthorName: "Reloader",
			},
		},
	}

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
