package service

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"math/rand"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	texttemplate "text/template"
	"time"
	"unicode/utf8"

	"github.com/resend/resend-go/v2"
)

type fakeSMTPAuthClient struct {
	authErrs   []error
	authCalls  []smtp.Auth
	authLine   string
	textClient *textproto.Conn
}

func (f *fakeSMTPAuthClient) Auth(auth smtp.Auth) error {
	f.authCalls = append(f.authCalls, auth)
	if len(f.authErrs) == 0 {
		return nil
	}
	err := f.authErrs[0]
	f.authErrs = f.authErrs[1:]
	return err
}

func (f *fakeSMTPAuthClient) Text() *textproto.Conn {
	return f.textClient
}

func (f *fakeSMTPAuthClient) Extension(name string) (bool, string) {
	if strings.EqualFold(name, "AUTH") && f.authLine != "" {
		return true, f.authLine
	}
	return false, ""
}

func TestSMTPAuthWithFallback_UsesPlainWhenAccepted(t *testing.T) {
	client := &fakeSMTPAuthClient{}
	fallback, err := smtpAuthWithFallback(client, "smtp.office365.com", "user", "pass")
	if err != nil {
		t.Fatalf("smtpAuthWithFallback returned error: %v", err)
	}
	if fallback {
		t.Fatalf("expected no fallback when PLAIN auth succeeds")
	}
	if len(client.authCalls) != 1 {
		t.Fatalf("expected 1 auth call, got %d", len(client.authCalls))
	}
	if _, ok := client.authCalls[0].(*loginAuth); ok {
		t.Fatalf("expected first auth to be PLAIN, got LOGIN")
	}
}

func TestSMTPAuthWithFallback_FallsBackToLoginOnOffice365Style504(t *testing.T) {
	client := &fakeSMTPAuthClient{
		authErrs: []error{
			errors.New("504 5.7.4 Unrecognized authentication type"),
			nil,
		},
		authLine: "XOAUTH2 LOGIN",
	}
	fallback, err := smtpAuthWithFallback(client, "smtp.office365.com", "user", "pass")
	if !fallback {
		t.Fatalf("expected fallback signal when Office 365 rejects PLAIN auth")
	}
	if err == nil {
		t.Fatalf("expected original PLAIN auth error to be returned for reconnect path")
	}
	if len(client.authCalls) != 1 {
		t.Fatalf("expected 1 auth call before reconnect, got %d", len(client.authCalls))
	}
	if _, ok := client.authCalls[0].(*loginAuth); ok {
		t.Fatalf("expected first auth attempt to remain PLAIN")
	}
}

func TestSMTPAuthWithFallback_DoesNotFallbackWithoutLoginSupport(t *testing.T) {
	wantErr := errors.New("504 5.7.4 Unrecognized authentication type")
	client := &fakeSMTPAuthClient{
		authErrs: []error{wantErr},
		authLine: "XOAUTH2",
	}
	fallback, err := smtpAuthWithFallback(client, "smtp.office365.com", "user", "pass")
	if fallback {
		t.Fatalf("did not expect fallback when server does not advertise LOGIN")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected original error, got %v", err)
	}
	if len(client.authCalls) != 1 {
		t.Fatalf("expected 1 auth call, got %d", len(client.authCalls))
	}
}

func TestSanitizeSubjectField(t *testing.T) {
	long := strings.Repeat("a", 100)
	longRunes := strings.Repeat("深", 100)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain ascii", "Acme", "Acme"},
		{"strips newline", "Acme\nEvil", "AcmeEvil"},
		{"strips crlf header-style", "Acme\r\nBcc: evil@example.com", "AcmeBcc: evil@example.com"},
		{"strips tab", "Acme\tTeam", "AcmeTeam"},
		{"strips unicode control", "Acme\x07Beep", "AcmeBeep"},
		{"preserves non-ascii", "深度学习工作区", "深度学习工作区"},
		{"preserves emoji", "Team 🚀", "Team 🚀"},
		{"truncates long ascii", long, strings.Repeat("a", maxSubjectFieldRunes-1) + "…"},
		{"truncates rune-aware", longRunes, strings.Repeat("深", maxSubjectFieldRunes-1) + "…"},
		{"empty stays empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSubjectField(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeSubjectField(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNewEmailService_TLSMode(t *testing.T) {
	tests := []struct {
		name         string
		smtpTLS      string
		smtpPort     string
		wantImplicit bool
	}{
		{"unset on 465 auto-enables implicit", "", "465", true},
		{"unset on 587 stays starttls", "", "587", false},
		{"unset default port stays starttls", "", "", false},
		{"explicit implicit on 587 forces SMTPS", "implicit", "587", true},
		{"smtps alias", "smtps", "587", true},
		{"ssl alias", "ssl", "587", true},
		{"explicit starttls on 465 overrides auto-detect", "starttls", "465", false},
		{"case-insensitive", "IMPLICIT", "587", true},
		{"trims whitespace", "  implicit  ", "587", true},
		{"unknown value falls back to starttls", "tls", "465", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Isolate from any ambient mail config so only SMTP_TLS/SMTP_PORT drive the result.
			t.Setenv("RESEND_API_KEY", "")
			t.Setenv("SMTP_HOST", "smtp.example.com")
			t.Setenv("SMTP_PORT", tt.smtpPort)
			t.Setenv("SMTP_TLS", tt.smtpTLS)

			s := NewEmailService()
			if s.smtpTLSImplicit != tt.wantImplicit {
				t.Errorf("SMTP_TLS=%q SMTP_PORT=%q: smtpTLSImplicit = %v, want %v",
					tt.smtpTLS, tt.smtpPort, s.smtpTLSImplicit, tt.wantImplicit)
			}
		})
	}
}

func TestNewEmailService_EHLOName(t *testing.T) {
	tests := []struct {
		name    string
		ehloEnv string
		want    string // when fromEnv is false, the os.Hostname() fallback is expected instead
		fromEnv bool
	}{
		{"explicit name used verbatim", "mail.example.com", "mail.example.com", true},
		{"explicit name is trimmed", "  mail.example.com  ", "mail.example.com", true},
		{"unset falls back to hostname", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Isolate from ambient mail config so only SMTP_EHLO_NAME drives the result.
			t.Setenv("RESEND_API_KEY", "")
			t.Setenv("SMTP_HOST", "smtp.example.com")
			t.Setenv("SMTP_EHLO_NAME", tt.ehloEnv)

			s := NewEmailService()
			if tt.fromEnv {
				if s.smtpEHLOName != tt.want {
					t.Errorf("SMTP_EHLO_NAME=%q: smtpEHLOName = %q, want %q", tt.ehloEnv, s.smtpEHLOName, tt.want)
				}
				return
			}
			// Unset: must mirror os.Hostname() exactly — including the empty result if
			// Hostname() errors, which makes sendSMTP skip the EHLO override.
			want, _ := os.Hostname()
			if s.smtpEHLOName != want {
				t.Errorf("SMTP_EHLO_NAME unset: smtpEHLOName = %q, want os.Hostname() %q", s.smtpEHLOName, want)
			}
		})
	}
}

