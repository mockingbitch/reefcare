package identity

import (
	"fmt"
	"net/url"

	"reefcare/internal/platform/mail"
)

// Email copy per locale. Messages use a generic greeting: the display name is chosen by
// whoever registered, who may not own the mailbox, so it must never appear in mail
// (it would turn ReefCare into a relay for attacker-written text).
type emailTemplate struct{ subject, body string }

var emailTemplates = map[string]map[string]emailTemplate{
	"verify_email": {
		"vi": {"Xác minh email ReefCare của bạn",
			"Xin chào,\n\nHãy xác minh địa chỉ email của bạn bằng liên kết sau (hết hạn sau 24 giờ):\n\n%s\n\nNếu bạn không đăng ký ReefCare, hãy bỏ qua email này.\n"},
		"en": {"Verify your ReefCare email",
			"Hello,\n\nPlease verify your email address using this link (expires in 24 hours):\n\n%s\n\nIf you did not sign up for ReefCare, you can ignore this email.\n"},
	},
	"already_registered": {
		"vi": {"Bạn đã có tài khoản ReefCare",
			"Xin chào,\n\nCó người vừa thử đăng ký ReefCare bằng email này, nhưng email đã có tài khoản. Nếu đó là bạn, hãy đăng nhập hoặc đặt lại mật khẩu tại:\n\n%s\n\nNếu không phải bạn, hãy bỏ qua email này.\n"},
		"en": {"You already have a ReefCare account",
			"Hello,\n\nSomeone tried to sign up for ReefCare with this email, but it already has an account. If that was you, sign in or reset your password at:\n\n%s\n\nIf it wasn't you, you can ignore this email.\n"},
	},
	"reset_password": {
		"vi": {"Đặt lại mật khẩu ReefCare",
			"Xin chào,\n\nDùng liên kết sau để đặt lại mật khẩu (hết hạn sau 1 giờ, chỉ dùng được một lần):\n\n%s\n\nNếu bạn không yêu cầu, hãy bỏ qua email này; mật khẩu của bạn không thay đổi.\n"},
		"en": {"Reset your ReefCare password",
			"Hello,\n\nUse this link to reset your password (expires in 1 hour, single use):\n\n%s\n\nIf you did not request this, ignore this email; your password is unchanged.\n"},
	},
	"password_changed": {
		"vi": {"Mật khẩu ReefCare của bạn đã được thay đổi",
			"Xin chào,\n\nMật khẩu tài khoản ReefCare của bạn vừa được thay đổi và các phiên đăng nhập khác đã bị đăng xuất. Nếu không phải bạn, hãy đặt lại mật khẩu ngay tại:\n\n%s\n"},
		"en": {"Your ReefCare password was changed",
			"Hello,\n\nYour ReefCare password was just changed and your other sessions were signed out. If this wasn't you, reset your password now at:\n\n%s\n"},
	},
	"account_deactivated": {
		"vi": {"Tài khoản ReefCare của bạn đã bị vô hiệu hoá",
			"Xin chào,\n\nTài khoản ReefCare của bạn vừa được vô hiệu hoá theo yêu cầu và mọi phiên đăng nhập đã bị đăng xuất. Dữ liệu cá nhân sẽ bị xoá hoặc ẩn danh sau 30 ngày.\n\nNếu không phải bạn yêu cầu, hãy trả lời email này ngay để được hỗ trợ.\n\n%s\n"},
		"en": {"Your ReefCare account was deactivated",
			"Hello,\n\nYour ReefCare account was deactivated at your request and every session was signed out. Personal data will be erased or anonymized after 30 days.\n\nIf you did not request this, reply to this email right away.\n\n%s\n"},
	},
}

// email builds a message linking to a frontend path. A token is placed in the URL
// fragment (#token=…), which browsers never send to servers or in Referer headers, so
// it cannot leak into proxy access logs or third-party requests.
func (s *Service) email(kind, locale, to, path, token string) mail.Message {
	t, ok := emailTemplates[kind][locale]
	if !ok {
		t = emailTemplates[kind]["vi"]
	}
	link := s.cfg.PublicBaseURL + path
	if token != "" {
		link += "#" + url.Values{"token": {token}}.Encode()
	}
	return mail.Message{To: to, Subject: t.subject, Body: fmt.Sprintf(t.body, link), Kind: kind}
}
