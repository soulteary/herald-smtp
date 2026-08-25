package smtp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/soulteary/herald-smtp/internal/config"
	"github.com/soulteary/provider-kit"
)

func setTestConfig(t *testing.T) {
	t.Helper()
	oldHost, oldPort := config.SMTPHost, config.SMTPPort
	oldUser, oldPass := config.SMTPUser, config.SMTPPass
	oldFrom, oldFromName := config.SMTPFrom, config.SMTPFromName
	oldTLS, oldStartTLS := config.UseTLS, config.UseStartTLS
	oldSkipVerify, oldTimeout := config.SkipTLSVerify, config.SMTPTimeoutSec
	t.Cleanup(func() {
		config.SMTPHost, config.SMTPPort = oldHost, oldPort
		config.SMTPUser, config.SMTPPass = oldUser, oldPass
		config.SMTPFrom, config.SMTPFromName = oldFrom, oldFromName
		config.UseTLS, config.UseStartTLS = oldTLS, oldStartTLS
		config.SkipTLSVerify, config.SMTPTimeoutSec = oldSkipVerify, oldTimeout
	})
	config.SMTPHost = "localhost"
	config.SMTPPort = 25
	config.SMTPUser = ""
	config.SMTPPass = ""
	config.SMTPFrom = "sender@example.com"
	config.SMTPFromName = "Herald"
	config.UseTLS = false
	config.UseStartTLS = false
	config.SkipTLSVerify = false
	config.SMTPTimeoutSec = int((5 * time.Second) / time.Second)
}

func TestNewClient_InvalidConfig(t *testing.T) {
	setTestConfig(t)
	config.SMTPHost = ""
	client, err := NewClient()
	if err != nil {
		t.Errorf("NewClient() err = %v, want nil when config invalid", err)
	}
	if client != nil {
		t.Fatal("NewClient() client should be nil when config is incomplete")
	}
}

func TestNewClient_ValidConfig(t *testing.T) {
	setTestConfig(t)
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil || client.provider == nil {
		t.Fatal("NewClient() should initialize provider")
	}
}

func TestNewClient_RejectsConflictingTLSModes(t *testing.T) {
	setTestConfig(t)
	config.UseTLS = true
	config.UseStartTLS = true
	client, err := NewClient()
	if client != nil || err == nil {
		t.Fatalf("NewClient() = (%v, %v), want TLS mode conflict", client, err)
	}
	if reason, ok := provider.GetErrorReason(err); !ok || reason != provider.ReasonInvalidConfig {
		t.Errorf("NewClient() reason = %q, want invalid_config", reason)
	}
}

func TestNewClient_UsesProviderKitValidation(t *testing.T) {
	setTestConfig(t)
	config.SMTPFrom = "invalid-address"
	client, err := NewClient()
	if client != nil || err == nil {
		t.Fatalf("NewClient() = (%v, %v), want invalid sender error", client, err)
	}
}

func TestClient_Send_NilReceiver(t *testing.T) {
	var c *Client
	ctx := context.Background()
	msg := provider.NewMessage("test@example.com").WithBody("body")
	result, err := c.Send(ctx, msg)
	if err == nil {
		t.Fatal("Send with nil client: err = nil, want provider_down")
	}
	if result == nil || result.Error == nil || result.Error.Reason != provider.ReasonProviderDown {
		t.Errorf("Send with nil client: result = %#v, want provider_down", result)
	}
}

func TestClient_Send_NilMessage(t *testing.T) {
	// Client may be nil from NewClient when config invalid; Send already handles nil client
	var c *Client
	result, err := c.Send(context.Background(), nil)
	if err == nil {
		t.Fatal("Send(nil msg) with nil client: err = nil")
	}
	if result == nil || result.Error == nil || result.Error.Reason != provider.ReasonProviderDown {
		t.Errorf("Send(nil msg) with nil client: result = %#v", result)
	}
}

// TestClient_Send_NilProvider covers client non-nil but provider nil (e.g. NewClient failed after alloc).
func TestClient_Send_NilProvider(t *testing.T) {
	c := &Client{provider: nil}
	ctx := context.Background()
	msg := provider.NewMessage("test@example.com").WithBody("body")
	result, err := c.Send(ctx, msg)
	if err == nil {
		t.Fatal("Send with nil provider: err = nil, want provider_down")
	}
	if result == nil || result.Error == nil || result.Error.Reason != provider.ReasonProviderDown {
		t.Errorf("Send with nil provider: result = %#v, want provider_down", result)
	}
}

func TestClient_SendRejectsUnsafeMessageBeforeDial(t *testing.T) {
	setTestConfig(t)
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tests := []struct {
		name   string
		msg    *provider.Message
		reason provider.ErrorReason
	}{
		{name: "invalid recipient", msg: provider.NewMessage("not-an-email"), reason: provider.ReasonInvalidDestination},
		{name: "subject injection", msg: provider.NewMessage("to@example.com").WithSubject("hello\r\nBcc: victim@example.com"), reason: provider.ReasonValidationFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, sendErr := client.Send(context.Background(), tt.msg)
			if sendErr == nil {
				t.Fatal("Send() error = nil")
			}
			if result == nil || result.Error == nil || result.Error.Reason != tt.reason {
				t.Fatalf("Send() result = %#v, want reason %q", result, tt.reason)
			}
			if !errors.Is(sendErr, result.Error) {
				t.Errorf("Send() error = %v, want result error", sendErr)
			}
		})
	}
}
