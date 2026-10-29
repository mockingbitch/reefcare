package mail

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRenderNormalizesLineEndings(t *testing.T) {
	s, err := NewSMTPSender(SMTPConfig{Host: "localhost", Port: 25, From: "ReefCare <no-reply@reefcare.test>"})
	if err != nil {
		t.Fatal(err)
	}
	out := string(s.render(mustAddr(t, "lan@example.com"), Message{Subject: "Xác minh", Body: "a\rb\nc\r\nd\r.\r"}))
	_, body, _ := strings.Cut(out, "\r\n\r\n")
	if body != "a\r\nb\r\nc\r\nd\r\n.\r\n" {
		t.Fatalf("body = %q", body)
	}
	if strings.Contains(strings.ReplaceAll(out, "\r\n", ""), "\r") || strings.Contains(strings.ReplaceAll(out, "\r\n", ""), "\n") {
		t.Fatal("bare CR or LF in rendered message")
	}
	if !strings.Contains(out, "Subject: =?utf-8?q?") {
		t.Errorf("non-ASCII subject must be encoded: %q", out)
	}
}

func mustAddr(t *testing.T, s string) mail.Address {
	t.Helper()
	a, err := mail.ParseAddress(s)
	if err != nil {
		t.Fatal(err)
	}
	return *a
}

func TestLogSafeDropsReplyText(t *testing.T) {
	err := &textproto.Error{Code: 550, Msg: "5.1.1 <victim@example.com>: Recipient rejected"}
	if got := logSafe(errors.Join(errors.New("smtp RCPT TO"), err)); got != "smtp reply 550" || strings.Contains(got, "victim") {
		t.Fatalf("logSafe = %q", got)
	}
}

type countingSender struct {
	mu sync.Mutex
	n  int
}

func (c *countingSender) Send(context.Context, Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	return nil
}

func TestAsyncDrainsAndRejectsAfterClose(t *testing.T) {
	sender := &countingSender{}
	a := NewAsync(sender, slog.New(slog.NewTextHandler(io.Discard, nil)), 10)
	for range 3 {
		if err := a.Send(context.Background(), Message{Kind: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.n != 3 {
		t.Fatalf("delivered %d, want 3", sender.n)
	}
	if err := a.Send(context.Background(), Message{}); err == nil {
		t.Fatal("send after close must fail, not panic")
	}
	if err := a.Close(ctx); err != nil {
		t.Fatal("second close must be a no-op")
	}
}
