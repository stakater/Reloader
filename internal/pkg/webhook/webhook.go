// Package webhook handles sending reload notifications to external endpoints.
// When --webhook-url is set, Reloader sends HTTP POST requests instead of modifying workloads.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	httputil "github.com/stakater/Reloader/internal/pkg/http"
)

// Payload represents the data sent to the webhook endpoint.
type Payload struct {
	Kind         string    `json:"kind"`
	Namespace    string    `json:"namespace"`
	ResourceName string    `json:"resourceName"`
	ResourceType string    `json:"resourceType"`
	Hash         string    `json:"hash"`
	Timestamp    time.Time `json:"timestamp"`

	// Workloads contains the list of workloads that would be reloaded.
	Workloads []WorkloadInfo `json:"workloads"`
}

// WorkloadInfo describes a workload that would be reloaded.
type WorkloadInfo struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// Client sends reload notifications to webhook endpoints.
type Client struct {
	httpClient *http.Client
	url        string
	log        logr.Logger
}

// NewClient creates a new webhook client.
func NewClient(url string, log logr.Logger) *Client {
	return &Client{
		httpClient: httputil.NewDefaultClient(),
		url:        url,
		log:        log,
	}
}

// Send posts the payload to the configured webhook URL.
func (c *Client) Send(ctx context.Context, payload Payload) error {
	if c.url == "" {
		return nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Reloader/2.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	c.log.V(1).Info("webhook notification sent",
		"url", c.url,
		"resourceType", payload.ResourceType,
		"resourceName", payload.ResourceName,
		"workloadCount", len(payload.Workloads),
	)

	return nil
}

// IsConfigured returns true if the webhook URL is set.
func (c *Client) IsConfigured() bool {
	return c != nil && c.url != ""
}
