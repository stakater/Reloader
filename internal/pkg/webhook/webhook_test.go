package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

func TestNewClient_SetsURL(t *testing.T) {
	c := NewClient("http://example.com/webhook", logr.Discard())

	if c == nil {
		t.Fatal("NewClient should not return nil")
	}
	if c.url != "http://example.com/webhook" {
		t.Errorf("URL = %q, want %q", c.url, "http://example.com/webhook")
	}
	if c.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", c.httpClient.Timeout, 30*time.Second)
	}
}

func TestIsConfigured_NilClient(t *testing.T) {
	var c *Client = nil

	if c.IsConfigured() {
		t.Error("IsConfigured() should return false for nil client")
	}
}

func TestIsConfigured_EmptyURL(t *testing.T) {
	c := NewClient("", logr.Discard())

	if c.IsConfigured() {
		t.Error("IsConfigured() should return false for empty URL")
	}
}

func TestIsConfigured_ValidURL(t *testing.T) {
	c := NewClient("http://example.com/webhook", logr.Discard())

	if !c.IsConfigured() {
		t.Error("IsConfigured() should return true for valid URL")
	}
}

func TestSend_EmptyURL_ReturnsNil(t *testing.T) {
	c := NewClient("", logr.Discard())

	payload := Payload{
		Kind:         "ConfigMap",
		Namespace:    "default",
		ResourceName: "my-config",
		ResourceType: "configmap",
	}

	err := c.Send(context.Background(), payload)
	if err != nil {
		t.Errorf("Send() with empty URL should return nil, got %v", err)
	}
}

func TestSend_MarshalPayload(t *testing.T) {
	var receivedPayload Payload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(server.URL, logr.Discard())

	payload := Payload{
		Kind:         "ConfigMap",
		Namespace:    "default",
		ResourceName: "my-config",
		ResourceType: "configmap",
		Hash:         "abc123",
		Timestamp:    time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Workloads: []WorkloadInfo{
			{Kind: "Deployment", Name: "my-deploy", Namespace: "default"},
		},
	}

	err := c.Send(context.Background(), payload)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if receivedPayload.Kind != "ConfigMap" {
		t.Errorf("Received Kind = %q, want %q", receivedPayload.Kind, "ConfigMap")
	}
	if receivedPayload.Namespace != "default" {
		t.Errorf("Received Namespace = %q, want %q", receivedPayload.Namespace, "default")
	}
	if receivedPayload.ResourceName != "my-config" {
		t.Errorf("Received ResourceName = %q, want %q", receivedPayload.ResourceName, "my-config")
	}
	if receivedPayload.Hash != "abc123" {
		t.Errorf("Received Hash = %q, want %q", receivedPayload.Hash, "abc123")
	}
	if len(receivedPayload.Workloads) != 1 {
		t.Errorf("Received Workloads count = %d, want 1", len(receivedPayload.Workloads))
	}
}

func TestSend_SetsCorrectHeaders(t *testing.T) {
	var contentType, userAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		userAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(server.URL, logr.Discard())

	err := c.Send(context.Background(), Payload{})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
	if userAgent != "Reloader/2.0" {
		t.Errorf("User-Agent = %q, want %q", userAgent, "Reloader/2.0")
	}
}

func TestSend_UsesPostMethod(t *testing.T) {
	var method string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(server.URL, logr.Discard())

	err := c.Send(context.Background(), Payload{})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if method != http.MethodPost {
		t.Errorf("Method = %q, want %q", method, http.MethodPost)
	}
}

func TestSend_Non2xxResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"200 OK", 200, false},
		{"201 Created", 201, false},
		{"204 No Content", 204, false},
		{"299 upper bound", 299, false},
		{"300 redirect", 300, true},
		{"400 Bad Request", 400, true},
		{"404 Not Found", 404, true},
		{"500 Internal Error", 500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			c := NewClient(server.URL, logr.Discard())
			err := c.Send(context.Background(), Payload{})

			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSend_NetworkError(t *testing.T) {
	// Use a URL that won't connect
	c := NewClient("http://127.0.0.1:1", logr.Discard())

	err := c.Send(context.Background(), Payload{})
	if err == nil {
		t.Error("Send() should return error for network failure")
	}
}

func TestSend_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(server.URL, logr.Discard())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := c.Send(ctx, Payload{})
	if err == nil {
		t.Error("Send() should return error for cancelled context")
	}
}

func TestPayload_JSONSerialization(t *testing.T) {
	payload := Payload{
		Kind:         "ConfigMap",
		Namespace:    "default",
		ResourceName: "my-config",
		ResourceType: "configmap",
		Hash:         "abc123",
		Timestamp:    time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Workloads: []WorkloadInfo{
			{Kind: "Deployment", Name: "my-deploy", Namespace: "default"},
			{Kind: "StatefulSet", Name: "my-sts", Namespace: "default"},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	var unmarshaled Payload
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if unmarshaled.Kind != payload.Kind {
		t.Errorf("Kind = %q, want %q", unmarshaled.Kind, payload.Kind)
	}
	if len(unmarshaled.Workloads) != 2 {
		t.Errorf("Workloads count = %d, want 2", len(unmarshaled.Workloads))
	}
}

func TestWorkloadInfo_JSONSerialization(t *testing.T) {
	info := WorkloadInfo{
		Kind:      "Deployment",
		Name:      "my-deploy",
		Namespace: "production",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var unmarshaled WorkloadInfo
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if unmarshaled.Kind != "Deployment" {
		t.Errorf("Kind = %q, want %q", unmarshaled.Kind, "Deployment")
	}
	if unmarshaled.Name != "my-deploy" {
		t.Errorf("Name = %q, want %q", unmarshaled.Name, "my-deploy")
	}
	if unmarshaled.Namespace != "production" {
		t.Errorf("Namespace = %q, want %q", unmarshaled.Namespace, "production")
	}
}
