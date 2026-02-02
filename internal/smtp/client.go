package smtp

import (
	"context"

	"github.com/soulteary/herald-smtp/internal/config"
	"github.com/soulteary/provider-kit"
)

// Client wraps provider-kit SMTP provider for sending email via HTTP /v1/send.
type Client struct {
	provider provider.Provider
}

// NewClient creates a client from config. Returns nil if config is invalid.
func NewClient() (*Client, error) {
	if !config.Valid() {
		return nil, nil
	}
	cfg := &provider.SMTPConfig{
		Host:        config.SMTPHost,
		Port:        config.SMTPPort,
		Username:    config.SMTPUser,
		Password:    config.SMTPPass,
		From:        config.SMTPFrom,
		UseStartTLS: config.UseStartTLS,
		Timeout:     config.SMTPTimeout(),
	}
	p, err := provider.NewSMTPProvider(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{provider: p}, nil
}

// Send sends an email using provider-kit Message; returns provider-kit SendResult and error.
func (c *Client) Send(ctx context.Context, msg *provider.Message) (*provider.SendResult, error) {
	if c == nil || c.provider == nil {
		return nil, nil
	}
	return c.provider.Send(ctx, msg)
}