func TestNewEmailService_FromEmailResolution(t *testing.T) {
	tests := []struct {
		name          string
		smtpHost      string
		smtpUsername  string
		smtpFromEmail string
		resendFrom    string
		want          string
	}{
		{
			name:       "resend mode uses resend from",
			resendFrom: "resend@example.com",
			want:       "resend@example.com",
		},
		{
			name:          "smtp mode prefers smtp from",
			smtpHost:      "smtp.example.com",
			smtpUsername:  "auth@example.com",
			smtpFromEmail: "sender@example.com",
			resendFrom:    "resend@example.com",
			want:          "sender@example.com",
		},
		{
			name:         "smtp mode falls back to resend from",
			smtpHost:     "smtp.example.com",
			smtpUsername: "auth@example.com",
			resendFrom:   "resend@example.com",
			want:         "resend@example.com",
		},
		{
			name: "default",
			want: "noreply@multica.ai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RESEND_API_KEY", "")
			t.Setenv("SMTP_HOST", tt.smtpHost)
			t.Setenv("SMTP_USERNAME", tt.smtpUsername)
			t.Setenv("SMTP_FROM_EMAIL", tt.smtpFromEmail)
			t.Setenv("RESEND_FROM_EMAIL", tt.resendFrom)

			s := NewEmailService()
			if s.fromEmail != tt.want {
				t.Fatalf("fromEmail = %q, want %q", s.fromEmail, tt.want)
			}
		})
	}
}

func TestSendSMTPRequiresConfiguredFromEmail(t *testing.T) {
	s := &EmailService{
		smtpHost:     "127.0.0.1",
		smtpPort:     "1",
		smtpUsername: "auth@example.com",
		smtpPassword: "testpass",
	}

	err := s.sendSMTP("to@example.com", "Test Subject", "<p>Hello</p>")
	if err == nil {
		t.Fatal("expected missing from email error")
	}
	if got := err.Error(); got != "SMTP_FROM_EMAIL or RESEND_FROM_EMAIL is required when SMTP_HOST is set" {
		t.Fatalf("error = %q, want missing from email error", got)
	}
}

func TestBuildInvitationParams_EscapesHTMLInBody(t *testing.T) {
	tests := []struct {
		name          string
		inviter       string
		workspace     string
		wantInBody    []string
		wantNotInBody []string
	}{
		{
			name:      "escapes script tag in inviter",
			inviter:   "<script>alert(1)</script>",
			workspace: "Acme",
			wantInBody: []string{
				"&lt;script&gt;alert(1)&lt;/script&gt;",
			},
			wantNotInBody: []string{
				"<script>alert(1)</script>",
			},
		},
		{
			name:      "escapes attribute-break payload in inviter",
			inviter:   `Alice" onclick="evil()`,
			workspace: "Acme",
			wantNotInBody: []string{
				`Alice" onclick="evil()`,
			},
		},
		{
			name:      "escapes anchor tag in workspace",
			inviter:   "Alice",
			workspace: `<a href="https://evil.example">Click</a>`,
			wantInBody: []string{
				"&lt;a href=",
				"&gt;Click&lt;/a&gt;",
			},
			wantNotInBody: []string{
				`<a href="https://evil.example">Click</a>`,
			},
		},
		{
			name:      "benign text unchanged",
			inviter:   "Alice",
			workspace: "Acme",
			wantInBody: []string{
				"Alice",
				"Acme",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := buildInvitationParams(
				"noreply@multica.ai",
				"invitee@example.com",
				tt.inviter,
				tt.workspace,
				"https://multica.ai/invite/abc-123",
			)
			for _, needle := range tt.wantInBody {
				if !strings.Contains(p.Html, needle) {
					t.Errorf("body missing %q\nbody: %s", needle, p.Html)
				}
			}
			for _, needle := range tt.wantNotInBody {
				if strings.Contains(p.Html, needle) {
					t.Errorf("body should not contain raw %q\nbody: %s", needle, p.Html)
				}
			}
		})
	}
}

func TestBuildInvitationParams_SubjectStripsControls(t *testing.T) {
	p := buildInvitationParams(
		"noreply@multica.ai",
		"invitee@example.com",
		"Alice\r\n",
		"Acme\t",
		"https://multica.ai/invite/abc",
	)
	if strings.ContainsAny(p.Subject, "\r\n\t") {
		t.Errorf("subject still contains control characters: %q", p.Subject)
	}
	if p.Subject != "Alice invited you to Acme on Multica" {
		t.Errorf("unexpected subject: %q", p.Subject)
	}
}

func TestBuildInvitationParams_SubjectNotHTMLEscaped(t *testing.T) {
	// Subject is not HTML-rendered; entities would render literally in inboxes.
	p := buildInvitationParams(
		"noreply@multica.ai",
		"invitee@example.com",
		"Alice",
		"Acme & Co.",
		"https://multica.ai/invite/abc",
	)
	if strings.Contains(p.Subject, "&amp;") {
		t.Errorf("subject should not be HTML-escaped, got %q", p.Subject)
	}
	if !strings.Contains(p.Subject, "Acme & Co.") {
		t.Errorf("subject missing literal ampersand: %q", p.Subject)
	}
}

func TestBuildInvitationParams_SubjectTruncated(t *testing.T) {
	longWorkspace := strings.Repeat("A", 200)
	p := buildInvitationParams(
		"noreply@multica.ai",
		"invitee@example.com",
		"Alice",
		longWorkspace,
		"https://multica.ai/invite/abc",
	)
	// Template: "Alice invited you to <ws> on Multica"
	// ws is capped at maxSubjectFieldRunes; overall subject should also be bounded.
	maxExpected := len("Alice invited you to  on Multica") + maxSubjectFieldRunes
	if runes := len([]rune(p.Subject)); runes > maxExpected {
		t.Errorf("subject not bounded: %d runes, max %d: %q", runes, maxExpected, p.Subject)
	}
	if !strings.Contains(p.Subject, "…") {
		t.Errorf("truncated subject should contain ellipsis marker: %q", p.Subject)
	}
}

func TestBuildInvitationParams_ToAndFromPassedThrough(t *testing.T) {
	p := buildInvitationParams(
		"noreply@multica.ai",
		"invitee@example.com",
		"Alice",
		"Acme",
		"https://multica.ai/invite/abc",
	)
	if p.From != "noreply@multica.ai" {
		t.Errorf("From = %q", p.From)
	}
	if len(p.To) != 1 || p.To[0] != "invitee@example.com" {
		t.Errorf("To = %v", p.To)
	}
	if !strings.Contains(p.Html, "https://multica.ai/invite/abc") {
		t.Errorf("body missing invite URL: %s", p.Html)
	}
}

