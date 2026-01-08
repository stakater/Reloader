package alerting

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stakater/Reloader/internal/pkg/config"
)

// testServer creates a test HTTP server that captures the request body.
// Returns the server and a function to retrieve the captured body.
func testServer(t *testing.T, expectedContentType string) (*httptest.Server, func() []byte) {
	t.Helper()
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != expectedContentType {
			t.Errorf("Expected Content-Type %s, got %s", expectedContentType, r.Header.Get("Content-Type"))
		}
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	return server, func() []byte { return body }
}

// testAlertMessage returns a standard AlertMessage for testing.
func testAlertMessage() AlertMessage {
	return AlertMessage{
		WorkloadKind:      "Deployment",
		WorkloadName:      "nginx",
		WorkloadNamespace: "default",
		ResourceKind:      "ConfigMap",
		ResourceName:      "nginx-config",
		ResourceNamespace: "default",
		Timestamp:         time.Now(),
	}
}

func TestNewAlerter(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*config.Config)
		wantType string
	}{
		{
			name: "disabled",
			setup: func(cfg *config.Config) {
				cfg.Alerting.Enabled = false
			},
			wantType: "*alerting.NoOpAlerter",
		},
		{
			name: "no webhook URL",
			setup: func(cfg *config.Config) {
				cfg.Alerting.Enabled = true
				cfg.Alerting.WebhookURL = ""
			},
			wantType: "*alerting.NoOpAlerter",
		},
		{
			name: "slack",
			setup: func(cfg *config.Config) {
				cfg.Alerting.Enabled = true
				cfg.Alerting.WebhookURL = "http://example.com/webhook"
				cfg.Alerting.Sink = "slack"
			},
			wantType: "*alerting.SlackAlerter",
		},
		{
			name: "teams",
			setup: func(cfg *config.Config) {
				cfg.Alerting.Enabled = true
				cfg.Alerting.WebhookURL = "http://example.com/webhook"
				cfg.Alerting.Sink = "teams"
			},
			wantType: "*alerting.TeamsAlerter",
		},
		{
			name: "gchat",
			setup: func(cfg *config.Config) {
				cfg.Alerting.Enabled = true
				cfg.Alerting.WebhookURL = "http://example.com/webhook"
				cfg.Alerting.Sink = "gchat"
			},
			wantType: "*alerting.GChatAlerter",
		},
		{
			name: "raw",
			setup: func(cfg *config.Config) {
				cfg.Alerting.Enabled = true
				cfg.Alerting.WebhookURL = "http://example.com/webhook"
				cfg.Alerting.Sink = "raw"
			},
			wantType: "*alerting.RawAlerter",
		},
		{
			name: "empty sink defaults to raw",
			setup: func(cfg *config.Config) {
				cfg.Alerting.Enabled = true
				cfg.Alerting.WebhookURL = "http://example.com/webhook"
				cfg.Alerting.Sink = ""
			},
			wantType: "*alerting.RawAlerter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefault()
			tt.setup(cfg)
			alerter := NewAlerter(cfg)
			gotType := getTypeName(alerter)
			if gotType != tt.wantType {
				t.Errorf("NewAlerter() type = %s, want %s", gotType, tt.wantType)
			}
		})
	}
}

func getTypeName(a Alerter) string {
	switch a.(type) {
	case *NoOpAlerter:
		return "*alerting.NoOpAlerter"
	case *SlackAlerter:
		return "*alerting.SlackAlerter"
	case *TeamsAlerter:
		return "*alerting.TeamsAlerter"
	case *GChatAlerter:
		return "*alerting.GChatAlerter"
	case *RawAlerter:
		return "*alerting.RawAlerter"
	default:
		return "unknown"
	}
}

func TestNoOpAlerter_Send(t *testing.T) {
	alerter := &NoOpAlerter{}
	if err := alerter.Send(context.Background(), AlertMessage{}); err != nil {
		t.Errorf("NoOpAlerter.Send() error = %v, want nil", err)
	}
}

