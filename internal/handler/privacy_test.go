package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/herald-smtp/internal/config"
	"github.com/soulteary/herald-smtp/internal/idempotency"
	"github.com/soulteary/logger-kit/v3"
	"github.com/soulteary/provider-kit"
)

func TestMalformedBodyDoesNotExposePayload(t *testing.T) {
	old := config.APIKey
	config.APIKey = "test-service-key"
	t.Cleanup(func() { config.APIKey = old })
	for _, contentType := range []string{"application/xml", "application/json"} {
		t.Run(contentType, func(t *testing.T) {
			const sensitive = "private-verification-code-938217"
			body := `<message><body>&` + sensitive + `;</body></message>`
			if contentType == "application/json" {
				body = `{"params":{"` + sensitive + `":[]}}`
			}
			var logs bytes.Buffer
			log := logger.New(logger.Config{Level: logger.InfoLevel, Output: &logs})
			app := fiber.New()
			app.Post("/v1/send", func(c fiber.Ctx) error {
				return SendHandler(c, &mockSender{sendFunc: nil}, idempotency.NewStore(300, 10), log)
			})
			req := httptest.NewRequest(http.MethodPost, "/v1/send", strings.NewReader(body))
			req.Header.Set("Content-Type", contentType)
			req.Header.Set("X-API-Key", config.APIKey)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var out provider.HTTPSendResponse
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusBadRequest || out.ErrorCode != "invalid_request" || out.OK {
				t.Fatalf("unexpected response: %d %+v", resp.StatusCode, out)
			}
			if strings.Contains(logs.String(), sensitive) {
				t.Error("parser leaked request payload to logs")
			}
			if strings.Contains(out.ErrorMessage, sensitive) {
				t.Error("parser leaked request payload to response")
			}
		})
	}
}
