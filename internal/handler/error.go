package handler

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/provider-kit"
)

// FrameworkErrorHandler keeps errors raised before route handlers on the same
// JSON contract as /v1/send responses.
func FrameworkErrorHandler(c fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		status = fiberErr.Code
	}

	code := provider.ReasonSendFailed
	message := "internal server error"
	switch status {
	case fiber.StatusBadRequest:
		code = "invalid_request"
		message = "invalid request"
	case fiber.StatusUnauthorized:
		code = provider.ReasonUnauthorized
		message = "unauthorized"
	case fiber.StatusNotFound:
		code = "not_found"
		message = "route not found"
	case fiber.StatusMethodNotAllowed:
		code = "method_not_allowed"
		message = "method not allowed"
	case fiber.StatusRequestEntityTooLarge:
		code = "invalid_request"
		message = "request body exceeds configured limit"
	case fiber.StatusUnsupportedMediaType:
		code = "invalid_request"
		message = "unsupported media type"
	case fiber.StatusTooManyRequests:
		code = provider.ReasonRateLimited
		message = "rate limited"
	}

	return c.Status(status).JSON(provider.HTTPSendResponse{
		OK: false, ErrorCode: string(code), ErrorMessage: message,
	})
}
