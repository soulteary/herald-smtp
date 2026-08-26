package smtp

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/herald-smtp/internal/config"
	"github.com/soulteary/provider-kit"
)

type smtpTestServer struct {
	listener net.Listener
	messages chan string
	errors   chan error
	reject   bool
}

func startSMTPTestServer(t *testing.T, rejectRecipient bool) *smtpTestServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &smtpTestServer{
		listener: listener,
		messages: make(chan string, 1),
		errors:   make(chan error, 1),
		reject:   rejectRecipient,
	}
	t.Cleanup(func() { _ = listener.Close() })
	go server.serve()
	return server
}

func (s *smtpTestServer) address() (string, int) {
	address := s.listener.Addr().(*net.TCPAddr)
	return address.IP.String(), address.Port
}

func (s *smtpTestServer) serve() {
	conn, err := s.listener.Accept()
	if err != nil {
		if !strings.Contains(err.Error(), "closed network connection") {
			s.errors <- err
		}
		return
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	reader := bufio.NewScanner(conn)
	writer := bufio.NewWriter(conn)
	writeResponse := func(response string) bool {
		if _, err := writer.WriteString(response); err != nil {
			s.errors <- err
			return false
		}
		if err := writer.Flush(); err != nil {
			s.errors <- err
			return false
		}
		return true
	}
	if !writeResponse("220 localhost ESMTP test server\r\n") {
		return
	}

	var message strings.Builder
	inData := false
	for reader.Scan() {
		line := reader.Text()
		if inData {
			if line == "." {
				inData = false
				s.messages <- message.String()
				if !writeResponse("250 2.0.0 queued\r\n") {
					return
				}
				continue
			}
			if strings.HasPrefix(line, "..") {
				line = line[1:]
			}
			message.WriteString(line)
			message.WriteString("\r\n")
			continue
		}

		command := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(command, "EHLO "):
			if !writeResponse("250-localhost\r\n250 8BITMIME\r\n") {
				return
			}
		case strings.HasPrefix(command, "HELO "), strings.HasPrefix(command, "MAIL FROM:"):
			if !writeResponse("250 2.1.0 OK\r\n") {
				return
			}
		case strings.HasPrefix(command, "RCPT TO:"):
			if s.reject {
				if !writeResponse("550 5.1.1 recipient rejected\r\n") {
					return
				}
			} else if !writeResponse("250 2.1.5 OK\r\n") {
				return
			}
		case command == "DATA":
			inData = true
			if !writeResponse("354 end with <CRLF>.<CRLF>\r\n") {
				return
			}
		case command == "QUIT":
			_ = writeResponse("221 2.0.0 bye\r\n")
			return
		default:
			if !writeResponse("502 5.5.1 command not implemented\r\n") {
				return
			}
		}
	}
	if err := reader.Err(); err != nil {
		s.errors <- err
	}
}

func configureSMTPTestClient(t *testing.T, server *smtpTestServer) *Client {
	t.Helper()
	setTestConfig(t)
	host, port := server.address()
	config.SMTPHost = host
	config.SMTPPort = port
	config.UseTLS = false
	config.UseStartTLS = false
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestClientSMTPProtocolIntegration(t *testing.T) {
	server := startSMTPTestServer(t, false)
	client := configureSMTPTestClient(t, server)
	message := provider.NewMessage("recipient@example.com").
		WithSubject("Protocol integration").
		WithBody("hello from herald-smtp")

	result, err := client.Send(context.Background(), message)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result == nil || !result.OK || result.MessageID == "" {
		t.Fatalf("Send() result = %#v, want successful result", result)
	}

	select {
	case wireMessage := <-server.messages:
		for _, expected := range []string{
			"From: Herald <sender@example.com>",
			"To: recipient@example.com",
			"Content-Type: text/plain; charset=UTF-8",
			"hello from herald-smtp",
		} {
			if !strings.Contains(wireMessage, expected) {
				t.Errorf("wire message does not contain %q:\n%s", expected, wireMessage)
			}
		}
	case err := <-server.errors:
		t.Fatalf("SMTP test server error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP message")
	}
}

func TestClientSMTPProtocolIntegrationRejectsRecipient(t *testing.T) {
	server := startSMTPTestServer(t, true)
	client := configureSMTPTestClient(t, server)

	result, err := client.Send(context.Background(), provider.NewMessage("rejected@example.com").WithBody("body"))
	if err == nil {
		t.Fatal("Send() error = nil, want SMTP rejection")
	}
	if result == nil || result.OK || result.Error == nil {
		t.Fatalf("Send() result = %#v, want failure result", result)
	}
	if result.Error.Reason != provider.ReasonSendFailed {
		t.Fatalf("Send() reason = %q, want send_failed", result.Error.Reason)
	}

	select {
	case message := <-server.messages:
		t.Fatalf("unexpected delivered message: %s", message)
	case serverErr := <-server.errors:
		if !strings.Contains(serverErr.Error(), "closed network connection") {
			t.Fatalf("SMTP test server error: %v", serverErr)
		}
	case <-time.After(50 * time.Millisecond):
	}
}