// --- loginAuth.Start security tests ---

func TestLoginAuth_Start_RefusesUnencryptedRemote(t *testing.T) {
	auth := &loginAuth{username: "user", password: "pass", host: "smtp.office365.com"}
	_, _, err := auth.Start(&smtp.ServerInfo{
		Name: "smtp.office365.com",
		TLS:  false,
	})
	if err == nil {
		t.Fatal("expected error for unencrypted remote connection")
	}
	if !strings.Contains(err.Error(), "unencrypted connection") {
		t.Errorf("expected 'unencrypted connection' error, got: %v", err)
	}
}

func TestLoginAuth_Start_AllowsTLS(t *testing.T) {
	auth := &loginAuth{username: "user", password: "pass", host: "smtp.office365.com"}
	_, _, err := auth.Start(&smtp.ServerInfo{
		Name: "smtp.office365.com",
		TLS:  true,
	})
	if err != nil {
		t.Fatalf("expected no error for TLS connection, got: %v", err)
	}
}

func TestLoginAuth_Start_AllowsLocalhost(t *testing.T) {
	auth := &loginAuth{username: "user", password: "pass", host: "localhost"}
	_, _, err := auth.Start(&smtp.ServerInfo{
		Name: "localhost",
		TLS:  false,
	})
	if err != nil {
		t.Fatalf("expected no error for localhost connection, got: %v", err)
	}
}

func TestLoginAuth_Start_RejectsWrongHost(t *testing.T) {
	auth := &loginAuth{username: "user", password: "pass", host: "smtp.office365.com"}
	_, _, err := auth.Start(&smtp.ServerInfo{
		Name: "evil-relay.example.com",
		TLS:  true,
	})
	if err == nil {
		t.Fatal("expected error for host mismatch")
	}
	if !strings.Contains(err.Error(), "wrong host name") {
		t.Errorf("expected 'wrong host name' error, got: %v", err)
	}
}

func TestLoginAuth_Start_AllowsLoopbackIPs(t *testing.T) {
	for _, name := range []string{"127.0.0.1", "::1"} {
		auth := &loginAuth{username: "user", password: "pass", host: name}
		_, _, err := auth.Start(&smtp.ServerInfo{
			Name: name,
			TLS:  false,
		})
		if err != nil {
			t.Errorf("expected no error for %s, got: %v", name, err)
		}
	}
}

// --- sendSMTP no panic on openSMTPClient failure ---

func TestSendSMTP_OpenClientFailureNoPanic(t *testing.T) {
	s := &EmailService{
		fromEmail:    "from@example.com",
		smtpHost:     "255.255.255.255", // unroutable, will time out or fail
		smtpPort:     "25",
		smtpUsername: "user",
		smtpPassword: "pass",
	}
	err := s.sendSMTP("to@example.com", "Subject", "<p>body</p>")
	if err == nil {
		t.Fatal("expected error from unreachable SMTP server")
	}
	// The important assertion: we reached here without panicking.
	t.Logf("sendSMTP correctly returned error: %v", err)
}

// --- Full sendSMTP flow tests with a mock SMTP server ---

// testSMTPServer is a minimal SMTP server that can simulate Office 365-style
// PLAIN auth rejection followed by LOGIN auth acceptance.
type testSMTPServer struct {
	Listener net.Listener
	Addr     string

	CapturedEnvelope chan string
	CapturedData     chan []byte

	// Auth mechs advertised in EHLO response (e.g. "LOGIN" or "PLAIN LOGIN")
	AuthMechs string
	// If true, AUTH PLAIN returns 504; otherwise it succeeds
	RejectPlain  bool
	ExpectedUser string
	ExpectedPass string
	// If true, advertise STARTTLS in EHLO
	AdvertiseSTARTTLS bool
	// If true, advertise 8BITMIME in EHLO
	Advertise8BITMIME bool
}

func startTestSMTPServer(t *testing.T, cfg testSMTPServer) (*testSMTPServer, func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	cfg.Listener = l
	cfg.Addr = l.Addr().String()
	if cfg.CapturedEnvelope == nil {
		cfg.CapturedEnvelope = make(chan string, 100)
	}
	if cfg.CapturedData == nil {
		cfg.CapturedData = make(chan []byte, 100)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go cfg.handleConn(conn)
		}
	}()

	cleanup := func() {
		l.Close()
		<-done
	}
	return &cfg, cleanup
}

func (s *testSMTPServer) handleConn(conn net.Conn) {
	defer conn.Close()

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	writeLine := func(format string, args ...interface{}) {
		fmt.Fprintf(rw, format+"\r\n", args...)
		rw.Flush()
	}
	readLine := func() string {
		line, err := rw.ReadString('\n')
		if err != nil {
			return ""
		}
		return strings.TrimRight(line, "\r\n")
	}

	writeLine("220 test-smtp ESMTP")

	// Wait for EHLO
	ehloLine := readLine()
	if !strings.HasPrefix(strings.ToUpper(ehloLine), "EHLO") {
		writeLine("500 unrecognized command")
		return
	}

	// Build EHLO response
	writeLine("250-test-smtp Hello")
	if s.AdvertiseSTARTTLS {
		writeLine("250-STARTTLS")
	}
	if s.Advertise8BITMIME {
		writeLine("250-8BITMIME")
	}
	if s.AuthMechs != "" {
		writeLine("250-AUTH " + s.AuthMechs)
	}
	writeLine("250 OK")

	// Read commands until QUIT
	for {
		line := readLine()
		if line == "" {
			return
		}

		upper := strings.ToUpper(line)

		switch {
		case strings.HasPrefix(upper, "AUTH PLAIN") || strings.HasPrefix(upper, "AUTH PLAIN "):
			if s.RejectPlain {
				writeLine("504 5.7.4 Unrecognized authentication type")
				continue
			}
			writeLine("235 2.7.0 Auth succeeded")

		case strings.HasPrefix(upper, "AUTH LOGIN"):
			writeLine("334 VXNlcm5hbWU6") // base64("Username:")
			userLine := readLine()
			userBytes, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(userLine))
			writeLine("334 UGFzc3dvcmQ6") // base64("Password:")
			passLine := readLine()
			passBytes, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(passLine))

			if string(userBytes) == s.ExpectedUser && string(passBytes) == s.ExpectedPass {
				writeLine("235 2.7.0 Auth succeeded")
			} else {
				writeLine("535 5.7.8 Auth failed")
			}

		case strings.HasPrefix(upper, "MAIL FROM:"):
			s.CapturedEnvelope <- line
			writeLine("250 OK")

		case strings.HasPrefix(upper, "RCPT TO:"):
			writeLine("250 OK")

		case upper == "DATA":
			writeLine("354 Start mail input; end with <CRLF>.<CRLF>")
			data, err := textproto.NewReader(rw.Reader).ReadDotBytes()
			if err != nil {
				return
			}
			s.CapturedData <- data
			writeLine("250 OK")

		case strings.HasPrefix(upper, "STARTTLS"):
			writeLine("220 Ready to start TLS")

		case strings.HasPrefix(upper, "QUIT"):
			writeLine("221 bye")
			return

		default:
			writeLine("500 unrecognized command")
		}
	}
}

