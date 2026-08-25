package handler

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"

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

const maxIdempotencyKeyBytes = 256

// SendHandler handles POST /v1/send from Herald.
func SendHandler(c *fiber.Ctx, smtpClient smtpSender, idemStore *idempotency.Store, log *logger.Logger) error {
	if !authorized(c) {
		log.Warn().Str("client_ip", c.IP()).Msg("send unauthorized: invalid or missing API key")
		return c.Status(fiber.StatusUnauthorized).JSON(provider.HTTPSendResponse{
			OK: false, ErrorCode: "unauthorized", ErrorMessage: "invalid or missing API key",
		})
	}

	req, err := parseRequest(c)
	if err != nil {
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

	fingerprint := ""
	reservationHeld := false
	if req.IdempotencyKey != "" {
		fingerprint = requestFingerprint(req)
		cached, decision, beginErr := idemStore.Begin(c.Context(), req.IdempotencyKey, fingerprint)
		if beginErr != nil {
			providerErr := provider.ErrSendFailed("idempotent request wait canceled", beginErr)
			return sendFailure(c, nil, providerErr)
		}
		switch decision {
		case idempotency.Conflict:
			providerErr := provider.ErrIdempotencyConflict("idempotency key was already used for a different request")
			return sendFailure(c, nil, providerErr)
		case idempotency.Hit:
			log.Debug().Str("to", maskedDestination(req.To)).Bool("cached_ok", cached.OK).Str("message_id", cached.MessageID).Msg("send idempotent hit")
			return c.JSON(provider.HTTPSendResponse{
				OK: cached.OK, MessageID: cached.MessageID, Provider: "smtp",
			})
		case idempotency.Proceed:
			reservationHeld = true
		}
	}
	finishReservation := func(ok bool, messageID string) {
		if reservationHeld {
			idemStore.Finish(req.IdempotencyKey, fingerprint, ok, messageID)
			reservationHeld = false
		}
	}
	defer finishReservation(false, "")

	result, err := smtpClient.Send(c.Context(), buildMessage(req))
	if err != nil {
		log.Warn().Err(err).Str("to", maskedDestination(req.To)).Msg("send_failed: SMTP error")
		return sendFailure(c, result, err)
	}
	if result == nil || !result.OK {
		return sendFailure(c, result, nil)
	}

	messageID := result.MessageID
	finishReservation(true, messageID)
	log.Info().Str("to", maskedDestination(req.To)).Str("message_id", messageID).Msg("send ok")
	return c.JSON(provider.HTTPSendResponse{
		OK: true, MessageID: messageID, Provider: "smtp",
	})
}

// authorized reports whether the request carries a valid API key (or auth is disabled).
func authorized(c *fiber.Ctx) bool {
	if config.APIKey == "" {
		return true
	}
	actual := sha256.Sum256([]byte(c.Get("X-API-Key")))
	expected := sha256.Sum256([]byte(config.APIKey))
	return subtle.ConstantTimeCompare(actual[:], expected[:]) == 1
}

// parseRequest parses the send request body and resolves the idempotency key from the header.
func parseRequest(c *fiber.Ctx) (provider.HTTPSendRequest, error) {
	var req provider.HTTPSendRequest
	if err := c.BodyParser(&req); err != nil {
		return req, err
	}
	headerKey := c.Get("Idempotency-Key")
	if req.IdempotencyKey != "" && headerKey != "" && req.IdempotencyKey != headerKey {
		return req, errors.New("conflicting idempotency keys in body and header")
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = headerKey
	}
	if len(req.IdempotencyKey) > maxIdempotencyKeyBytes {
		return req, errors.New("idempotency key exceeds 256 bytes")
	}
	return req, nil
}

func requestFingerprint(req provider.HTTPSendRequest) string {
	req.IdempotencyKey = ""
	payload, _ := json.Marshal(req)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func maskedDestination(raw string) string {
	address, err := mail.ParseAddress(raw)
	if err != nil {
		return "***"
	}
	at := strings.LastIndexByte(address.Address, '@')
	if at <= 0 {
		return "***"
	}
	return "***" + address.Address[at:]
}

// buildMessage constructs the provider message from the request, applying defaults.
func buildMessage(req provider.HTTPSendRequest) *provider.Message {
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
	return msg
}

// sendFailure preserves provider-kit error reasons in the HTTP response and
// maps them to retry-friendly status codes. Failed sends are deliberately not
// cached so callers can retry transient SMTP failures with the same key.
func sendFailure(c *fiber.Ctx, result *provider.SendResult, sendErr error) error {
	reason := provider.ReasonSendFailed
	message := ""
	if result != nil && result.Error != nil {
		reason = result.Error.Reason
		message = result.Error.Message
	} else {
		var providerErr *provider.ProviderError
		if errors.As(sendErr, &providerErr) {
			reason = providerErr.Reason
			message = providerErr.Message
		} else if sendErr != nil {
			message = sendErr.Error()
		}
	}
	return c.Status(statusForReason(reason)).JSON(provider.HTTPSendResponse{
		OK: false, ErrorCode: string(reason), ErrorMessage: message,
	})
}

func statusForReason(reason provider.ErrorReason) int {
	switch reason {
	case provider.ReasonInvalidDestination, provider.ReasonValidationFailed:
		return fiber.StatusBadRequest
	case provider.ReasonUnauthorized:
		return fiber.StatusUnauthorized
	case provider.ReasonIdempotencyConflict:
		return fiber.StatusConflict
	case provider.ReasonRateLimited:
		return fiber.StatusTooManyRequests
	case provider.ReasonTimeout:
		return fiber.StatusGatewayTimeout
	case provider.ReasonProviderDown, provider.ReasonInvalidConfig, provider.ReasonNotRegistered:
		return fiber.StatusServiceUnavailable
	default:
		return fiber.StatusInternalServerError
	}
}
