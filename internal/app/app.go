// Package app is the composition root: it wires configuration, infrastructure and
// modules together and runs the requested command.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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
	"reefcare/internal/platform/jobs"
	"reefcare/internal/platform/logging"
	"reefcare/internal/platform/mail"
	"reefcare/internal/platform/metrics"
	"reefcare/internal/tanks"
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
	// Metrics is optional; when set, every request is recorded.
	Metrics *metrics.Metrics
}

// App is the wired application: the HTTP handler and the background job runner, which
// share the same modules.
type App struct {
	Handler http.Handler
	Jobs    *jobs.Runner
}

// New wires every module into the root router and registers their background jobs.
func New(d Deps) App {
	runner := jobs.NewRunner(d.Pool, d.Clock, d.Log, jobs.Config{Workers: d.Config.JobWorkers, PollInterval: d.Config.JobPollInterval})
	if d.Metrics != nil {
		runner.Observe(d.Metrics)
	}
	idSvc := identity.NewService(d.Pool, d.Clock, identity.NewHasher(d.Argon2, d.Config.Argon2MaxConcurrent), d.Mailer,
		identity.DefaultLimits(d.Clock), identity.DefaultConfig(d.Config.PublicBaseURL), d.Log)
	// Every module holding user data registers how to erase it (docs/security.md §9).
	idSvc.OnErase(tanks.EraseUserData)
	idSvc.RegisterJobs(runner)
	idHTTP := identity.NewHandler(idSvc, d.Log, d.Config.SecureCookies())
	tankHTTP := tanks.NewHandler(tanks.NewService(d.Pool, d.Clock, tanks.DefaultLimits(d.Clock), d.Log), idHTTP.RequireAuth, d.Log)

	var observe func(http.Handler) http.Handler
	if d.Metrics != nil {
		observe = d.Metrics.Middleware
	}
	handler := httpserver.NewRouter(d.Log, httpserver.Options{
		Observe:        observe,
		Ready:          d.Ready,
		TrustedProxies: d.Config.TrustedProxies,
		AllowedOrigins: d.Config.AllowedOrigins,
		Mount: func(api chi.Router) {
			api.Use(idHTTP.Authenticate)
			idHTTP.Mount(api)
			tankHTTP.Mount(api)
		},
	})
	return App{Handler: handler, Jobs: runner}
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
	var metricsReg *metrics.Metrics
	if cfg.MetricsAddr != "" {
		metricsReg = metrics.New(pool)
	}
	application := New(Deps{
		Config: cfg, Log: log, Pool: pool, Clock: clock.System{}, Mailer: sender, Metrics: metricsReg,
		Argon2: identity.DefaultArgon2Params,
		Ready: map[string]httpserver.ReadinessCheck{
			"database":   pool.Ping,
			"migrations": m.CheckCompatible,
		},
	})
	srv := httpserver.NewServer(cfg.HTTPAddr, application.Handler)

	// Jobs run until the HTTP server has drained; started jobs then finish (each is
	// bounded by its timeout) before the pool closes. Unstarted jobs stay queued.
	jobsCtx, stopJobs := context.WithCancel(context.WithoutCancel(ctx))
	jobsDone := make(chan struct{})
	go func() {
		defer close(jobsDone)
		log.Info("job runner started", "workers", cfg.JobWorkers)
		application.Jobs.Run(jobsCtx)
	}()
	defer func() {
		stopJobs()
		select {
		case <-jobsDone:
			return
		case <-time.After(cfg.ShutdownTimeout):
		}
		// Past the deadline: cancel running handlers so the pool can close. Their jobs
		// are re-queued once the lease expires.
		log.Error("job runner did not stop in time; cancelling running jobs")
		application.Jobs.Abort()
		select {
		case <-jobsDone:
		case <-time.After(15 * time.Second):
			log.Error("jobs still running after cancellation")
		}
	}()

	errCh := make(chan error, 2)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	var metricsSrv *http.Server
	if metricsReg != nil {
		if host, _, _ := net.SplitHostPort(cfg.MetricsAddr); cfg.IsProduction() && (host == "" || host == "0.0.0.0" || host == "::") {
			log.Warn("metrics listen on every interface; keep the port off public networks (firewall or private bind address)", "addr", cfg.MetricsAddr)
		}
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", metricsReg.Handler())
		metricsSrv = httpserver.NewServer(cfg.MetricsAddr, mux)
		go func() {
			log.Info("metrics listening", "addr", cfg.MetricsAddr)
			if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("metrics: %w", err)
			}
		}()
	}

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down", "timeout", cfg.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()
	if metricsSrv != nil {
		// Metrics stay up while the API drains, then stop.
		defer func() { _ = metricsSrv.Shutdown(shutdownCtx) }()
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("shutdown complete")
	return nil
}
