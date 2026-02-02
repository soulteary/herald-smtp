package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/soulteary/herald-smtp/internal/config"
	"github.com/soulteary/herald-smtp/internal/idempotency"
	"github.com/soulteary/logger-kit"
	"github.com/soulteary/provider-kit"
)

// mockSender implements smtpSender for tests.
type mockSender struct {
	sendFunc func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error)
}

func (m *mockSender) Send(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, msg)
	}
	return nil, nil
}

func testApp(mock smtpSender) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	log := logger.New(logger.Config{Level: logger.InfoLevel, ServiceName: "test"})
	idemStore := idempotency.NewStore(300)
	app.Post("/v1/send", func(c *fiber.Ctx) error {
		return SendHandler(c, mock, idemStore, log)
	})
	return app
}

func TestSendHandler_Unauthorized(t *testing.T) {
	old := config.APIKey
	defer func() { config.APIKey = old }()
	config.APIKey = "secret"

	mock := &mockSender{}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestSendHandler_InvalidRequest_BadBody(t *testing.T) {
	mock := &mockSender{}
	app := testApp(mock)

	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSendHandler_InvalidDestination(t *testing.T) {
	mock := &mockSender{}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{To: ""})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSendHandler_Success(t *testing.T) {
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			return provider.NewSuccessResult("smtp", provider.ChannelEmail, "msg-123"), nil
		},
	}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{
		To:   "u@example.com",
		Body: "Hello",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var out provider.HTTPSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.MessageID != "msg-123" {
		t.Errorf("response OK=%v message_id=%q", out.OK, out.MessageID)
	}
}

func TestSendHandler_SendError(t *testing.T) {
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			return nil, context.DeadlineExceeded
		},
	}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestSendHandler_SendErrorWithIdempotencyKey(t *testing.T) {
	// Error path with IdempotencyKey: idemStore.Set(key, false, "") is called
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			return nil, context.DeadlineExceeded
		},
	}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com", IdempotencyKey: "err-key"})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
	// Retry with same key should get cached failure
	req2 := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("cached failure response status = %d (expect 200 with ok:false)", resp2.StatusCode)
	}
	var out provider.HTTPSendResponse
	_ = json.NewDecoder(resp2.Body).Decode(&out)
	if out.OK {
		t.Error("cached response should be ok=false")
	}
}

func TestSendHandler_ResultNotOK(t *testing.T) {
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			return provider.NewFailureResult("smtp", provider.ChannelEmail, provider.NewProviderError(provider.ReasonSendFailed, "rejected")), nil
		},
	}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestSendHandler_IdempotentHit(t *testing.T) {
	// Use one app so idemStore is shared; first request succeeds, second uses same key and returns cached
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			return provider.NewSuccessResult("smtp", provider.ChannelEmail, "cached-msg"), nil
		},
	}
	app := testApp(mock)
	key := "idem-key-hit"

	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com", IdempotencyKey: key})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp1, _ := app.Test(req, -1)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request status = %d", resp1.StatusCode)
	}

	// Second request with same key should return cached (mock should not be called again if we had call count; we just check 200 and same body)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("idempotent second request status = %d", resp2.StatusCode)
	}
	var out provider.HTTPSendResponse
	_ = json.NewDecoder(resp2.Body).Decode(&out)
	if !out.OK || out.MessageID != "cached-msg" {
		t.Errorf("idempotent response OK=%v message_id=%q", out.OK, out.MessageID)
	}
}

func TestSendHandler_ResultNil(t *testing.T) {
	// Send returns (nil, nil) -> 500
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			return nil, nil
		},
	}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestSendHandler_IdempotencyKeyFromHeader(t *testing.T) {
	// Idempotency-Key from header when not in body
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			return provider.NewSuccessResult("smtp", provider.ChannelEmail, "hdr-msg"), nil
		},
	}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com"}) // no idempotency_key in body
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "header-key")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	// Second request with same key from header should be cached
	req2 := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "header-key")
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("second request status = %d", resp2.StatusCode)
	}
	var out provider.HTTPSendResponse
	_ = json.NewDecoder(resp2.Body).Decode(&out)
	if !out.OK || out.MessageID != "hdr-msg" {
		t.Errorf("cached response OK=%v message_id=%q", out.OK, out.MessageID)
	}
}

func TestSendHandler_DefaultSubjectAndBodyFromParams(t *testing.T) {
	var captured *provider.Message
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			captured = msg
			return provider.NewSuccessResult("smtp", provider.ChannelEmail, "ok"), nil
		},
	}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{
		To:      "u@example.com",
		Subject: "",
		Body:    "",
		Params:  map[string]string{"code": "123456"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if captured == nil {
		t.Fatal("Send was not called")
	}
	// Default subject = "Verification code", body from params["code"]
	if captured.Subject != "Verification code" {
		t.Errorf("subject = %q, want Verification code", captured.Subject)
	}
	expectBody := "Your verification code is: 123456"
	if captured.Body != expectBody {
		t.Errorf("body = %q, want %q", captured.Body, expectBody)
	}
}

// TestSendHandler_DefaultBodyWhenNoCode covers body fallback when params has no "code".
func TestSendHandler_DefaultBodyWhenNoCode(t *testing.T) {
	var captured *provider.Message
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			captured = msg
			return provider.NewSuccessResult("smtp", provider.ChannelEmail, "ok"), nil
		},
	}
	app := testApp(mock)

	body, _ := json.Marshal(provider.HTTPSendRequest{
		To:     "u@example.com",
		Body:   "",
		Params: map[string]string{"other": "x"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if captured == nil {
		t.Fatal("Send was not called")
	}
	expectBody := "You have a verification message. Please check your code."
	if captured.Body != expectBody {
		t.Errorf("body = %q, want %q", captured.Body, expectBody)
	}
}
