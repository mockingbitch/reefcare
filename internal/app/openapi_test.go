package app

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"reefcare/internal/config"
	"reefcare/internal/identity"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/mail"
)

var (
	specPath   = regexp.MustCompile(`^  (/\S*):\s*$`)
	specMethod = regexp.MustCompile(`^    (get|put|post|patch|delete|head|options):`)
)

// TestOpenAPIMatchesRoutes keeps api/openapi.yaml honest: every route served by the
// application is documented, and every documented operation exists.
func TestOpenAPIMatchesRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]bool{}
	inPaths, current := false, ""
	for line := range strings.Lines(string(raw)) {
		line = strings.TrimRight(line, "\n")
		switch {
		case line == "paths:":
			inPaths = true
		case inPaths && line != "" && !strings.HasPrefix(line, " "):
			inPaths = false
		case inPaths && specPath.MatchString(line):
			current = specPath.FindStringSubmatch(line)[1]
		case inPaths && current != "" && specMethod.MatchString(line):
			documented[strings.ToUpper(specMethod.FindStringSubmatch(line)[1])+" "+current] = true
		}
	}

	application := New(Deps{
		Config: config.Config{PublicBaseURL: "http://reefcare.test"},
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Clock:  clock.System{}, Mailer: mail.LogSender{Log: slog.New(slog.NewTextHandler(io.Discard, nil))},
		Argon2: identity.Argon2Params{MemoryKiB: 1024, Iterations: 1, Parallelism: 1},
	})
	routes, ok := application.Handler.(chi.Routes)
	if !ok {
		t.Fatalf("router is %T, not chi.Routes", application.Handler)
	}
	served := map[string]bool{}
	err = chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if route != "/" {
			route = strings.TrimSuffix(route, "/")
		}
		served[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var missing, stale []string
	for r := range served {
		if !documented[r] {
			missing = append(missing, r)
		}
	}
	for r := range documented {
		if !served[r] {
			stale = append(stale, r)
		}
	}
	slices.Sort(missing)
	slices.Sort(stale)
	if len(missing) > 0 {
		t.Errorf("routes missing from api/openapi.yaml:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("api/openapi.yaml documents routes that do not exist:\n  %s", strings.Join(stale, "\n  "))
	}
	if len(documented) < 10 {
		t.Fatalf("parsed only %d operations from the spec; parser broken?", len(documented))
	}
}
