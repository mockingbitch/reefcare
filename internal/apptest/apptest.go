//go:build integration

// Package apptest runs the full HTTP application against an isolated PostgreSQL
// database for API integration tests: one Env per test, one Browser per signed-in
// device (own cookie jar and CSRF token).
package apptest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/app"
	"reefcare/internal/config"
	"reefcare/internal/identity"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database/dbtest"
	"reefcare/internal/platform/jobs"
	"reefcare/internal/platform/mail"
)

// Password is a valid password used by Register.
const Password = "correct horse battery"

// Origin is the public origin the test app is configured with.
const Origin = "http://reefcare.test"

// Outbox records emails instead of sending them.
type Outbox struct {
	mu       sync.Mutex
	msgs     []mail.Message
	failNext int
}

// ErrSMTPDown is what Send returns while failures are injected with FailNext.
var ErrSMTPDown = errors.New("smtp unavailable")

// FailNext makes the next n sends fail, like an SMTP outage.
func (o *Outbox) FailNext(n int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failNext = n
}

// Send implements mail.Sender.
func (o *Outbox) Send(_ context.Context, m mail.Message) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.failNext > 0 {
		o.failNext--
		return ErrSMTPDown
	}
	o.msgs = append(o.msgs, m)
	return nil
}

// All returns a copy of every message sent so far.
func (o *Outbox) All() []mail.Message {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]mail.Message(nil), o.msgs...)
}

// Tokens travel in the URL fragment so they never reach server logs or Referer headers.
var tokenRe = regexp.MustCompile(`#token=([A-Za-z0-9_-]+)`)

