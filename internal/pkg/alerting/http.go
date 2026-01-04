package alerting

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	httputil "github.com/stakater/Reloader/internal/pkg/http"
)

// httpClient wraps http.Client with common configuration.
type httpClient struct {
	client *http.Client
}

// newHTTPClient creates a new httpClient with optional proxy support.
func newHTTPClient(proxyURL string) *httpClient {
	cfg := httputil.DefaultConfig()
	cfg.Timeout = httputil.AlertingTimeout
	cfg.ProxyURL = proxyURL

	return &httpClient{
		client: httputil.NewClient(cfg),
	}
}

// post sends a POST request with JSON body.
func (c *httpClient) post(ctx context.Context, url string, body []byte) error {
	return c.doPost(ctx, url, body, "application/json")
}

// postText sends a POST request with plain text body.
func (c *httpClient) postText(ctx context.Context, url string, text string) error {
	return c.doPost(ctx, url, []byte(text), "text/plain")
}

// doPost sends a POST request with the specified content type.
func (c *httpClient) doPost(ctx context.Context, url string, body []byte, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