func TestSendSMTP_FallbackReconnectsAndAuthsWithLOGIN(t *testing.T) {
	srv, cleanup := startTestSMTPServer(t, testSMTPServer{
		AuthMechs:    "PLAIN LOGIN",
		RejectPlain:  true,
		ExpectedUser: "testuser",
		ExpectedPass: "testpass",
	})
	defer cleanup()
	host, port, _ := net.SplitHostPort(srv.Addr)

	s := &EmailService{
		fromEmail:    "from@example.com",
		smtpHost:     host,
		smtpPort:     port,
		smtpUsername: "testuser",
		smtpPassword: "testpass",
	}
	// smtpEHLOName is empty so net/smtp defaults to "localhost", which the
	// test server accepts. No STARTTLS advertised → plain connection to
	// localhost, which loginAuth.Start allows.

	err := s.sendSMTP("to@example.com", "Test Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("sendSMTP failed: %v", err)
	}
}

func TestSendSMTP_PlainAuthSucceedsWithoutFallback(t *testing.T) {
	srv, cleanup := startTestSMTPServer(t, testSMTPServer{
		AuthMechs:    "PLAIN LOGIN",
		RejectPlain:  false, // PLAIN succeeds
		ExpectedUser: "testuser",
		ExpectedPass: "testpass",
	})
	defer cleanup()
	host, port, _ := net.SplitHostPort(srv.Addr)

	s := &EmailService{
		fromEmail:    "from@example.com",
		smtpHost:     host,
		smtpPort:     port,
		smtpUsername: "testuser",
		smtpPassword: "testpass",
	}

	err := s.sendSMTP("to@example.com", "Test Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("sendSMTP failed: %v", err)
	}
}

func TestSendSMTP_NoAuthWhenUsernameEmpty(t *testing.T) {
	srv, cleanup := startTestSMTPServer(t, testSMTPServer{
		AuthMechs: "PLAIN LOGIN",
	})
	defer cleanup()
	host, port, _ := net.SplitHostPort(srv.Addr)

	s := &EmailService{
		fromEmail: "from@example.com",
		smtpHost:  host,
		smtpPort:  port,
		// smtpUsername is empty → unauthenticated relay
	}

	err := s.sendSMTP("to@example.com", "Test Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("sendSMTP failed for unauthenticated relay: %v", err)
	}
}

func TestSendSMTP_LoginAuthRejectsUnencryptedRemote(t *testing.T) {
	// Simulate a remote server that advertises LOGIN but not STARTTLS.
	// Since the connection is not TLS and not localhost, loginAuth.Start
	// must refuse to send credentials.
	auth := &loginAuth{
		username: "user",
		password: "pass",
		host:     "smtp.remote.example.com",
	}
	_, _, err := auth.Start(&smtp.ServerInfo{
		Name: "smtp.remote.example.com",
		TLS:  false,
	})
	if err == nil {
		t.Fatal("expected error: LOGIN auth on unencrypted remote connection")
	}
	if !strings.Contains(err.Error(), "unencrypted connection") {
		t.Errorf("expected 'unencrypted connection' error, got: %v", err)
	}
}

func TestEmailTemplateDataContract(t *testing.T) {
	// This contract is intentionally closed. Adding a field must first revisit
	// the field-admission rule; do not merely update the expected field list.
	tests := []struct {
		name       string
		value      any
		wantFields []string
	}{
		{
			name:       "verification",
			value:      verificationTemplateData{},
			wantFields: []string{"Code", "ExpiresInMinutes", "AppName"},
		},
		{
			name:       "invitation",
			value:      invitationTemplateData{},
			wantFields: []string{"InviterName", "WorkspaceName", "InviteURL", "AppName", "AppURL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := reflect.TypeOf(tt.value)
			if typ.NumField() != len(tt.wantFields) {
				t.Fatalf("fields = %d, want exactly %d; adding template data requires re-evaluating the frozen field-admission rule", typ.NumField(), len(tt.wantFields))
			}
			for i, want := range tt.wantFields {
				field := typ.Field(i)
				if field.Name != want {
					t.Errorf("field %d = %q, want %q", i, field.Name, want)
				}
				if field.Type.Kind() != reflect.String {
					t.Errorf("field %s kind = %s, want string", field.Name, field.Type.Kind())
				}
				for _, unsafeType := range []reflect.Type{
					reflect.TypeOf(htmltemplate.HTML("")),
					reflect.TypeOf(htmltemplate.URL("")),
					reflect.TypeOf(htmltemplate.JS("")),
					reflect.TypeOf(htmltemplate.CSS("")),
					reflect.TypeOf(htmltemplate.Srcset("")),
				} {
					if field.Type == unsafeType {
						t.Errorf("field %s bypasses contextual escaping with %s", field.Name, field.Type)
					}
				}
			}
		})
	}
}

func TestSanitizeRenderedSubjectBounds(t *testing.T) {
	if got := sanitizeRenderedSubject(strings.Repeat("a", 300)); got != strings.Repeat("a", 200) {
		t.Fatalf("300 ASCII runes sanitized to %d runes, want 200", utf8.RuneCountInString(got))
	}

	cjk := sanitizeRenderedSubject(strings.Repeat("中", 200))
	if got := utf8.RuneCountInString(cjk); got != 91 {
		t.Fatalf("200 CJK runes sanitized to %d runes, want 91", got)
	}
	if got := subjectLine(cjk); got != 996 {
		t.Fatalf("CJK Subject line = %d octets, want 996", got)
	}
	if got := subjectLine(strings.Repeat("中", 92)); got != 1018 {
		t.Fatalf("92-rune CJK Subject line = %d octets, want 1018", got)
	}
	if got := subjectLine(strings.Repeat("a", 200)); got != 209 {
		t.Fatalf("200-rune ASCII Subject line = %d octets, want 209", got)
	}
}

func TestSubjectSanitizersStripTheSameControlCodePoints(t *testing.T) {
	controls := []rune{'\u0000', '\u0009', '\u000a', '\u000b', '\u000c', '\u000d', '\u007f', '\u0085', '\u009f'}
	for _, control := range controls {
		t.Run(fmt.Sprintf("U+%04X", control), func(t *testing.T) {
			in := "before" + string(control) + "after"
			want := "beforeafter"
			if got := sanitizeSubjectField(in); got != want {
				t.Errorf("sanitizeSubjectField = %q, want %q", got, want)
			}
			if got := sanitizeRenderedSubject(in); got != want {
				t.Errorf("sanitizeRenderedSubject = %q, want %q", got, want)
			}
		})
	}
}

