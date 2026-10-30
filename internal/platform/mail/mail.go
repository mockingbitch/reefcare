// Package mail sends transactional email. Messages are sent from durable jobs
// (identity's "identity.email" job), never from the request path.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

// Message is a plain-text email.
type Message struct {
	To      string
	Subject string
	Body    string
	// Kind is a short label for logs and metrics (e.g. "verify_email"). Never log To or Body.
	Kind string
}

// Sender delivers a message.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// SMTPConfig configures SMTPSender.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// RequireTLS refuses to send unless the server offers STARTTLS (production).
	RequireTLS bool
	Timeout    time.Duration
}

// SMTPSender sends through an SMTP relay with optional STARTTLS and PLAIN auth.
type SMTPSender struct {
	cfg  SMTPConfig
	from mail.Address
}

// NewSMTPSender validates cfg and returns a sender.
func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("invalid MAIL_FROM: %w", err)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &SMTPSender{cfg: cfg, from: *from}, nil
}

// Send delivers m, honoring ctx and the configured timeout.
func (s *SMTPSender) Send(ctx context.Context, m Message) error {
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprint(s.cfg.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	} else if s.cfg.RequireTLS {
		return errors.New("smtp server does not offer STARTTLS")
	}
	if s.cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to.Address); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(s.render(*to, m)); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("finish message: %w", err)
	}
	return c.Quit()
}

func (s *SMTPSender) render(to mail.Address, m Message) []byte {
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	domain := s.from.Address[strings.LastIndex(s.from.Address, "@")+1:]

	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", s.from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domain)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	// Normalize every line ending, including lone CR, to CRLF so no bare CR or LF reaches
	// SMTP DATA (SMTP smuggling via lenient relays).
	body := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(m.Body)
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return b.Bytes()
}

// SafeReason reduces a send error to its SMTP reply code or network error class: server
// reply text often echoes the recipient address, which must never be logged or stored
// (docs/security.md §8).
func SafeReason(err error) string {
	var tpErr *textproto.Error
	if errors.As(err, &tpErr) {
		return fmt.Sprintf("smtp reply %d", tpErr.Code)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return "network: " + netErr.Error()
	}
	return "smtp error"
}

// LogSender records only that a message would have been sent. It never logs the body,
// which contains single-use tokens.
type LogSender struct{ Log *slog.Logger }

// Send logs the message kind.
func (l LogSender) Send(ctx context.Context, m Message) error {
	l.Log.InfoContext(ctx, "email not sent: no SMTP configured", "kind", m.Kind)
	return nil
}
