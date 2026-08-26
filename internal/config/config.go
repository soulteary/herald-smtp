package config

import (
	"time"

	"github.com/soulteary/cli-kit/env"
)

var (
	Port                = env.Get("PORT", ":8084")
	APIKey              = env.Get("API_KEY", "")
	SMTPHost            = env.Get("SMTP_HOST", "")
	SMTPPort            = env.GetInt("SMTP_PORT", 587)
	SMTPUser            = env.Get("SMTP_USER", "")
	SMTPPass            = env.Get("SMTP_PASSWORD", "")
	SMTPFrom            = env.Get("SMTP_FROM", "")
	SMTPFromName        = env.Get("SMTP_FROM_NAME", "")
	UseTLS              = env.GetBool("SMTP_USE_TLS", false)
	UseStartTLS         = env.GetBool("SMTP_USE_STARTTLS", true)
	SkipTLSVerify       = env.GetBool("SMTP_SKIP_TLS_VERIFY", false)
	SMTPTimeoutSec      = env.GetInt("SMTP_TIMEOUT_SECONDS", 30)
	SMTPMaxConcurrent   = env.GetInt("SMTP_MAX_CONCURRENT_SENDS", 16)
	LogLevel            = env.Get("LOG_LEVEL", "info")
	IdemTTLSec          = env.GetInt("IDEMPOTENCY_TTL_SECONDS", 300)
	IdemMaxEntries      = env.GetInt("IDEMPOTENCY_MAX_ENTRIES", 10000)
	HTTPBodyLimitBytes  = env.GetInt("HTTP_BODY_LIMIT_BYTES", 64*1024)
	HTTPReadTimeoutSec  = env.GetInt("HTTP_READ_TIMEOUT_SECONDS", 10)
	HTTPWriteTimeoutSec = env.GetInt("HTTP_WRITE_TIMEOUT_SECONDS", 40)
	HTTPIdleTimeoutSec  = env.GetInt("HTTP_IDLE_TIMEOUT_SECONDS", 60)
)

const (
	defaultSMTPTimeoutSec      = 30
	defaultSMTPMaxConcurrent   = 16
	defaultIdemMaxEntries      = 10000
	defaultHTTPBodyLimitBytes  = 64 * 1024
	defaultHTTPReadTimeoutSec  = 10
	defaultHTTPWriteTimeoutSec = 40
	defaultHTTPIdleTimeoutSec  = 60
)

// Valid returns true when SMTP is configured (host, from required for send).
func Valid() bool {
	return SMTPHost != "" && SMTPFrom != ""
}

// SMTPTimeout returns a reasonable send timeout (used when building provider-kit SMTPConfig).
func SMTPTimeout() time.Duration {
	return positiveDuration(SMTPTimeoutSec, defaultSMTPTimeoutSec)
}

// SMTPMaxConcurrentSends bounds simultaneous SMTP connections.
func SMTPMaxConcurrentSends() int {
	return positiveInt(SMTPMaxConcurrent, defaultSMTPMaxConcurrent)
}

// IdempotencyMaxEntries returns the maximum number of in-flight and cached keys.
func IdempotencyMaxEntries() int {
	return positiveInt(IdemMaxEntries, defaultIdemMaxEntries)
}

// HTTPBodyLimit returns the maximum accepted request size in bytes.
func HTTPBodyLimit() int {
	return positiveInt(HTTPBodyLimitBytes, defaultHTTPBodyLimitBytes)
}

// HTTPReadTimeout limits how long the server spends reading a request.
func HTTPReadTimeout() time.Duration {
	return positiveDuration(HTTPReadTimeoutSec, defaultHTTPReadTimeoutSec)
}

// HTTPWriteTimeout includes enough time for the configured SMTP operation.
func HTTPWriteTimeout() time.Duration {
	configured := positiveDuration(HTTPWriteTimeoutSec, defaultHTTPWriteTimeoutSec)
	minimum := SMTPTimeout() + 5*time.Second
	if configured < minimum {
		return minimum
	}
	return configured
}

// HTTPIdleTimeout limits idle keep-alive connections.
func HTTPIdleTimeout() time.Duration {
	return positiveDuration(HTTPIdleTimeoutSec, defaultHTTPIdleTimeoutSec)
}

func positiveInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func positiveDuration(value, fallback int) time.Duration {
	return time.Duration(positiveInt(value, fallback)) * time.Second
}