func TestAlerter_Send(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		newAlert    func(url string) Alerter
		validate    func(t *testing.T, body []byte)
	}{
		{
			name:        "slack",
			contentType: "application/json",
			newAlert:    func(url string) Alerter { return NewSlackAlerter(url, "", "Test Cluster") },
			validate: func(t *testing.T, body []byte) {
				var msg slackMessage
				if err := json.Unmarshal(body, &msg); err != nil {
					t.Fatalf("Failed to unmarshal: %v", err)
				}
				if len(msg.Attachments) != 1 {
					t.Fatalf("Expected 1 attachment, got %d", len(msg.Attachments))
				}
				if msg.Attachments[0].Text == "" {
					t.Error("Expected non-empty attachment text")
				}
				if msg.Attachments[0].Color != "good" {
					t.Errorf("Expected color 'good', got %s", msg.Attachments[0].Color)
				}
				if msg.Attachments[0].AuthorName != "Reloader" {
					t.Errorf("Expected author_name 'Reloader', got %s", msg.Attachments[0].AuthorName)
				}
			},
		},
		{
			name:        "teams",
			contentType: "application/json",
			newAlert:    func(url string) Alerter { return NewTeamsAlerter(url, "", "") },
			validate: func(t *testing.T, body []byte) {
				var msg teamsMessage
				if err := json.Unmarshal(body, &msg); err != nil {
					t.Fatalf("Failed to unmarshal: %v", err)
				}
				if msg.Type != "MessageCard" {
					t.Errorf("@type = %s, want MessageCard", msg.Type)
				}
			},
		},
		{
			name:        "gchat",
			contentType: "application/json",
			newAlert:    func(url string) Alerter { return NewGChatAlerter(url, "", "") },
			validate: func(t *testing.T, body []byte) {
				var msg gchatMessage
				if err := json.Unmarshal(body, &msg); err != nil {
					t.Fatalf("Failed to unmarshal: %v", err)
				}
				if len(msg.Cards) != 1 {
					t.Errorf("cards = %d, want 1", len(msg.Cards))
				}
			},
		},
		{
			name:        "raw plain text (default)",
			contentType: "text/plain",
			newAlert:    func(url string) Alerter { return NewRawAlerter(url, "", "custom-info", false) },
			validate: func(t *testing.T, body []byte) {
				text := string(body)
				if text == "" {
					t.Error("Expected non-empty text")
				}
				if !strings.Contains(text, "custom-info") {
					t.Error("Expected text to contain 'custom-info'")
				}
				if !strings.Contains(text, "nginx") {
					t.Error("Expected text to contain workload name 'nginx'")
				}
			},
		},
		{
			name:        "raw structured JSON",
			contentType: "application/json",
			newAlert:    func(url string) Alerter { return NewRawAlerter(url, "", "custom-info", true) },
			validate: func(t *testing.T, body []byte) {
				var msg rawMessage
				if err := json.Unmarshal(body, &msg); err != nil {
					t.Fatalf("Failed to unmarshal: %v", err)
				}
				if msg.Event != "reload" {
					t.Errorf("event = %s, want reload", msg.Event)
				}
				if msg.WorkloadName != "nginx" {
					t.Errorf("workloadName = %s, want nginx", msg.WorkloadName)
				}
				if msg.Additional != "custom-info" {
					t.Errorf("additional = %s, want custom-info", msg.Additional)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, getBody := testServer(t, tt.contentType)
			defer server.Close()

			alerter := tt.newAlert(server.URL)
			if err := alerter.Send(context.Background(), testAlertMessage()); err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			tt.validate(t, getBody())
		})
	}
}

func TestAlerter_WebhookError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	alerter := NewRawAlerter(server.URL, "", "", false)
	if err := alerter.Send(context.Background(), AlertMessage{}); err == nil {
		t.Error("Expected error for non-2xx response")
	}
}
