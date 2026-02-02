package router

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/soulteary/health-kit"
	"github.com/soulteary/herald-smtp/internal/config"
	"github.com/soulteary/herald-smtp/internal/handler"
	"github.com/soulteary/herald-smtp/internal/idempotency"
	"github.com/soulteary/herald-smtp/internal/smtp"
	"github.com/soulteary/logger-kit"
	"github.com/soulteary/provider-kit"
)

// sendClient is the minimal interface used by the send handler (for injection in tests).
type sendClient interface {
	Send(ctx context.Context, msg *provider.Message) (*provider.SendResult, error)
}

// Setup mounts routes. smtpClient can be nil if config invalid (send will return 503).
func Setup(app *fiber.App, log *logger.Logger) {
	setupWith(app, log, nil)
}

// setupWith mounts routes; if inject is non-nil it is used as the send client (for tests).
func setupWith(app *fiber.App, log *logger.Logger, inject sendClient) {
	idemStore := idempotency.NewStore(config.IdemTTLSec)
	var smtpClient sendClient
	if inject != nil {
		smtpClient = inject
	} else if config.Valid() {
		client, err := smtp.NewClient()
		if err != nil {
			log.Warn().Err(err).Msg("failed to create SMTP client")
		} else {
			smtpClient = client
		}
	}
	v1 := app.Group("/v1")
	v1.Post("/send", func(c *fiber.Ctx) error {
		if smtpClient == nil {
			log.Warn().Msg("send 503: SMTP not configured")
			return c.Status(fiber.StatusServiceUnavailable).JSON(provider.HTTPSendResponse{
				OK: false, ErrorCode: "provider_down", ErrorMessage: "SMTP not configured",
			})
		}
		return handler.SendHandler(c, smtpClient, idemStore, log)
	})
	app.Get("/healthz", health.SimpleFiberHandler("herald-smtp"))
}