func TestSanitizeRenderedSubjectProperties(t *testing.T) {
	characterSets := [][]rune{
		[]rune("abc XYZ 019&"),
		[]rune("中文漢字界語"),
		[]rune("😀🚀🧪🌍"),
		[]rune("éøßñç"),
		[]rune("a 中😀é "),
	}
	rng := rand.New(rand.NewSource(1))
	for setIndex, alphabet := range characterSets {
		for sample := 0; sample < 4000; sample++ {
			runes := make([]rune, rng.Intn(maxRenderedSubjectRunes+1))
			for i := range runes {
				runes[i] = alphabet[rng.Intn(len(alphabet))]
			}
			in := string(runes)
			out := sanitizeRenderedSubject(in)
			if subjectLine(out) > maxSubjectLineOctets {
				t.Fatalf("set %d sample %d: sanitized line is %d octets", setIndex, sample, subjectLine(out))
			}
			if !strings.HasPrefix(in, out) {
				t.Fatalf("set %d sample %d: output is not a rune prefix", setIndex, sample)
			}
			if again := sanitizeRenderedSubject(out); again != out {
				t.Fatalf("set %d sample %d: sanitizer is not idempotent", setIndex, sample)
			}
			if subjectLine(in) <= maxSubjectLineOctets && out != in {
				t.Fatalf("set %d sample %d: sanitizer changed an already valid subject", setIndex, sample)
			}
			if out != in {
				next := runes[utf8.RuneCountInString(out)]
				if subjectLine(out+string(next)) <= maxSubjectLineOctets {
					t.Fatalf("set %d sample %d: output is not maximal", setIndex, sample)
				}
			}
		}
	}
}

func TestSanitizeRenderedSubjectExactBoundary(t *testing.T) {
	var exact string
	for accented := 0; accented <= maxRenderedSubjectRunes && exact == ""; accented++ {
		for ascii := 0; accented+ascii <= maxRenderedSubjectRunes; ascii++ {
			candidate := strings.Repeat("é", accented) + strings.Repeat("a", ascii)
			if subjectLine(candidate) == maxSubjectLineOctets {
				exact = candidate
				break
			}
		}
	}
	if exact == "" {
		t.Skip("no exact 998-octet boundary found for the deterministic enumeration")
	}
	t.Logf("derived exact boundary with %d runes", utf8.RuneCountInString(exact))
	if got := sanitizeRenderedSubject(exact); got != exact {
		t.Fatal("exactly 998-octet subject was truncated")
	}
	extended := exact + "a"
	if got := sanitizeRenderedSubject(extended); got == extended || utf8.RuneCountInString(got) >= utf8.RuneCountInString(extended) {
		t.Fatal("one rune beyond the exact boundary was not truncated")
	}
}

func TestNewEmailServiceLoadsOptionalTemplatesIndependently(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		verificationSubjectTemplateFile: "Verify {{.Code}}",
		verificationHTMLTemplateFile:    "<b>{{.Code}}</b>",
		invitationSubjectTemplateFile:   "Join {{.WorkspaceName}}",
		invitationHTMLTemplateFile:      `<a href="{{.InviteURL}}">Join</a>`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EMAIL_TEMPLATE_DIR", dir)
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("SMTP_HOST", "")

	s := NewEmailService()
	if s.verificationSubjectTemplate == nil || s.verificationHTMLTemplate == nil ||
		s.invitationSubjectTemplate == nil || s.invitationHTMLTemplate == nil {
		t.Fatalf("not all templates loaded: %+v", s)
	}
}

type fakeEmailSender struct {
	mu       sync.Mutex
	requests []*resend.SendEmailRequest
	err      error
}

func (f *fakeEmailSender) Send(params *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	copyParams := *params
	copyParams.To = append([]string(nil), params.To...)
	f.requests = append(f.requests, &copyParams)
	return &resend.SendEmailResponse{Id: "test"}, f.err
}

func (f *fakeEmailSender) lastRequest(t *testing.T) *resend.SendEmailRequest {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) == 0 {
		t.Fatal("no email request captured")
	}
	return f.requests[len(f.requests)-1]
}

func capturedMessage(t *testing.T, srv *testSMTPServer) (*mail.Message, string) {
	t.Helper()
	select {
	case data := <-srv.CapturedData:
		msg, err := mail.ReadMessage(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("parse captured message: %v\n%s", err, data)
		}
		var reader io.Reader = msg.Body
		if msg.Header.Get("Content-Transfer-Encoding") == "quoted-printable" {
			reader = mimeQuotedPrintableReader(msg.Body)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read captured body: %v", err)
		}
		return msg, string(body)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SMTP DATA")
		return nil, ""
	}
}

func mimeQuotedPrintableReader(r io.Reader) io.Reader {
	return quotedprintable.NewReader(r)
}

func mustTextEmailTemplate(t *testing.T, name, source string) *texttemplate.Template {
	t.Helper()
	tmpl, err := texttemplate.New(name).Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	return tmpl
}

func mustHTMLEmailTemplate(t *testing.T, name, source string) *htmltemplate.Template {
	t.Helper()
	tmpl, err := htmltemplate.New(name).Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	return tmpl
}

func captureProcessOutput(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = w, w
	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	defer func() {
		os.Stdout, os.Stderr = oldStdout, oldStderr
	}()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return <-done
}

func TestSendVerificationCodeCustomTemplatesReachResend(t *testing.T) {
	sender := &fakeEmailSender{}
	s := &EmailService{
		client:                      sender,
		fromEmail:                   "configured@example.com",
		verificationSubjectTemplate: mustTextEmailTemplate(t, verificationSubjectTemplateFile, "Code {{.Code}}\r\nBcc: evil@example.com\t"),
		verificationHTMLTemplate: mustHTMLEmailTemplate(t, verificationHTMLTemplateFile,
			"<p>{{.Code}} expires in {{.ExpiresInMinutes}} minutes on {{.AppName}}</p>"),
	}

	if err := s.SendVerificationCode("to@example.com", "918273"); err != nil {
		t.Fatal(err)
	}
	request := sender.lastRequest(t)
	if request.Subject != "Code 918273Bcc: evil@example.com" {
		t.Fatalf("Subject = %q", request.Subject)
	}
	if request.Html != "<p>918273 expires in 10 minutes on Multica</p>" {
		t.Fatalf("Html = %q", request.Html)
	}
	if request.From != "configured@example.com" || len(request.To) != 1 || request.To[0] != "to@example.com" {
		t.Fatalf("identity changed: From=%q To=%v", request.From, request.To)
	}
}

