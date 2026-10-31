// Package config loads and validates application configuration from the environment.
package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Environment names.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Config is the complete, validated runtime configuration.
type Config struct {
	Env      string
	HTTPAddr string
	// MetricsAddr serves Prometheus metrics on a separate listener that must not be
	// exposed publicly; empty disables metrics.
	MetricsAddr     string
	PublicBaseURL   string
	ShutdownTimeout time.Duration
	TrustedProxies  []netip.Prefix

	DatabaseURL   string
	DBMaxConns    int32
	DBConnTimeout time.Duration

	LogLevel  slog.Level
	LogFormat string // "json" or "text"

	// AllowedOrigins may send state-changing API requests: the origin of PublicBaseURL
	// plus ALLOWED_ORIGINS (e.g. a frontend dev server).
	AllowedOrigins []string

	// SMTP; when SMTPHost is empty outside production, emails are only logged.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	MailFrom     string

	// Argon2MaxConcurrent bounds simultaneous password hashes (64 MiB each);
	// 0 means GOMAXPROCS. Keep it × 64 MiB well under the container memory limit.
	Argon2MaxConcurrent int

	// JobWorkers is the number of background jobs run concurrently by this instance.
	JobWorkers      int
	JobPollInterval time.Duration

	// LLMProvider selects the optional narrative adapter of Reef Intelligence
	// (docs/knowledge-engine.md §8). Only "none" exists: deterministic assessments work
	// without any LLM, and adding a provider is a reviewed decision (open item IN13).
	LLMProvider string
}

// SecureCookies reports whether cookies must carry the Secure attribute.
func (c Config) SecureCookies() bool { return strings.HasPrefix(c.PublicBaseURL, "https://") }

// IsProduction reports whether the app runs in production.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// Load reads configuration using lookup (normally os.LookupEnv) and returns every
// problem found at once, so a misconfigured deployment fails with a complete report.
func Load(lookup func(string) (string, bool)) (Config, error) {
	p := parser{lookup: lookup}

	cfg := Config{
		Env:                 p.oneOf("APP_ENV", EnvDevelopment, EnvDevelopment, EnvTest, EnvProduction),
		HTTPAddr:            p.str("HTTP_ADDR", ":8080"),
		MetricsAddr:         p.str("METRICS_ADDR", ""),
		PublicBaseURL:       p.str("PUBLIC_BASE_URL", ""),
		ShutdownTimeout:     p.duration("SHUTDOWN_TIMEOUT", 30*time.Second),
		TrustedProxies:      p.proxyPrefixes("TRUSTED_PROXY_CIDRS"),
		DatabaseURL:         p.required("DATABASE_URL"),
		DBMaxConns:          int32(p.intRange("DB_MAX_CONNS", 10, 1, 500)), //nolint:gosec // bounded to 1..500 by intRange
		DBConnTimeout:       p.duration("DB_CONNECT_TIMEOUT", 5*time.Second),
		LogLevel:            p.logLevel("LOG_LEVEL", slog.LevelInfo),
		LogFormat:           p.oneOf("LOG_FORMAT", "json", "json", "text"),
		SMTPHost:            p.str("SMTP_HOST", ""),
		SMTPPort:            p.intRange("SMTP_PORT", 587, 1, 65535),
		SMTPUsername:        p.str("SMTP_USERNAME", ""),
		SMTPPassword:        p.str("SMTP_PASSWORD", ""),
		MailFrom:            p.str("MAIL_FROM", "ReefCare <no-reply@localhost>"),
		Argon2MaxConcurrent: p.intRange("ARGON2_MAX_CONCURRENCY", 0, 0, 256),
		JobWorkers:          p.intRange("JOB_WORKERS", 4, 1, 64),
		JobPollInterval:     p.duration("JOB_POLL_INTERVAL", time.Second),
		LLMProvider:         p.oneOf("LLM_PROVIDER", "none", "none"),
	}
	if cfg.PublicBaseURL == "" && !cfg.IsProduction() {
		cfg.PublicBaseURL = "http://localhost:8080"
	}
	cfg.PublicBaseURL = strings.TrimRight(cfg.PublicBaseURL, "/")
	if cfg.IsProduction() {
		if cfg.SMTPHost == "" {
			p.fail("SMTP_HOST", "is required in production")
		}
		if _, set := p.get("MAIL_FROM"); !set {
			p.fail("MAIL_FROM", "is required in production")
		}
	}

	if cfg.MetricsAddr != "" {
		if _, _, err := net.SplitHostPort(cfg.MetricsAddr); err != nil {
			p.fail("METRICS_ADDR", "must be host:port or :port")
		} else if cfg.MetricsAddr == cfg.HTTPAddr {
			p.fail("METRICS_ADDR", "must differ from HTTP_ADDR (metrics stay off the public listener)")
		}
	}

	if cfg.DatabaseURL != "" {
		if u, err := url.Parse(cfg.DatabaseURL); err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
			p.fail("DATABASE_URL", "must be a postgres:// or postgresql:// URL")
		}
	}
	if cfg.PublicBaseURL != "" {
		if origin, err := originOf(cfg.PublicBaseURL); err != nil {
			p.fail("PUBLIC_BASE_URL", "must be an absolute URL")
		} else if cfg.IsProduction() && !strings.HasPrefix(origin, "https://") {
			p.fail("PUBLIC_BASE_URL", "must use https in production")
		} else {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
		}
	} else if cfg.IsProduction() {
		p.fail("PUBLIC_BASE_URL", "is required in production")
	}
	if v, ok := p.get("ALLOWED_ORIGINS"); ok {
		for _, s := range strings.Split(v, ",") {
			origin, err := originOf(strings.TrimSpace(s))
			if err != nil || origin != strings.TrimSpace(s) {
				p.fail("ALLOWED_ORIGINS", "must be a comma-separated list of origins such as http://localhost:5173")
				break
			}
			if cfg.IsProduction() && !strings.HasPrefix(origin, "https://") {
				p.fail("ALLOWED_ORIGINS", "must use https in production")
				break
			}
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
		}
	}
	// Production runs behind a reverse proxy (docs/architecture.md §1). Without trusting it,
	// every client shares the proxy's IP and per-IP limits become one global bucket.
	if cfg.IsProduction() && len(cfg.TrustedProxies) == 0 {
		if _, set := p.get("TRUSTED_PROXY_CIDRS"); !set {
			p.fail("TRUSTED_PROXY_CIDRS", "is required in production")
		}
	}

	if len(p.errs) > 0 {
		return Config{}, &Error{Problems: p.errs}
	}
	return cfg, nil
}

