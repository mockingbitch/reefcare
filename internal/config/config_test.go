package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{"DATABASE_URL": "postgres://u:p@localhost:5432/reefcare"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != EnvDevelopment || cfg.HTTPAddr != ":8080" || cfg.DBMaxConns != 10 ||
		cfg.ShutdownTimeout != 30*time.Second || cfg.LogLevel != slog.LevelInfo || cfg.LogFormat != "json" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestAllowedOrigins(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"DATABASE_URL": "postgres://localhost/db", "PUBLIC_BASE_URL": "https://reefcare.example/app/",
		"ALLOWED_ORIGINS": "http://localhost:5173",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://reefcare.example", "http://localhost:5173"}
	if strings.Join(cfg.AllowedOrigins, ",") != strings.Join(want, ",") {
		t.Errorf("allowed origins = %v, want %v", cfg.AllowedOrigins, want)
	}
	if cfg.PublicBaseURL != "https://reefcare.example/app" || !cfg.SecureCookies() {
		t.Errorf("public base url = %q secure=%v", cfg.PublicBaseURL, cfg.SecureCookies())
	}
}

func TestLoadReportsAllErrors(t *testing.T) {
	_, err := Load(env(map[string]string{
		"APP_ENV":          "staging",
		"DB_MAX_CONNS":     "0",
		"SHUTDOWN_TIMEOUT": "soon",
		"LOG_LEVEL":        "loud",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, key := range []string{"APP_ENV", "DATABASE_URL", "DB_MAX_CONNS", "SHUTDOWN_TIMEOUT", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s: %v", key, err)
		}
	}
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "non-postgres database url",
			env:     map[string]string{"DATABASE_URL": "mysql://localhost/db"},
			wantErr: "DATABASE_URL",
		},
		{
			name:    "production requires public base url",
			env:     map[string]string{"APP_ENV": "production", "DATABASE_URL": "postgres://localhost/db"},
			wantErr: "PUBLIC_BASE_URL is required",
		},
		{
			name: "production requires https",
			env: map[string]string{
				"APP_ENV": "production", "DATABASE_URL": "postgres://localhost/db",
				"PUBLIC_BASE_URL": "http://reefcare.example",
			},
			wantErr: "https",
		},
		{
			name: "valid production",
			env: map[string]string{
				"APP_ENV": "production", "DATABASE_URL": "postgresql://localhost/db",
				"PUBLIC_BASE_URL": "https://reefcare.example", "TRUSTED_PROXY_CIDRS": "10.0.0.2",
				"SMTP_HOST": "smtp.example", "MAIL_FROM": "ReefCare <no-reply@reefcare.example>",
			},
		},
		{
			name: "production requires smtp",
			env: map[string]string{
				"APP_ENV": "production", "DATABASE_URL": "postgresql://localhost/db",
				"PUBLIC_BASE_URL": "https://reefcare.example", "TRUSTED_PROXY_CIDRS": "10.0.0.2",
			},
			wantErr: "SMTP_HOST is required",
		},
		{
			name: "production allowed origins must be https",
			env: map[string]string{
				"APP_ENV": "production", "DATABASE_URL": "postgresql://localhost/db",
				"PUBLIC_BASE_URL": "https://reefcare.example", "TRUSTED_PROXY_CIDRS": "10.0.0.2",
				"SMTP_HOST": "smtp.example", "MAIL_FROM": "ReefCare <no-reply@reefcare.example>",
				"ALLOWED_ORIGINS": "http://admin.reefcare.example",
			},
			wantErr: "ALLOWED_ORIGINS must use https",
		},
		{
			name:    "allowed origins must be bare origins",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/db", "ALLOWED_ORIGINS": "http://localhost:5173/app"},
			wantErr: "ALLOWED_ORIGINS",
		},
		{
			name:    "invalid trusted proxy cidr",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/db", "TRUSTED_PROXY_CIDRS": "10.0.0.0/8, nonsense"},
			wantErr: "TRUSTED_PROXY_CIDRS",
		},
		{
			name: "valid trusted proxies including bare ips",
			env:  map[string]string{"DATABASE_URL": "postgres://localhost/db", "TRUSTED_PROXY_CIDRS": "10.0.0.0/8, 172.16.0.0/12, 192.0.2.10, 2001:db8::1"},
		},
		{
			name:    "trust-everything ipv4 rejected",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/db", "TRUSTED_PROXY_CIDRS": "0.0.0.0/0"},
			wantErr: "too wide",
		},
		{
			name:    "trust-everything ipv6 rejected",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/db", "TRUSTED_PROXY_CIDRS": "::/0"},
			wantErr: "too wide",
		},
		{
			name: "production requires trusted proxies",
			env: map[string]string{
				"APP_ENV": "production", "DATABASE_URL": "postgresql://localhost/db",
				"PUBLIC_BASE_URL": "https://reefcare.example",
			},
			wantErr: "TRUSTED_PROXY_CIDRS is required",
		},
		{
			name:    "whitespace-only value counts as missing",
			env:     map[string]string{"DATABASE_URL": "   "},
			wantErr: "DATABASE_URL is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(env(tt.env))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