func TestSendInvitationEmailCustomTemplatesReachResend(t *testing.T) {
	t.Setenv("FRONTEND_ORIGIN", "javascript:alert(1)")
	sender := &fakeEmailSender{}
	s := &EmailService{
		client:                    sender,
		fromEmail:                 "configured@example.com",
		invitationSubjectTemplate: mustTextEmailTemplate(t, invitationSubjectTemplateFile, "{{.InviterName}} / {{.WorkspaceName}} / {{.AppName}}"),
		invitationHTMLTemplate: mustHTMLEmailTemplate(t, invitationHTMLTemplateFile,
			`<p>{{.WorkspaceName}}</p><a href="{{.InviteURL}}">Invite</a><a href="{{.AppURL}}">Home</a>`),
	}

	if err := s.SendInvitationEmail("to@example.com", "Alice", "A&B <img src=x>", "abc"); err != nil {
		t.Fatal(err)
	}
	request := sender.lastRequest(t)
	if request.Subject != "Alice / A&B <img src=x> / Multica" {
		t.Fatalf("Subject = %q", request.Subject)
	}
	if !strings.Contains(request.Html, "A&amp;B &lt;img src=x&gt;") || strings.Contains(request.Html, "A&amp;amp;B") {
		t.Fatalf("contextual escaping failed: %q", request.Html)
	}
	if strings.Count(request.Html, "#ZgotmplZ") != 2 {
		t.Fatalf("unsafe URLs were not filtered: %q", request.Html)
	}
}

func TestSendInvitationEmailTemplateComponentsAreIndependent(t *testing.T) {
	t.Setenv("FRONTEND_ORIGIN", "https://example.com")
	sender := &fakeEmailSender{}
	s := &EmailService{
		client:                 sender,
		fromEmail:              "configured@example.com",
		invitationHTMLTemplate: mustHTMLEmailTemplate(t, invitationHTMLTemplateFile, "custom {{.WorkspaceName}}"),
	}

	if err := s.SendInvitationEmail("to@example.com", "Alice", "Acme", "abc"); err != nil {
		t.Fatal(err)
	}
	request := sender.lastRequest(t)
	if request.Subject != "Alice invited you to Acme on Multica" {
		t.Fatalf("built-in Subject changed: %q", request.Subject)
	}
	if request.Html != "custom Acme" {
		t.Fatalf("custom Html = %q", request.Html)
	}
}

func TestDEVModeDoesNotRenderTemplates(t *testing.T) {
	s := &EmailService{
		verificationSubjectTemplate: mustTextEmailTemplate(t, verificationSubjectTemplateFile, "{{slice .Code 0 99}}"),
		verificationHTMLTemplate:    mustHTMLEmailTemplate(t, verificationHTMLTemplateFile, "{{.Code}} rendered"),
	}
	output := captureProcessOutput(t, func() {
		if err := s.SendVerificationCode("dev@example.com", "918273"); err != nil {
			t.Fatal(err)
		}
	})
	if output != "[DEV] Verification code for dev@example.com: 918273\n" {
		t.Fatalf("DEV output changed: %q", output)
	}
}

