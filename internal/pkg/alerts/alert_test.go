package alert

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

// TestSendWebhookAlert_LogsSendErrors is a regression test for #949: a failing
// webhook (here, a non-2xx response) previously produced no output at all
// because the errors returned by the send functions were discarded. They must
// now be surfaced as error logs.
func TestSendWebhookAlert_LogsSendErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	hook := logrustest.NewGlobal()
	defer hook.Reset()

	t.Setenv("ALERT_WEBHOOK_URL", server.URL)
	t.Setenv("ALERT_SINK", string(AlertSinkTeams))

	SendWebhookAlert("test message")

	var logged bool
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.ErrorLevel && strings.Contains(entry.Message, "Error sending alert") {
			logged = true
			break
		}
	}
	assert.True(t, logged, "expected the swallowed webhook error to be logged")
}

// TestSendTeamsAlert_AdaptiveCardPayload is a regression test for #1304: Teams
// Workflows (Power Automate) webhooks reject the old payload with "Property
// 'type' must be 'AdaptiveCard'". The alert must be sent as a message with an
// Adaptive Card attachment, which both Workflows and the legacy incoming
// webhooks accept. Workflows answers with 202 Accepted, so that must not be
// reported as an error.
func TestSendTeamsAlert_AdaptiveCardPayload(t *testing.T) {
	var got map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Content-Type"), "application/json")
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	errs := sendTeamsAlert(server.URL, "", "Changes detected in 'my-config' of type 'CONFIGMAP' in namespace 'default'")
	assert.Empty(t, errs)

	assert.Equal(t, "message", got["type"])
	attachments, ok := got["attachments"].([]interface{})
	if !assert.True(t, ok, "attachments missing: %v", got) || !assert.Len(t, attachments, 1) {
		return
	}
	attachment := attachments[0].(map[string]interface{})
	assert.Equal(t, "application/vnd.microsoft.card.adaptive", attachment["contentType"])

	card, ok := attachment["content"].(map[string]interface{})
	if !assert.True(t, ok, "card content missing: %v", attachment) {
		return
	}
	assert.Equal(t, "AdaptiveCard", card["type"])
	assert.NotEmpty(t, card["version"])

	body, ok := card["body"].([]interface{})
	if !assert.True(t, ok) || !assert.Len(t, body, 1) {
		return
	}
	block := body[0].(map[string]interface{})
	assert.Equal(t, "TextBlock", block["type"])
	assert.Equal(t, "Changes detected in 'my-config' of type 'CONFIGMAP' in namespace 'default'", block["text"])
	assert.Equal(t, true, block["wrap"])
}