// WaitFor returns the most recent message of kind sent to `to` among messages after
// index `after`, polling briefly because some emails come from background work.
func (o *Outbox) WaitFor(t testing.TB, kind, to string, after int) mail.Message {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		msgs := o.All()
		for i := len(msgs) - 1; i >= after; i-- {
			if msgs[i].Kind == kind && msgs[i].To == to {
				return msgs[i]
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s email to %s; outbox: %+v", kind, to, msgs)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// LastToken returns the token from the most recent message of kind sent to `to`.
func (o *Outbox) LastToken(t testing.TB, kind, to string) string {
	t.Helper()
	return o.TokenAfter(t, kind, to, 0)
}

// TokenAfter is LastToken restricted to messages after index `after`.
func (o *Outbox) TokenAfter(t testing.TB, kind, to string, after int) string {
	t.Helper()
	m := tokenRe.FindStringSubmatch(o.WaitFor(t, kind, to, after).Body)
	if m == nil {
		t.Fatalf("no fragment token in %s email", kind)
	}
	return m[1]
}

// Env is a running application with its database, clock and outbox.
type Env struct {
	T      testing.TB
	Srv    *httptest.Server
	Pool   *pgxpool.Pool
	Clock  *clock.Fixed
	Outbox *Outbox
	Jobs   *jobs.Runner
}

// RunJobs runs every job that is due at the test clock's time (including periodic jobs
// of the current time bucket) and waits until none is pending or running, including
// jobs the background runner picked up first and jobs that enqueued further jobs.
func (e *Env) RunJobs() {
	e.T.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := e.Jobs.RunDue(ctx); err != nil {
			e.T.Fatal(err)
		}
		var busy int
		if err := e.Pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE status IN ('pending', 'running') AND run_at <= $1`, e.Clock.Now()).Scan(&busy); err != nil {
			e.T.Fatal(err)
		}
		if busy == 0 {
			return
		}
		if time.Now().After(deadline) {
			e.T.Fatalf("%d due jobs still pending or running", busy)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// New starts the application on a fresh database. The clock starts at
// 2026-10-29 03:00 UTC and only moves when the test advances it.
func New(t testing.TB) *Env {
	t.Helper()
	pool := dbtest.New(t)
	e := &Env{T: t, Pool: pool, Clock: clock.NewFixed(time.Date(2026, 10, 29, 3, 0, 0, 0, time.UTC)), Outbox: &Outbox{}}
	application := app.New(app.Deps{
		Config: config.Config{PublicBaseURL: Origin, AllowedOrigins: []string{Origin}, JobWorkers: 2, JobPollInterval: 5 * time.Millisecond},
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Pool:   pool, Clock: e.Clock, Mailer: e.Outbox,
		Argon2: identity.Argon2Params{MemoryKiB: 1024, Iterations: 1, Parallelism: 1},
	})
	e.Jobs = application.Jobs
	e.Srv = httptest.NewServer(application.Handler)
	// Jobs (emails, audits) run in the background like in production; tests that need
	// them wait (Outbox.WaitFor) or call RunJobs.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); application.Jobs.Run(ctx) }()
	t.Cleanup(func() {
		e.Srv.Close()
		cancel()
		<-done
	})
	return e
}

// Browser is a client with its own cookie jar, like one signed-in device.
type Browser struct {
	e    *Env
	HTTP *http.Client
	CSRF string
}

// Browser returns a new, signed-out client.
func (e *Env) Browser() *Browser {
	jar, _ := cookiejar.New(nil)
	return &Browser{e: e, HTTP: &http.Client{Jar: jar}}
}

// Response is a decoded API response.
type Response struct {
	Status int
	Header http.Header
	Body   map[string]any
	Raw    string
}

// ErrCode returns error.code from an error envelope, or "".
func (r Response) ErrCode() string {
	if e, ok := r.Body["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

// FieldCodes maps error.details[].field to its code.
func (r Response) FieldCodes() map[string]string {
	out := map[string]string{}
	e, _ := r.Body["error"].(map[string]any)
	details, _ := e["details"].([]any)
	for _, d := range details {
		m, _ := d.(map[string]any)
		f, _ := m["field"].(string)
		c, _ := m["code"].(string)
		out[f] = c
	}
	return out
}

// Do sends a request to /api/v1+path with a JSON body (when non-nil), the browser's
// CSRF token, and extra header name/value pairs.
func (b *Browser) Do(method, path string, body any, headers ...string) Response {
	b.e.T.Helper()
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			b.e.T.Fatal(err)
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, b.e.Srv.URL+"/api/v1"+path, rd)
	if err != nil {
		b.e.T.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if b.CSRF != "" {
		req.Header.Set("X-CSRF-Token", b.CSRF)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := b.HTTP.Do(req)
	if err != nil {
		b.e.T.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := Response{Status: resp.StatusCode, Header: resp.Header, Raw: string(raw)}
	_ = json.Unmarshal(raw, &out.Body)
	return out
}

// Expect fails the test unless r has the status and (when non-empty) error code.
func (b *Browser) Expect(r Response, status int, code string) Response {
	b.e.T.Helper()
	if r.Status != status || (code != "" && r.ErrCode() != code) {
		b.e.T.Fatalf("got %d %s, want %d %s", r.Status, r.Raw, status, code)
	}
	return r
}

// Register signs up an account with Password.
func (e *Env) Register(email, name string) {
	e.T.Helper()
	b := e.Browser()
	b.Expect(b.Do("POST", "/auth/register", map[string]string{
		"email": email, "password": Password, "display_name": name, "locale": "vi", "timezone": "Asia/Ho_Chi_Minh",
	}), http.StatusAccepted, "")
}

// Login signs in and stores the CSRF token on success.
func (b *Browser) Login(email, password string) Response {
	b.e.T.Helper()
	r := b.Do("POST", "/auth/login", map[string]string{"email": email, "password": password})
	if r.Status == http.StatusOK {
		b.CSRF, _ = r.Body["csrf_token"].(string)
	}
	return r
}

// SignUp registers, verifies the email and signs in, returning the browser and the
// user's id.
func (e *Env) SignUp(email, name string) (*Browser, string) {
	e.T.Helper()
	e.Register(email, name)
	b := e.Browser()
	b.Expect(b.Do("POST", "/auth/verify-email", map[string]string{"token": e.Outbox.LastToken(e.T, "verify_email", email)}), http.StatusOK, "")
	r := b.Expect(b.Login(email, Password), http.StatusOK, "")
	id, _ := r.Body["user"].(map[string]any)["id"].(string)
	return b, id
}

// MustURL parses s or panics.
func MustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