// originOf returns scheme://host[:port] of an absolute http(s) URL.
func originOf(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("not an absolute http(s) URL")
	}
	return u.Scheme + "://" + u.Host, nil
}

// Error lists every configuration problem found.
type Error struct {
	Problems []error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("invalid configuration:")
	for _, p := range e.Problems {
		b.WriteString("\n  - ")
		b.WriteString(p.Error())
	}
	return b.String()
}

// Unwrap exposes the individual problems to errors.Is / errors.As.
func (e *Error) Unwrap() []error { return e.Problems }

type parser struct {
	lookup func(string) (string, bool)
	errs   []error
}

func (p *parser) fail(key, msg string) {
	p.errs = append(p.errs, fmt.Errorf("%s %s", key, msg))
}

func (p *parser) get(key string) (string, bool) {
	v, ok := p.lookup(key)
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}

func (p *parser) str(key, def string) string {
	if v, ok := p.get(key); ok {
		return v
	}
	return def
}

func (p *parser) required(key string) string {
	v, ok := p.get(key)
	if !ok {
		p.fail(key, "is required")
	}
	return v
}

func (p *parser) oneOf(key, def string, allowed ...string) string {
	v := p.str(key, def)
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	p.fail(key, fmt.Sprintf("must be one of %s", strings.Join(allowed, ", ")))
	return def
}

func (p *parser) intRange(key string, def, minV, maxV int) int {
	v, ok := p.get(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < minV || n > maxV {
		p.fail(key, fmt.Sprintf("must be an integer between %d and %d", minV, maxV))
		return def
	}
	return n
}

func (p *parser) duration(key string, def time.Duration) time.Duration {
	v, ok := p.get(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		p.fail(key, "must be a positive duration such as 30s or 1m")
		return def
	}
	return d
}

// proxyPrefixes parses a comma-separated list of proxy CIDRs or bare IPs; empty means
// none. Ranges wider than /8 (IPv4) or /32 (IPv6) are rejected: trusting them would let
// arbitrary clients spoof X-Forwarded-For and defeat per-IP rate limits.
func (p *parser) proxyPrefixes(key string) []netip.Prefix {
	v, ok := p.get(key)
	if !ok {
		return nil
	}
	var out []netip.Prefix
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		pfx, err := netip.ParsePrefix(s)
		if err != nil {
			addr, aerr := netip.ParseAddr(s)
			if aerr != nil {
				p.fail(key, "must be a comma-separated list of CIDRs or IPs such as 10.0.0.0/8,192.0.2.10")
				return nil
			}
			pfx = netip.PrefixFrom(addr, addr.BitLen())
		}
		if a := pfx.Addr(); a.Is4In6() && pfx.Bits() >= 96 {
			pfx = netip.PrefixFrom(a.Unmap(), pfx.Bits()-96)
		}
		if minBits := map[bool]int{true: 8, false: 32}[pfx.Addr().Is4()]; pfx.Bits() < minBits {
			p.fail(key, fmt.Sprintf("entry %s is too wide; trust only your reverse proxies", s))
			return nil
		}
		out = append(out, pfx.Masked())
	}
	return out
}

func (p *parser) logLevel(key string, def slog.Level) slog.Level {
	v, ok := p.get(key)
	if !ok {
		return def
	}
	var l slog.Level
	if err := l.UnmarshalText([]byte(v)); err != nil {
		p.fail(key, "must be one of debug, info, warn, error")
		return def
	}
	return l
}
