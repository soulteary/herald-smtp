package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"net"
	"net/http"
	"os"
	"os/exec"
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
	config.Port = strings.TrimPrefix(address, "127.0.0.1:")
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

func TestMainShutdownTimeoutExitsNonzero(t *testing.T) {
	if os.Getenv("HERALD_SMTP_TEST_BLOCKED_SHUTDOWN") != "1" {
		args := []string{"-test.run=^TestMainShutdownTimeoutExitsNonzero$"}
		if coverage := flag.Lookup("test.gocoverdir"); coverage != nil && coverage.Value.String() != "" {
			args = append(args, "-test.gocoverdir="+coverage.Value.String())
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		cmd.Env = append(os.Environ(), "HERALD_SMTP_TEST_BLOCKED_SHUTDOWN=1")
		output, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		if ctx.Err() != nil || !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatalf("shutdown must exit 1: err=%v context=%v\n%s", err, ctx.Err(), output)
		}
		if strings.Contains(string(output), "--- FAIL:") {
			t.Fatalf("shutdown helper assertions failed: %s", output)
		}
		if !strings.Contains(string(output), "shutdown error") || !strings.Contains(string(output), "context deadline exceeded") {
			t.Fatalf("missing shutdown deadline diagnostic: %s", output)
		}
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	config.Port = strings.TrimPrefix(address, "127.0.0.1:")
	config.SMTPHost, config.SMTPFrom = "", ""
	config.SMTPTimeoutSec = 1
	config.ShutdownTimeoutSec = 6
	config.HTTPReadTimeoutSec = 60
	done := make(chan struct{})
	go func() { main(); close(done) }()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := client.Get("http://" + address + "/healthz")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatal("unhealthy")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	_, err = connection.Write([]byte("POST /v1/send HTTP/1.1\r\nHost: fixture\r\nContent-Length: 10\r\nExpect: 100-continue\r\n\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodPost})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 100 {
		t.Fatalf("continue status=%d", response.StatusCode)
	}
	_ = response.Body.Close()
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	// The accepted request is waiting for its body beyond the shutdown budget.
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown deadline not enforced")
	}
}
