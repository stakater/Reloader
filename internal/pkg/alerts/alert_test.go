package alert

import (
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
