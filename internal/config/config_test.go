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
	got := SMTPTimeout()
	if got != 30*time.Second {
		t.Errorf("SMTPTimeout() = %v, want 30s", got)
	}
}