func TestCustomTemplateSMTPWireSafetyAndTransferEncoding(t *testing.T) {
	for _, advertise8Bit := range []bool{false, true} {
		t.Run(fmt.Sprintf("8BITMIME=%v", advertise8Bit), func(t *testing.T) {
			srv, cleanup := startTestSMTPServer(t, testSMTPServer{Advertise8BITMIME: advertise8Bit})
			defer cleanup()
			host, port, _ := net.SplitHostPort(srv.Addr)
			s := &EmailService{
				fromEmail:                   "configured@example.com",
				smtpHost:                    host,
				smtpPort:                    port,
				verificationSubjectTemplate: mustTextEmailTemplate(t, verificationSubjectTemplateFile, "你好\r\nBcc: evil@example.com\t"),
				verificationHTMLTemplate: mustHTMLEmailTemplate(t, verificationHTMLTemplateFile,
					"From: evil@example.com\nReply-To: evil@example.com\n.\n完成 {{.Code}}"),
			}

			if err := s.SendVerificationCode("to@example.com", "918273"); err != nil {
				t.Fatal(err)
			}
			select {
			case envelope := <-srv.CapturedEnvelope:
				if envelope != "MAIL FROM:<configured@example.com>" && envelope != "MAIL FROM:<configured@example.com> BODY=8BITMIME" {
					t.Errorf("SMTP envelope = %q", envelope)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for SMTP envelope")
			}
			msg, body := capturedMessage(t, srv)
			decodedSubject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
			if err != nil {
				t.Fatal(err)
			}
			if decodedSubject != "你好Bcc: evil@example.com" {
				t.Errorf("Subject = %q", decodedSubject)
			}
			for _, name := range []string{"Bcc", "Cc", "Reply-To"} {
				if got := msg.Header.Get(name); got != "" {
					t.Errorf("injected %s header = %q", name, got)
				}
			}
			if got := msg.Header.Get("From"); got != "configured@example.com" {
				t.Errorf("From = %q", got)
			}
			wantCTE := "quoted-printable"
			if advertise8Bit {
				wantCTE = "8bit"
			}
			if got := msg.Header.Get("Content-Transfer-Encoding"); got != wantCTE {
				t.Errorf("Content-Transfer-Encoding = %q, want %q", got, wantCTE)
			}
			if body != "From: evil@example.com\nReply-To: evil@example.com\n.\n完成 918273\n" {
				t.Errorf("decoded body = %q", body)
			}
		})
	}
}

func writeEmailTemplate(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newEmailServiceWithTemplateDir(t *testing.T, dir string) (*EmailService, string) {
	t.Helper()
	t.Setenv("EMAIL_TEMPLATE_DIR", dir)
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("SMTP_HOST", "")
	var service *EmailService
	output := captureProcessOutput(t, func() {
		service = NewEmailService()
	})
	return service, output
}

func TestEmailTemplateLoadingFailureSemantics(t *testing.T) {
	t.Run("missing directory falls back once", func(t *testing.T) {
		s, output := newEmailServiceWithTemplateDir(t, filepath.Join(t.TempDir(), "missing"))
		if strings.Count(output, "template directory") != 1 {
			t.Fatalf("template directory warnings = %d\n%s", strings.Count(output, "template directory"), output)
		}
		if s.verificationSubjectTemplate != nil || s.verificationHTMLTemplate != nil ||
			s.invitationSubjectTemplate != nil || s.invitationHTMLTemplate != nil {
			t.Fatal("templates loaded from a missing directory")
		}
	})

	t.Run("missing files are silent", func(t *testing.T) {
		_, output := newEmailServiceWithTemplateDir(t, t.TempDir())
		if strings.Contains(output, "template ") {
			t.Fatalf("optional missing files warned: %s", output)
		}
	})

	t.Run("parse failure is isolated", func(t *testing.T) {
		dir := t.TempDir()
		writeEmailTemplate(t, dir, verificationSubjectTemplateFile, "{{.Code")
		writeEmailTemplate(t, dir, verificationHTMLTemplateFile, "custom {{.Code}}")
		s, output := newEmailServiceWithTemplateDir(t, dir)
		if s.verificationSubjectTemplate != nil || s.verificationHTMLTemplate == nil {
			t.Fatalf("parse failure was not isolated: %+v", s)
		}
		if strings.Count(output, "failed to parse template "+verificationSubjectTemplateFile) != 1 {
			t.Fatalf("parse warning missing or duplicated: %s", output)
		}
	})

	t.Run("unknown field remains active and falls back at runtime", func(t *testing.T) {
		dir := t.TempDir()
		writeEmailTemplate(t, dir, verificationSubjectTemplateFile, "{{.SMTPPassword}}")
		s, startupOutput := newEmailServiceWithTemplateDir(t, dir)
		if s.verificationSubjectTemplate == nil {
			t.Fatal("diagnostic disabled the template")
		}
		if !strings.Contains(startupOutput, "template remains active") || !strings.Contains(startupOutput, "SMTPPassword") {
			t.Fatalf("diagnostic warning is not actionable: %s", startupOutput)
		}
		sender := &fakeEmailSender{}
		s.client = sender
		runtimeOutput := captureProcessOutput(t, func() {
			if err := s.SendVerificationCode("to@example.com", "918273"); err != nil {
				t.Fatal(err)
			}
		})
		request := sender.lastRequest(t)
		if request.Subject != "Your Multica verification code" {
			t.Fatalf("runtime failure sent partial subject: %q", request.Subject)
		}
		// This sentinel probe turns red if the warning is changed to include the
		// template data (for example with a data=%+v logging argument).
		if !strings.Contains(runtimeOutput, "using built-in content") || strings.Contains(runtimeOutput, "918273") {
			t.Fatalf("runtime warning leaked data or omitted fallback: %s", runtimeOutput)
		}
	})

	t.Run("zero-value false positive does not disable template", func(t *testing.T) {
		dir := t.TempDir()
		writeEmailTemplate(t, dir, verificationSubjectTemplateFile, "{{slice .Code 0 3}}")
		s, startupOutput := newEmailServiceWithTemplateDir(t, dir)
		if !strings.Contains(startupOutput, "template remains active") {
			t.Fatalf("expected diagnostic warning: %s", startupOutput)
		}
		sender := &fakeEmailSender{}
		s.client = sender
		if err := s.SendVerificationCode("to@example.com", "123456"); err != nil {
			t.Fatal(err)
		}
		if got := sender.lastRequest(t).Subject; got != "123" {
			t.Fatalf("active template rendered %q, want 123", got)
		}
	})

	t.Run("zero-value false negative falls back without partial output", func(t *testing.T) {
		dir := t.TempDir()
		writeEmailTemplate(t, dir, verificationHTMLTemplateFile, "prefix {{if .Code}}{{slice .Code 0 99}}{{end}} suffix")
		s, startupOutput := newEmailServiceWithTemplateDir(t, dir)
		if strings.Contains(startupOutput, "template diagnostic") {
			t.Fatalf("zero value unexpectedly diagnosed the false negative: %s", startupOutput)
		}
		sender := &fakeEmailSender{}
		s.client = sender
		runtimeOutput := captureProcessOutput(t, func() {
			if err := s.SendVerificationCode("to@example.com", "918273"); err != nil {
				t.Fatal(err)
			}
		})
		request := sender.lastRequest(t)
		if strings.Contains(request.Html, "prefix") || !strings.Contains(request.Html, "This code expires in 10 minutes.") {
			t.Fatalf("runtime failure did not use complete built-in body: %q", request.Html)
		}
		if strings.Contains(runtimeOutput, "918273") {
			t.Fatalf("runtime warning leaked verification code: %s", runtimeOutput)
		}
	})
}

func TestBuiltInVerificationContentAndTTLRemainCompatible(t *testing.T) {
	sender := &fakeEmailSender{}
	s := &EmailService{client: sender, fromEmail: "from@example.com"}
	if err := s.SendVerificationCode("to@example.com", "123456"); err != nil {
		t.Fatal(err)
	}
	request := sender.lastRequest(t)
	wantBody := `<div style="font-family: sans-serif; max-width: 400px; margin: 0 auto;">
			<h2>Your verification code</h2>
			<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 24px 0;">123456</p>
			<p>This code expires in 10 minutes.</p>
			<p style="color: #666; font-size: 14px;">If you didn't request this code, you can safely ignore this email.</p>
		</div>`
	if request.Subject != "Your Multica verification code" || request.Html != wantBody {
		t.Fatalf("built-in output changed\nSubject: %q\nHtml: %q", request.Subject, request.Html)
	}
	minutes := int(verificationCodeTTL / time.Minute)
	if !strings.Contains(request.Html, fmt.Sprintf("expires in %d minutes", minutes)) {
		t.Fatalf("built-in body is not tied to verificationCodeTTL=%s", verificationCodeTTL)
	}
}

func maskAndValidateDynamicEmailHeaders(t *testing.T, data []byte) string {
	t.Helper()
	lines := strings.Split(string(data), "\n")
	foundDate, foundMessageID := false, false
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "Date: "):
			if _, err := mail.ParseDate(strings.TrimPrefix(line, "Date: ")); err != nil {
				t.Errorf("invalid Date header %q: %v", line, err)
			}
			lines[i] = "Date: <dynamic>"
			foundDate = true
		case strings.HasPrefix(line, "Message-ID: "):
			value := strings.TrimPrefix(line, "Message-ID: ")
			if !strings.HasPrefix(value, "<") || !strings.Contains(value, "@") || !strings.HasSuffix(value, ">") {
				t.Errorf("invalid Message-ID header %q", line)
			}
			lines[i] = "Message-ID: <dynamic>"
			foundMessageID = true
		}
	}
	if !foundDate || !foundMessageID {
		t.Fatalf("missing dynamic headers: Date=%v Message-ID=%v", foundDate, foundMessageID)
	}
	return strings.Join(lines, "\n")
}

