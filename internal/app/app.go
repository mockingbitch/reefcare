// Package app is the composition root: it wires configuration, infrastructure and
// modules together and runs the requested command.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/db"
	"reefcare/internal/config"
	"reefcare/internal/identity"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/httpserver"
	"reefcare/internal/platform/logging"
	"reefcare/internal/platform/mail"
)

const usage = `usage: api [command]

commands:
  serve            run the HTTP API (default)
  migrate up       apply all pending migrations
  migrate down     roll back the most recent migration
  migrate status   print migration status`

// Run executes the command in args (without the program name) until ctx is cancelled.
func Run(ctx context.Context, args []string, lookupEnv func(string) (string, bool), stdout, stderr io.Writer) error {
	cfg, err := config.Load(lookupEnv)
	if err != nil {
		return err
	}
	log := logging.New(stderr, cfg.LogLevel, cfg.LogFormat)

	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "serve":
		return serve(ctx, cfg, log)
	case "migrate":
		if len(args) != 2 {
			return errors.New(usage)
		}
		return migrate(ctx, cfg, log, args[1], stdout)
	default:
		return errors.New(usage)
	}
}

func openDB(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	return database.Open(ctx, database.PoolConfig{
		URL:            cfg.DatabaseURL,
		MaxConns:       cfg.DBMaxConns,
		ConnectTimeout: cfg.DBConnTimeout,
	})
}

func migrate(ctx context.Context, cfg config.Config, log *slog.Logger, action string, stdout io.Writer) error {
	pool, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	m, err := database.NewMigrator(pool, db.Migrations, log)
	if err != nil {
		return err
	}
	defer m.Close()

	switch action {
	case "up":
		return m.Up(ctx)
	case "down":
		return m.Down(ctx)
	case "status":
		statuses, err := m.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			applied := "pending"
			if !s.AppliedAt.IsZero() {
				applied = s.AppliedAt.UTC().Format(time.RFC3339)
			}
			fmt.Fprintf(stdout, "%-6d %-40s %s\n", s.Source.Version, s.Source.Path, applied)
		}
		return nil
	default:
		return errors.New(usage)
	}
}

// Deps are the collaborators the HTTP handler is built from. Tests supply their own
// clock, mailer and cheaper Argon2 parameters.
type Deps struct {
	Config config.Config
	Log    *slog.Logger
	Pool   *pgxpool.Pool
	Clock  clock.Clock
	Mailer mail.Sender
	Argon2 identity.Argon2Params
	Ready  map[string]httpserver.ReadinessCheck
}

// NewHTTPHandler wires every module into the root router. The returned close function
// waits for modules' background work and must be called after the server stops.
func NewHTTPHandler(d Deps) (http.Handler, func(context.Context) error) {
	idSvc := identity.NewService(d.Pool, d.Clock, identity.NewHasher(d.Argon2, d.Config.Argon2MaxConcurrent), d.Mailer,
		identity.DefaultLimits(d.Clock), identity.DefaultConfig(d.Config.PublicBaseURL), d.Log)
	idHTTP := identity.NewHandler(idSvc, d.Log, d.Config.SecureCookies())

	return httpserver.NewRouter(d.Log, httpserver.Options{
		Ready:          d.Ready,
		TrustedProxies: d.Config.TrustedProxies,
		AllowedOrigins: d.Config.AllowedOrigins,
		Mount: func(api chi.Router) {
			api.Use(idHTTP.Authenticate)
			idHTTP.Mount(api)
		},
	}), idSvc.Close
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	m, err := database.NewMigrator(pool, db.Migrations, log)
	if err != nil {
		return err
	}
	defer m.Close()

	var sender mail.Sender = mail.LogSender{Log: log}
	if cfg.SMTPHost != "" {
		smtpSender, err := mail.NewSMTPSender(mail.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
			From: cfg.MailFrom, RequireTLS: cfg.IsProduction(),
		})
		if err != nil {
			return err
		}
		sender = smtpSender
	}
	mailer := mail.NewAsync(sender, log, 1000)

	router, closeModules := NewHTTPHandler(Deps{
		Config: cfg, Log: log, Pool: pool, Clock: clock.System{}, Mailer: mailer,
		Argon2: identity.DefaultArgon2Params,
		Ready: map[string]httpserver.ReadinessCheck{
			"database":   pool.Ping,
			"migrations": m.CheckCompatible,
		},
	})
	srv := httpserver.NewServer(cfg.HTTPAddr, router)
	// Runs after Shutdown: module background work may still queue emails, so it is
	// drained before the mail queue closes.
	defer func() {
		drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := closeModules(drainCtx); err != nil {
			log.Error("background work", "err", err)
		}
		if err := mailer.Close(drainCtx); err != nil {
			log.Error("email queue", "err", err)
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down", "timeout", cfg.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("shutdown complete")
	return nil
}
