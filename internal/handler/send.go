package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/soulteary/herald-smtp/internal/config"
	"github.com/soulteary/herald-smtp/internal/idempotency"
	"github.com/soulteary/logger-kit"
	"github.com/soulteary/provider-kit"
)

// smtpSender sends email; *smtp.Client implements it. Used for testing with mock.
type smtpSender interface {
	Send(ctx context.Context, msg *provider.Message) (*provider.SendResult, error)
}

// SendHandler handles POST /v1/send from Herald.
func SendHandler(c *fiber.Ctx, smtpClient smtpSender, idemStore *idempotency.Store, log *logger.Logger) error {
	if config.APIKey != "" && c.Get("X-API-Key") != config.APIKey {
		log.Warn().Str("client_ip", c.IP()).Msg("send unauthorized: invalid or missing API key")
		return c.Status(fiber.StatusUnauthorized).JSON(provider.HTTPSendResponse{
			OK: false, ErrorCode: "unauthorized", ErrorMessage: "invalid or missing API key",
		})
	}
	var req provider.HTTPSendRequest
	if err := c.BodyParser(&req); err != nil {
		log.Warn().Err(err).Msg("send invalid_request: body parse error")
		return c.Status(fiber.StatusBadRequest).JSON(provider.HTTPSendResponse{
			OK: false, ErrorCode: "invalid_request", ErrorMessage: err.Error(),
		})
	}
	if req.To == "" {
		log.Warn().Msg("send invalid_destination: to is required")
		return c.Status(fiber.StatusBadRequest).JSON(provider.HTTPSendResponse{
			OK: false, ErrorCode: "invalid_destination", ErrorMessage: "to is required",
		})
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = c.Get("Idempotency-Key")
	}
	if req.IdempotencyKey != "" {
		if cached, hit := idemStore.Get(req.IdempotencyKey); hit {
			log.Debug().Str("to", req.To).Bool("cached_ok", cached.OK).Str("message_id", cached.MessageID).Msg("send idempotent hit")
			return c.JSON(provider.HTTPSendResponse{
				OK: cached.OK, MessageID: cached.MessageID, Provider: "smtp",
			})
		}
	}
	subject := req.Subject
	if subject == "" {
		subject = "Verification code"
	}
	body := req.Body
	if body == "" && len(req.Params) > 0 {
		if code, ok := req.Params["code"]; ok {
			body = "Your verification code is: " + code
		}
	}
	if body == "" {
		body = "You have a verification message. Please check your code."
	}
	msg := provider.NewMessage(req.To).
		WithSubject(subject).
		WithBody(body).
		WithLocale(req.Locale).
		WithIdempotencyKey(req.IdempotencyKey)
	if len(req.Params) > 0 {
		msg.WithParams(req.Params)
	}
	result, err := smtpClient.Send(c.Context(), msg)
	if err != nil {
		log.Warn().Err(err).Str("to", req.To).Msg("send_failed: SMTP error")
		if req.IdempotencyKey != "" {
			idemStore.Set(req.IdempotencyKey, false, "")
		}
		return c.Status(fiber.StatusInternalServerError).JSON(provider.HTTPSendResponse{
			OK: false, ErrorCode: "send_failed", ErrorMessage: err.Error(),
		})
	}
	if result == nil || !result.OK {
		errCode := "send_failed"
		errMsg := ""
		if result != nil && result.Error != nil {
			errCode = string(result.Error.Reason)
			errMsg = result.Error.Message
		}
		if req.IdempotencyKey != "" {
			idemStore.Set(req.IdempotencyKey, false, "")
		}
		return c.Status(fiber.StatusInternalServerError).JSON(provider.HTTPSendResponse{
			OK: false, ErrorCode: errCode, ErrorMessage: errMsg,
		})
	}
	messageID := result.MessageID
	if req.IdempotencyKey != "" {
		idemStore.Set(req.IdempotencyKey, true, messageID)
	}
	log.Info().Str("to", req.To).Str("message_id", messageID).Msg("send ok")
	return c.JSON(provider.HTTPSendResponse{
		OK: true, MessageID: messageID, Provider: "smtp",
	})
}
