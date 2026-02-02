package config

import (
	"time"

	"github.com/soulteary/cli-kit/env"
)

var (
	Port       = env.Get("PORT", ":8084")
	APIKey     = env.Get("API_KEY", "")
	SMTPHost   = env.Get("SMTP_HOST", "")
	SMTPPort   = env.GetInt("SMTP_PORT", 587)
	SMTPUser   = env.Get("SMTP_USER", "")
	SMTPPass   = env.Get("SMTP_PASSWORD", "")
	SMTPFrom   = env.Get("SMTP_FROM", "")
	UseStartTLS = env.GetBool("SMTP_USE_STARTTLS", true)
	LogLevel   = env.Get("LOG_LEVEL", "info")
	IdemTTLSec = env.GetInt("IDEMPOTENCY_TTL_SECONDS", 300)
)

// Valid returns true when SMTP is configured (host, from required for send).
func Valid() bool {
	return SMTPHost != "" && SMTPFrom != ""
}

// SMTPTimeout returns a reasonable send timeout (used when building provider-kit SMTPConfig).
func SMTPTimeout() time.Duration {
	return 30 * time.Second
}
