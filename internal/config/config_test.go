package config

import (
	"testing"
	"time"
)

func TestValid(t *testing.T) {
	// Valid() depends on env at init; we only check it returns a bool and does not panic
	got := Valid()
	_ = got
}

func TestValid_Logic(t *testing.T) {
	// Valid() is true only when both SMTPHost and SMTPFrom are non-empty (read at init)
	// We cannot change env in test without affecting other tests, so we only assert no panic
	if Valid() && (SMTPHost == "" || SMTPFrom == "") {
		t.Errorf("Valid() true but SMTP_HOST=%q SMTP_FROM=%q", SMTPHost, SMTPFrom)
	}
}

func TestSMTPTimeout(t *testing.T) {
	old := SMTPTimeoutSec
	t.Cleanup(func() { SMTPTimeoutSec = old })
	SMTPTimeoutSec = 45
	got := SMTPTimeout()
	if got != 45*time.Second {
		t.Errorf("SMTPTimeout() = %v, want 45s", got)
	}
}

func TestRuntimeLimits(t *testing.T) {
	oldSMTPTimeout := SMTPTimeoutSec
	oldSMTPMaxConcurrent := SMTPMaxConcurrent
	oldMaxEntries := IdemMaxEntries
	oldBodyLimit := HTTPBodyLimitBytes
	oldReadTimeout := HTTPReadTimeoutSec
	oldWriteTimeout := HTTPWriteTimeoutSec
	oldIdleTimeout := HTTPIdleTimeoutSec
	oldShutdownTimeout := ShutdownTimeoutSec
	t.Cleanup(func() {
		SMTPTimeoutSec = oldSMTPTimeout
		SMTPMaxConcurrent = oldSMTPMaxConcurrent
		IdemMaxEntries = oldMaxEntries
		HTTPBodyLimitBytes = oldBodyLimit
		HTTPReadTimeoutSec = oldReadTimeout
		HTTPWriteTimeoutSec = oldWriteTimeout
		HTTPIdleTimeoutSec = oldIdleTimeout
		ShutdownTimeoutSec = oldShutdownTimeout
	})

	SMTPTimeoutSec = 30
	SMTPMaxConcurrent = 7
	IdemMaxEntries = 123
	HTTPBodyLimitBytes = 4096
	HTTPReadTimeoutSec = 7
	HTTPWriteTimeoutSec = 50
	HTTPIdleTimeoutSec = 70
	ShutdownTimeoutSec = 55

	if got := SMTPMaxConcurrentSends(); got != 7 {
		t.Errorf("SMTPMaxConcurrentSends() = %d, want 7", got)
	}

	if got := IdempotencyMaxEntries(); got != 123 {
		t.Errorf("IdempotencyMaxEntries() = %d, want 123", got)
	}
	if got := HTTPBodyLimit(); got != 4096 {
		t.Errorf("HTTPBodyLimit() = %d, want 4096", got)
	}
	if got := HTTPReadTimeout(); got != 7*time.Second {
		t.Errorf("HTTPReadTimeout() = %v, want 7s", got)
	}
	if got := HTTPWriteTimeout(); got != 50*time.Second {
		t.Errorf("HTTPWriteTimeout() = %v, want 50s", got)
	}
	if got := HTTPIdleTimeout(); got != 70*time.Second {
		t.Errorf("HTTPIdleTimeout() = %v, want 70s", got)
	}
	if got := ShutdownTimeout(); got != 55*time.Second {
		t.Errorf("ShutdownTimeout() = %v, want 55s", got)
	}
}

func TestRuntimeLimitsUseSafeFallbacks(t *testing.T) {
	oldSMTPTimeout := SMTPTimeoutSec
	oldSMTPMaxConcurrent := SMTPMaxConcurrent
	oldMaxEntries := IdemMaxEntries
	oldBodyLimit := HTTPBodyLimitBytes
	oldReadTimeout := HTTPReadTimeoutSec
	oldWriteTimeout := HTTPWriteTimeoutSec
	oldIdleTimeout := HTTPIdleTimeoutSec
	oldShutdownTimeout := ShutdownTimeoutSec
	t.Cleanup(func() {
		SMTPTimeoutSec = oldSMTPTimeout
		SMTPMaxConcurrent = oldSMTPMaxConcurrent
		IdemMaxEntries = oldMaxEntries
		HTTPBodyLimitBytes = oldBodyLimit
		HTTPReadTimeoutSec = oldReadTimeout
		HTTPWriteTimeoutSec = oldWriteTimeout
		HTTPIdleTimeoutSec = oldIdleTimeout
		ShutdownTimeoutSec = oldShutdownTimeout
	})

	SMTPTimeoutSec = 0
	SMTPMaxConcurrent = 0
	IdemMaxEntries = 0
	HTTPBodyLimitBytes = -1
	HTTPReadTimeoutSec = 0
	HTTPWriteTimeoutSec = 1
	HTTPIdleTimeoutSec = 0
	ShutdownTimeoutSec = 1

	if got := SMTPTimeout(); got != 30*time.Second {
		t.Errorf("SMTPTimeout() = %v, want default 30s", got)
	}
	if got := SMTPMaxConcurrentSends(); got != 16 {
		t.Errorf("SMTPMaxConcurrentSends() = %d, want default 16", got)
	}
	if got := IdempotencyMaxEntries(); got != 10000 {
		t.Errorf("IdempotencyMaxEntries() = %d, want default 10000", got)
	}
	if got := HTTPBodyLimit(); got != 64*1024 {
		t.Errorf("HTTPBodyLimit() = %d, want default 65536", got)
	}
	if got := HTTPReadTimeout(); got != 10*time.Second {
		t.Errorf("HTTPReadTimeout() = %v, want default 10s", got)
	}
	if got := HTTPWriteTimeout(); got != 35*time.Second {
		t.Errorf("HTTPWriteTimeout() = %v, want SMTP timeout plus 5s", got)
	}
	if got := HTTPIdleTimeout(); got != 60*time.Second {
		t.Errorf("HTTPIdleTimeout() = %v, want default 60s", got)
	}
	if got := ShutdownTimeout(); got != 35*time.Second {
		t.Errorf("ShutdownTimeout() = %v, want SMTP timeout plus 5s", got)
	}
}
