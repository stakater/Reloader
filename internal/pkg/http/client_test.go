package http

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, cfg.Timeout)
	}
	if cfg.MaxIdleConns != 100 {
		t.Errorf("expected MaxIdleConns 100, got %d", cfg.MaxIdleConns)
	}
	if cfg.MaxIdleConnsPerHost != 10 {
		t.Errorf("expected MaxIdleConnsPerHost 10, got %d", cfg.MaxIdleConnsPerHost)
	}
	if cfg.IdleConnTimeout != 90*time.Second {
		t.Errorf("expected IdleConnTimeout 90s, got %v", cfg.IdleConnTimeout)
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ClientConfig
		wantNil bool
	}{
		{
			name:    "default config",
			cfg:     DefaultConfig(),
			wantNil: false,
		},
		{
			name: "custom timeout",
			cfg: ClientConfig{
				Timeout:             5 * time.Second,
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     30 * time.Second,
			},
			wantNil: false,
		},
		{
			name: "with proxy",
			cfg: ClientConfig{
				Timeout:             DefaultTimeout,
				ProxyURL:            "http://proxy.example.com:8080",
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
			wantNil: false,
		},
		{
			name: "with invalid proxy URL",
			cfg: ClientConfig{
				Timeout:             DefaultTimeout,
				ProxyURL:            "://invalid",
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
			wantNil: false,
		},
		{
			name: "zero values",
			cfg: ClientConfig{
				Timeout: 0,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				client := NewClient(tt.cfg)

				if tt.wantNil && client != nil {
					t.Error("expected nil client")
				}
				if !tt.wantNil && client == nil {
					t.Error("expected non-nil client")
				}

				if client != nil {
					if client.Timeout != tt.cfg.Timeout {
						t.Errorf("expected timeout %v, got %v", tt.cfg.Timeout, client.Timeout)
					}

					transport, ok := client.Transport.(*http.Transport)
					if !ok {
						t.Fatal("expected *http.Transport")
					}
					if transport.MaxIdleConns != tt.cfg.MaxIdleConns {
						t.Errorf("expected MaxIdleConns %d, got %d", tt.cfg.MaxIdleConns, transport.MaxIdleConns)
					}
					if transport.MaxIdleConnsPerHost != tt.cfg.MaxIdleConnsPerHost {
						t.Errorf("expected MaxIdleConnsPerHost %d, got %d", tt.cfg.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
					}
				}
			},
		)
	}
}

func TestNewDefaultClient(t *testing.T) {
	client := NewDefaultClient()

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.Timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}

	if transport.MaxIdleConns != 100 {
		t.Errorf("expected MaxIdleConns 100, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("expected MaxIdleConnsPerHost 10, got %d", transport.MaxIdleConnsPerHost)
	}
}

func TestConstants(t *testing.T) {
	if DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout 30s, got %v", DefaultTimeout)
	}
	if AlertingTimeout != 10*time.Second {
		t.Errorf("expected AlertingTimeout 10s, got %v", AlertingTimeout)
	}
}
