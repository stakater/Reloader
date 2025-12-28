package alerting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stakater/Reloader/internal/pkg/config"
)

func TestNewAlerter_Disabled(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Alerting.Enabled = false

	alerter := NewAlerter(cfg)
	if _, ok := alerter.(*NoOpAlerter); !ok {
		t.Error("Expected NoOpAlerter when alerting is disabled")
	}
}

func TestNewAlerter_NoWebhookURL(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Alerting.Enabled = true
	cfg.Alerting.WebhookURL = ""

	alerter := NewAlerter(cfg)
	if _, ok := alerter.(*NoOpAlerter); !ok {
		t.Error("Expected NoOpAlerter when webhook URL is empty")
	}
}

func TestNewAlerter_Slack(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Alerting.Enabled = true
	cfg.Alerting.WebhookURL = "http://example.com/webhook"
	cfg.Alerting.Sink = "slack"

	alerter := NewAlerter(cfg)
	if _, ok := alerter.(*SlackAlerter); !ok {
		t.Error("Expected SlackAlerter for sink=slack")
	}
}

func TestNewAlerter_Teams(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Alerting.Enabled = true
	cfg.Alerting.WebhookURL = "http://example.com/webhook"
	cfg.Alerting.Sink = "teams"

	alerter := NewAlerter(cfg)
	if _, ok := alerter.(*TeamsAlerter); !ok {
		t.Error("Expected TeamsAlerter for sink=teams")
	}
}

func TestNewAlerter_GChat(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Alerting.Enabled = true
	cfg.Alerting.WebhookURL = "http://example.com/webhook"
	cfg.Alerting.Sink = "gchat"

	alerter := NewAlerter(cfg)
	if _, ok := alerter.(*GChatAlerter); !ok {
		t.Error("Expected GChatAlerter for sink=gchat")
	}
}

func TestNewAlerter_Raw(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Alerting.Enabled = true
	cfg.Alerting.WebhookURL = "http://example.com/webhook"
	cfg.Alerting.Sink = "raw"

	alerter := NewAlerter(cfg)
	if _, ok := alerter.(*RawAlerter); !ok {
		t.Error("Expected RawAlerter for sink=raw")
	}
}

func TestNewAlerter_DefaultIsRaw(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Alerting.Enabled = true
	cfg.Alerting.WebhookURL = "http://example.com/webhook"
	cfg.Alerting.Sink = "" // Empty sink should default to raw

	alerter := NewAlerter(cfg)
	if _, ok := alerter.(*RawAlerter); !ok {
		t.Error("Expected RawAlerter for empty sink")
	}
}

func TestNoOpAlerter_Send(t *testing.T) {
	alerter := &NoOpAlerter{}
	err := alerter.Send(context.Background(), AlertMessage{})
	if err != nil {
		t.Errorf("NoOpAlerter.Send() error = %v, want nil", err)
	}
}

func TestSlackAlerter_Send(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		receivedBody = make([]byte, r.ContentLength)
		r.Body.Read(receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	alerter := NewSlackAlerter(server.URL, "", "Test Cluster")
	msg := AlertMessage{
		WorkloadKind:      "Deployment",
		WorkloadName:      "nginx",
		WorkloadNamespace: "default",
		ResourceKind:      "ConfigMap",
		ResourceName:      "nginx-config",
		ResourceNamespace: "default",
		Timestamp:         time.Now(),
	}

	err := alerter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("SlackAlerter.Send() error = %v", err)
	}

	var slackMsg slackMessage
	if err := json.Unmarshal(receivedBody, &slackMsg); err != nil {
		t.Fatalf("Failed to unmarshal slack message: %v", err)
	}

	if slackMsg.Text == "" {
		t.Error("Expected non-empty text in slack message")
	}
}

func TestTeamsAlerter_Send(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody = make([]byte, r.ContentLength)
		r.Body.Read(receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	alerter := NewTeamsAlerter(server.URL, "", "")
	msg := AlertMessage{
		WorkloadKind:      "Deployment",
		WorkloadName:      "nginx",
		WorkloadNamespace: "default",
		ResourceKind:      "ConfigMap",
		ResourceName:      "nginx-config",
		ResourceNamespace: "default",
		Timestamp:         time.Now(),
	}

	err := alerter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("TeamsAlerter.Send() error = %v", err)
	}

	var teamsMsg teamsMessage
	if err := json.Unmarshal(receivedBody, &teamsMsg); err != nil {
		t.Fatalf("Failed to unmarshal teams message: %v", err)
	}

	if teamsMsg.Type != "MessageCard" {
		t.Errorf("Expected @type=MessageCard, got %s", teamsMsg.Type)
	}
}

func TestGChatAlerter_Send(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody = make([]byte, r.ContentLength)
		r.Body.Read(receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	alerter := NewGChatAlerter(server.URL, "", "")
	msg := AlertMessage{
		WorkloadKind:      "Deployment",
		WorkloadName:      "nginx",
		WorkloadNamespace: "default",
		ResourceKind:      "ConfigMap",
		ResourceName:      "nginx-config",
		ResourceNamespace: "default",
		Timestamp:         time.Now(),
	}

	err := alerter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("GChatAlerter.Send() error = %v", err)
	}

	var gchatMsg gchatMessage
	if err := json.Unmarshal(receivedBody, &gchatMsg); err != nil {
		t.Fatalf("Failed to unmarshal gchat message: %v", err)
	}

	if len(gchatMsg.Cards) != 1 {
		t.Errorf("Expected 1 card, got %d", len(gchatMsg.Cards))
	}
}

func TestRawAlerter_Send(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody = make([]byte, r.ContentLength)
		r.Body.Read(receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	alerter := NewRawAlerter(server.URL, "", "custom-info")
	msg := AlertMessage{
		WorkloadKind:      "Deployment",
		WorkloadName:      "nginx",
		WorkloadNamespace: "default",
		ResourceKind:      "ConfigMap",
		ResourceName:      "nginx-config",
		ResourceNamespace: "default",
		Timestamp:         time.Now(),
	}

	err := alerter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("RawAlerter.Send() error = %v", err)
	}

	var rawMsg rawMessage
	if err := json.Unmarshal(receivedBody, &rawMsg); err != nil {
		t.Fatalf("Failed to unmarshal raw message: %v", err)
	}

	if rawMsg.Event != "reload" {
		t.Errorf("Expected event=reload, got %s", rawMsg.Event)
	}
	if rawMsg.WorkloadName != "nginx" {
		t.Errorf("Expected workloadName=nginx, got %s", rawMsg.WorkloadName)
	}
	if rawMsg.Additional != "custom-info" {
		t.Errorf("Expected additional=custom-info, got %s", rawMsg.Additional)
	}
}

func TestAlerter_WebhookError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	alerter := NewRawAlerter(server.URL, "", "")
	err := alerter.Send(context.Background(), AlertMessage{})
	if err == nil {
		t.Error("Expected error for non-2xx response")
	}
}
