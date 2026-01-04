// Package http provides shared HTTP client functionality.
package http

import (
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second

	// AlertingTimeout is the shorter timeout used for alerting.
	AlertingTimeout = 10 * time.Second
)

// ClientConfig configures an HTTP client.
type ClientConfig struct {
	// Timeout for HTTP requests.
	Timeout time.Duration

	// ProxyURL is an optional proxy URL.
	ProxyURL string

	// MaxIdleConns controls the maximum number of idle connections.
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum idle connections per host.
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum time an idle connection remains open.
	IdleConnTimeout time.Duration
}

// DefaultConfig returns the default HTTP client configuration.
func DefaultConfig() ClientConfig {
	return ClientConfig{
		Timeout:             DefaultTimeout,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}

// NewClient creates a new HTTP client with the given configuration.
func NewClient(cfg ClientConfig) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
	}

	if cfg.ProxyURL != "" {
		if proxy, err := url.Parse(cfg.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxy)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}
}

// NewDefaultClient creates an HTTP client with default configuration.
func NewDefaultClient() *http.Client {
	return NewClient(DefaultConfig())
}
