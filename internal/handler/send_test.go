package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	calls := 0
	mock := &mockSender{
		sendFunc: func(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
			calls++
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
	// Transient failures are not cached, so the same key can be retried.
	req2 := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusInternalServerError {
		t.Errorf("retry response status = %d, want 500", resp2.StatusCode)
	}
	if calls != 2 {
		t.Errorf("Send() calls = %d, want 2 retries", calls)
	}
}

func TestSendHandler_MapsProviderErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    *provider.ProviderError
		status int
	}{
		{name: "invalid destination", err: provider.ErrInvalidDestination("invalid recipient"), status: http.StatusBadRequest},
		{name: "validation", err: provider.ErrValidationFailed("invalid subject"), status: http.StatusBadRequest},
		{name: "timeout", err: provider.ErrTimeout("SMTP send timed out", context.DeadlineExceeded), status: http.StatusGatewayTimeout},
		{name: "provider down", err: provider.ErrProviderDown("SMTP unavailable", nil), status: http.StatusServiceUnavailable},
		{name: "invalid config", err: provider.ErrInvalidConfig("bad config"), status: http.StatusServiceUnavailable},
		{name: "rate limited", err: provider.ErrRateLimited("slow down"), status: http.StatusTooManyRequests},
		{name: "unauthorized", err: provider.ErrUnauthorized("bad credentials"), status: http.StatusUnauthorized},
		{name: "idempotency conflict", err: provider.ErrIdempotencyConflict("key reused"), status: http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSender{sendFunc: func(context.Context, *provider.Message) (*provider.SendResult, error) {
				result := provider.NewFailureResult("smtp", provider.ChannelEmail, tt.err)
				return result, tt.err
			}}
			app := testApp(mock)
			body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com"})
			req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.status)
			}
			var out provider.HTTPSendResponse
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatal(err)
			}
			if out.ErrorCode != string(tt.err.Reason) || out.ErrorMessage != tt.err.Message {
				t.Errorf("response = %#v, want reason %q message %q", out, tt.err.Reason, tt.err.Message)
			}
		})
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

func TestSendHandler_RejectsConflictingIdempotencyKeys(t *testing.T) {
	mock := &mockSender{}
	app := testApp(mock)
	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com", IdempotencyKey: "body-key"})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "header-key")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSendHandler_RejectsOversizedIdempotencyKey(t *testing.T) {
	mock := &mockSender{}
	app := testApp(mock)
	body, _ := json.Marshal(provider.HTTPSendRequest{
		To:             "u@example.com",
		IdempotencyKey: strings.Repeat("k", maxIdempotencyKeyBytes+1),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSendHandler_IdempotencyKeyContentConflict(t *testing.T) {
	calls := 0
	mock := &mockSender{sendFunc: func(context.Context, *provider.Message) (*provider.SendResult, error) {
		calls++
		return provider.NewSuccessResult("smtp", provider.ChannelEmail, "msg-123"), nil
	}}
	app := testApp(mock)

	firstBody, _ := json.Marshal(provider.HTTPSendRequest{To: "first@example.com", IdempotencyKey: "same-key"})
	first := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(firstBody))
	first.Header.Set("Content-Type", "application/json")
	firstResp, err := app.Test(first, -1)
	if err != nil || firstResp.StatusCode != http.StatusOK {
		t.Fatalf("first request status = %d, error = %v", firstResp.StatusCode, err)
	}

	secondBody, _ := json.Marshal(provider.HTTPSendRequest{To: "second@example.com", IdempotencyKey: "same-key"})
	second := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(secondBody))
	second.Header.Set("Content-Type", "application/json")
	secondResp, err := app.Test(second, -1)
	if err != nil {
		t.Fatal(err)
	}
	if secondResp.StatusCode != http.StatusConflict {
		t.Fatalf("second request status = %d, want 409", secondResp.StatusCode)
	}
	var out provider.HTTPSendResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.ErrorCode != string(provider.ReasonIdempotencyConflict) {
		t.Errorf("error_code = %q, want idempotency_conflict", out.ErrorCode)
	}
	if calls != 1 {
		t.Errorf("Send() calls = %d, want 1", calls)
	}
}

func TestSendHandler_ConcurrentIdempotentRequestsSendOnce(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	mock := &mockSender{sendFunc: func(context.Context, *provider.Message) (*provider.SendResult, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return provider.NewSuccessResult("smtp", provider.ChannelEmail, "msg-concurrent"), nil
	}}
	app := testApp(mock)
	body, _ := json.Marshal(provider.HTTPSendRequest{To: "u@example.com", IdempotencyKey: "concurrent-key"})
	send := func(result chan<- *http.Response) {
		req := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			result <- nil
			return
		}
		result <- resp
	}

	responses := make(chan *http.Response, 2)
	go send(responses)
	<-started
	go send(responses)
	time.Sleep(25 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		close(release)
		t.Fatalf("concurrent Send() calls before release = %d, want 1", got)
	}
	close(release)
	for range 2 {
		resp := <-responses
		if resp == nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("concurrent response = %#v, want HTTP 200", resp)
		}
		var out provider.HTTPSendResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		if !out.OK || out.MessageID != "msg-concurrent" {
			t.Errorf("concurrent response body = %#v", out)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("concurrent Send() calls = %d, want 1", got)
	}
}

func TestMaskedDestination(t *testing.T) {
	tests := map[string]string{
		"user@example.com":              "***@example.com",
		"A <user@example.com>":          "***@example.com",
		"x@example.com":                 "***@example.com",
		"not-an-email":                  "***",
		"user@example.com secret-token": "***",
		"local":                         "***",
	}
	for input, want := range tests {
		if got := maskedDestination(input); got != want {
			t.Errorf("maskedDestination(%q) = %q, want %q", input, got, want)
		}
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
