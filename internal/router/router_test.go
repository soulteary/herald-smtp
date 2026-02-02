package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/soulteary/herald-smtp/internal/config"
	"github.com/soulteary/logger-kit"
	"github.com/soulteary/provider-kit"
)

// mockSendClient implements sendClient for router tests.
type mockSendClient struct {
	sendFunc func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error)
}

func (m *mockSendClient) Send(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, msg)
	}
	return provider.NewSuccessResult("smtp", provider.ChannelEmail, "mock-msg"), nil
}

func TestRouter_Healthz(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	log := logger.New(logger.Config{Level: logger.InfoLevel, ServiceName: "test"})
	Setup(app, log)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /healthz status = %d, want 200", resp.StatusCode)
	}
}

// TestRouter_Send_503WhenNotConfigured: when no client (nil inject and config invalid), POST /v1/send returns 503.
func TestRouter_Send_503WhenNotConfigured(t *testing.T) {
	if config.Valid() {
		t.Skip("SMTP configured in env; cannot test 503 path")
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	log := logger.New(logger.Config{Level: logger.InfoLevel, ServiceName: "test"})
	Setup(app, log)

	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("POST /v1/send status = %d, want 503 when SMTP not configured", resp.StatusCode)
	}
	var out provider.HTTPSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.OK || out.ErrorCode != "provider_down" {
		t.Errorf("response OK=%v error_code=%q", out.OK, out.ErrorCode)
	}
}

// TestRouter_Send_200WhenInjectClient: when inject client is provided, POST /v1/send uses it and returns 200.
func TestRouter_Send_200WhenInjectClient(t *testing.T) {
	mock := &mockSendClient{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			return provider.NewSuccessResult("smtp", provider.ChannelEmail, "inject-msg"), nil
		},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	log := logger.New(logger.Config{Level: logger.InfoLevel, ServiceName: "test"})
	setupWith(app, log, mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("POST /v1/send status = %d, want 200", resp.StatusCode)
	}
	var out provider.HTTPSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.MessageID != "inject-msg" {
		t.Errorf("response OK=%v message_id=%q", out.OK, out.MessageID)
	}
}
