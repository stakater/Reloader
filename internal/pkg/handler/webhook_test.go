package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stakater/Reloader/pkg/common"
)

func TestSendUpgradeWebhookReturnsErrorOnNon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	err := sendUpgradeWebhook(common.Config{ResourceName: "my-cm", Namespace: "default"}, server.URL)
	assert.Error(t, err)
}

func TestSendUpgradeWebhookSucceedsOn2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	err := sendUpgradeWebhook(common.Config{ResourceName: "my-cm", Namespace: "default"}, server.URL)
	assert.NoError(t, err)
}
