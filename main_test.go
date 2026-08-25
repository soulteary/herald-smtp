package main

import (
	"testing"
	"time"

	"github.com/soulteary/herald-smtp/internal/config"
)

func TestNewAppUsesRuntimeLimits(t *testing.T) {
	oldSMTPTimeout := config.SMTPTimeoutSec
	oldBodyLimit := config.HTTPBodyLimitBytes
	oldReadTimeout := config.HTTPReadTimeoutSec
	oldWriteTimeout := config.HTTPWriteTimeoutSec
	oldIdleTimeout := config.HTTPIdleTimeoutSec
	t.Cleanup(func() {
		config.SMTPTimeoutSec = oldSMTPTimeout
		config.HTTPBodyLimitBytes = oldBodyLimit
		config.HTTPReadTimeoutSec = oldReadTimeout
		config.HTTPWriteTimeoutSec = oldWriteTimeout
		config.HTTPIdleTimeoutSec = oldIdleTimeout
	})

	config.SMTPTimeoutSec = 20
	config.HTTPBodyLimitBytes = 8192
	config.HTTPReadTimeoutSec = 8
	config.HTTPWriteTimeoutSec = 30
	config.HTTPIdleTimeoutSec = 90

	app := newApp()
	cfg := app.Config()
	if cfg.BodyLimit != 8192 {
		t.Errorf("BodyLimit = %d, want 8192", cfg.BodyLimit)
	}
	if cfg.ReadTimeout != 8*time.Second {
		t.Errorf("ReadTimeout = %v, want 8s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want 30s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 90*time.Second {
		t.Errorf("IdleTimeout = %v, want 90s", cfg.IdleTimeout)
	}
}
