package mail

import (
	"errors"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
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

func TestSafeReasonDropsReplyText(t *testing.T) {
	err := &textproto.Error{Code: 550, Msg: "5.1.1 <victim@example.com>: Recipient rejected"}
	if got := SafeReason(errors.Join(errors.New("smtp RCPT TO"), err)); got != "smtp reply 550" || strings.Contains(got, "victim") {
		t.Fatalf("SafeReason = %q", got)
	}
}
