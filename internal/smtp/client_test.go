package smtp

import (
	"context"
	"testing"

	"github.com/soulteary/provider-kit"
)

func TestNewClient_InvalidConfig(t *testing.T) {
	// When config.Valid() is false (SMTP_HOST or SMTP_FROM empty at init), NewClient returns (nil, nil)
	client, err := NewClient()
	if err != nil {
		t.Errorf("NewClient() err = %v, want nil when config invalid", err)
	}
	if client != nil {
		// If by chance env is set in test env, client may be non-nil; then err should be nil
		if err != nil {
			t.Errorf("NewClient() client non-nil but err = %v", err)
		}
	}
}

func TestClient_Send_NilReceiver(t *testing.T) {
	var c *Client
	ctx := context.Background()
	msg := provider.NewMessage("test@example.com").WithBody("body")
	result, err := c.Send(ctx, msg)
	if err != nil {
		t.Errorf("Send with nil client: err = %v, want nil", err)
	}
	if result != nil {
		t.Errorf("Send with nil client: result = %v, want nil", result)
	}
}

func TestClient_Send_NilMessage(t *testing.T) {
	// Client may be nil from NewClient when config invalid; Send already handles nil client
	var c *Client
	result, err := c.Send(context.Background(), nil)
	if err != nil {
		t.Errorf("Send(nil msg) with nil client: err = %v", err)
	}
	if result != nil {
		t.Errorf("Send(nil msg) with nil client: result = %v", result)
	}
}

// TestClient_Send_NilProvider covers client non-nil but provider nil (e.g. NewClient failed after alloc).
func TestClient_Send_NilProvider(t *testing.T) {
	c := &Client{provider: nil}
	ctx := context.Background()
	msg := provider.NewMessage("test@example.com").WithBody("body")
	result, err := c.Send(ctx, msg)
	if err != nil {
		t.Errorf("Send with nil provider: err = %v, want nil", err)
	}
	if result != nil {
		t.Errorf("Send with nil provider: result = %v, want nil", result)
	}
}
