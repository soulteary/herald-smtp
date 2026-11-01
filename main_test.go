package main

import (
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
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

// Exercise the real entrypoint, including signal-driven graceful shutdown.
func TestMainLifecycle(t *testing.T) {
	oldPort, oldHost, oldFrom := config.Port, config.SMTPHost, config.SMTPFrom
	t.Cleanup(func() { config.Port, config.SMTPHost, config.SMTPFrom = oldPort, oldHost, oldFrom })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	config.Port = strings.TrimPrefix(address, "127.0.0.1")
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	config.SMTPHost, config.SMTPFrom = "", ""
	done := make(chan struct{})
	go func() { main(); close(done) }()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := client.Get("http://" + address + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("health status = %d", resp.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
	if resp, err := client.Get("http://" + address + "/healthz"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("server accepts requests after shutdown")
	}
}