func TestBuiltInVerificationSMTPWireRemainsCompatible(t *testing.T) {
	srv, cleanup := startTestSMTPServer(t, testSMTPServer{})
	defer cleanup()
	host, port, _ := net.SplitHostPort(srv.Addr)
	s := &EmailService{
		fromEmail: "from@example.com",
		smtpHost:  host,
		smtpPort:  port,
	}
	if err := s.SendVerificationCode("to@example.com", "123456"); err != nil {
		t.Fatal(err)
	}

	var data []byte
	select {
	case data = <-srv.CapturedData:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SMTP DATA")
	}
	got := maskAndValidateDynamicEmailHeaders(t, data)

	body := `<div style="font-family: sans-serif; max-width: 400px; margin: 0 auto;">
			<h2>Your verification code</h2>
			<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 24px 0;">123456</p>
			<p>This code expires in 10 minutes.</p>
			<p style="color: #666; font-size: 14px;">If you didn't request this code, you can safely ignore this email.</p>
		</div>`
	var encodedBody strings.Builder
	qp := quotedprintable.NewWriter(&encodedBody)
	if _, err := qp.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := qp.Close(); err != nil {
		t.Fatal(err)
	}
	wireBody := strings.ReplaceAll(encodedBody.String(), "\r\n", "\n")
	want := "From: from@example.com\n" +
		"To: to@example.com\n" +
		"Subject: Your Multica verification code\n" +
		"Date: <dynamic>\n" +
		"Message-ID: <dynamic>\n" +
		"MIME-Version: 1.0\n" +
		"Content-Type: text/html; charset=UTF-8\n" +
		"Content-Transfer-Encoding: quoted-printable\n\n" + wireBody + "\n"
	if got != want {
		t.Fatalf("built-in SMTP wire changed after masking dynamic headers\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuiltInInvitationSMTPContentRemainsCompatible(t *testing.T) {
	t.Setenv("FRONTEND_ORIGIN", "https://example.com")
	srv, cleanup := startTestSMTPServer(t, testSMTPServer{})
	defer cleanup()
	host, port, _ := net.SplitHostPort(srv.Addr)
	s := &EmailService{fromEmail: "from@example.com", smtpHost: host, smtpPort: port}
	if err := s.SendInvitationEmail("to@example.com", "Alice", "A&B <s>", "abc"); err != nil {
		t.Fatal(err)
	}
	msg, body := capturedMessage(t, srv)
	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		t.Fatal(err)
	}
	if decodedSubject != "Alice invited you to A&B <s> on Multica" {
		t.Fatalf("Subject = %q", decodedSubject)
	}
	if !strings.Contains(body, "A&amp;B &lt;s&gt;") || strings.Contains(body, "A&B <s>") {
		t.Fatalf("built-in invitation escaping changed: %q", body)
	}
}

func TestRenderedCJKSubjectSMTPBoundary(t *testing.T) {
	srv, cleanup := startTestSMTPServer(t, testSMTPServer{})
	defer cleanup()
	host, port, _ := net.SplitHostPort(srv.Addr)
	s := &EmailService{
		fromEmail:                   "from@example.com",
		smtpHost:                    host,
		smtpPort:                    port,
		verificationSubjectTemplate: mustTextEmailTemplate(t, verificationSubjectTemplateFile, strings.Repeat("中", 200)),
	}
	if err := s.SendVerificationCode("to@example.com", "123456"); err != nil {
		t.Fatal(err)
	}
	msg, _ := capturedMessage(t, srv)
	rawSubject := msg.Header.Get("Subject")
	if got := len("Subject: " + rawSubject); got != 996 {
		t.Fatalf("Subject line = %d octets, want 996", got)
	}
	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(rawSubject)
	if err != nil {
		t.Fatal(err)
	}
	if decodedSubject != strings.Repeat("中", 91) {
		t.Fatalf("decoded Subject has %d runes, want 91", utf8.RuneCountInString(decodedSubject))
	}
}

func TestCustomTemplateSMTPAndResendSinksMatch(t *testing.T) {
	subjectTemplate := mustTextEmailTemplate(t, invitationSubjectTemplateFile, "Welcome {{.WorkspaceName}}")
	htmlTemplate := mustHTMLEmailTemplate(t, invitationHTMLTemplateFile, "<p>{{.InviterName}} / {{.InviteURL}}</p>")
	t.Setenv("FRONTEND_ORIGIN", "https://example.com")

	sender := &fakeEmailSender{}
	resendService := &EmailService{
		client:                    sender,
		fromEmail:                 "from@example.com",
		invitationSubjectTemplate: subjectTemplate,
		invitationHTMLTemplate:    htmlTemplate,
	}
	if err := resendService.SendInvitationEmail("to@example.com", "Alice", "Acme", "abc"); err != nil {
		t.Fatal(err)
	}
	want := sender.lastRequest(t)

	srv, cleanup := startTestSMTPServer(t, testSMTPServer{})
	defer cleanup()
	host, port, _ := net.SplitHostPort(srv.Addr)
	smtpService := &EmailService{
		fromEmail:                 "from@example.com",
		smtpHost:                  host,
		smtpPort:                  port,
		invitationSubjectTemplate: subjectTemplate,
		invitationHTMLTemplate:    htmlTemplate,
	}
	if err := smtpService.SendInvitationEmail("to@example.com", "Alice", "Acme", "abc"); err != nil {
		t.Fatal(err)
	}
	msg, body := capturedMessage(t, srv)
	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		t.Fatal(err)
	}
	if decodedSubject != want.Subject || strings.TrimSuffix(body, "\n") != want.Html {
		t.Fatalf("sink mismatch: SMTP=(%q,%q), Resend=(%q,%q)", decodedSubject, body, want.Subject, want.Html)
	}
}

func TestEmailTemplatesAreSafeForConcurrentUse(t *testing.T) {
	t.Setenv("FRONTEND_ORIGIN", "https://example.com")
	sender := &fakeEmailSender{}
	s := &EmailService{
		client:                      sender,
		fromEmail:                   "from@example.com",
		verificationSubjectTemplate: mustTextEmailTemplate(t, verificationSubjectTemplateFile, "verify {{.Code}}"),
		verificationHTMLTemplate:    mustHTMLEmailTemplate(t, verificationHTMLTemplateFile, "<p>{{.Code}}</p>"),
		invitationSubjectTemplate:   mustTextEmailTemplate(t, invitationSubjectTemplateFile, "join {{.WorkspaceName}}"),
		invitationHTMLTemplate:      mustHTMLEmailTemplate(t, invitationHTMLTemplateFile, "<p>{{.InviterName}}</p>"),
	}

	var wg sync.WaitGroup
	wantMarker := make(map[string]string, 50)
	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			wantMarker[fmt.Sprintf("v%d@example.com", i)] = fmt.Sprintf("%06d", i)
		} else {
			wantMarker[fmt.Sprintf("i%d@example.com", i)] = fmt.Sprintf("Inviter%d", i)
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i%2 == 0 {
				err = s.SendVerificationCode(fmt.Sprintf("v%d@example.com", i), fmt.Sprintf("%06d", i))
			} else {
				err = s.SendInvitationEmail(fmt.Sprintf("i%d@example.com", i), fmt.Sprintf("Inviter%d", i), fmt.Sprintf("Workspace%d", i), fmt.Sprintf("id-%d", i))
			}
			if err != nil {
				t.Errorf("send %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.requests) != 50 {
		t.Fatalf("captured %d requests, want 50", len(sender.requests))
	}
	for _, request := range sender.requests {
		if len(request.To) != 1 {
			t.Errorf("incoherent request: %+v", request)
			continue
		}
		marker, ok := wantMarker[request.To[0]]
		if !ok || !strings.Contains(request.Html, marker) {
			t.Errorf("request content crossed sends: To=%v Html=%q, want marker %q", request.To, request.Html, marker)
		}
	}
}
