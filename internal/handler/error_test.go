package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/provider-kit"
)

func TestFrameworkErrorHandlerReturnsJSON(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FrameworkErrorHandler})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var out provider.HTTPSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.OK || out.ErrorCode != "not_found" {
		t.Fatalf("response = %#v, want not_found", out)
	}
}

func TestFrameworkErrorHandlerMapsBodyLimit(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 8, ErrorHandler: FrameworkErrorHandler})
	app.Post("/body", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/body", strings.NewReader("payload-too-large"))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}

	var out provider.HTTPSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.OK || out.ErrorCode != "invalid_request" {
		t.Fatalf("response = %#v, want invalid_request", out)
	}
}
