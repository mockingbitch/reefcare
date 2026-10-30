// Package metrics exposes Prometheus metrics (docs/architecture.md §8): HTTP traffic by
// route pattern, database pool state, and Go runtime/process metrics. It uses its own
// registry (no global state) and is served on a separate, internal-only listener.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns the registry and the HTTP instruments.
type Metrics struct {
	reg      *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

// New registers runtime, process and (when pool is non-nil) pool metrics.
func New(pool *pgxpool.Pool) *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "reefcare_http_requests_total", Help: "HTTP requests by method, route pattern and status code.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "reefcare_http_request_duration_seconds", Help: "HTTP request latency by method and route pattern.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"method", "route"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "reefcare_http_requests_in_flight", Help: "HTTP requests currently being served.",
		}),
	}
	reg.MustRegister(m.requests, m.duration, m.inFlight,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if pool != nil {
		reg.MustRegister(poolCollector{pool: pool})
	}
	return m
}

// Handler serves the metrics in the Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{Registry: m.reg})
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Middleware records every request. The route label is chi's route pattern (never the
// raw path), so IDs don't explode label cardinality and tokens never become labels.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.inFlight.Inc()
		defer m.inFlight.Dec()
		rec := &recorder{ResponseWriter: w}
		defer func() {
			route := "unmatched"
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				route = rc.RoutePattern()
			}
			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			method := normalizeMethod(r.Method)
			m.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
			m.duration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
		}()
		next.ServeHTTP(rec, r)
	})
}

// normalizeMethod bounds the method label to known verbs.
func normalizeMethod(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodOptions:
		return m
	default:
		return "OTHER"
	}
}

// poolCollector reports pgxpool statistics at scrape time.
type poolCollector struct{ pool *pgxpool.Pool }

var (
	poolTotal    = prometheus.NewDesc("reefcare_db_pool_connections", "Database pool connections by state.", []string{"state"}, nil)
	poolMax      = prometheus.NewDesc("reefcare_db_pool_max_connections", "Configured maximum pool size.", nil, nil)
	poolAcquires = prometheus.NewDesc("reefcare_db_pool_acquires_total", "Successful connection acquisitions.", nil, nil)
	poolWaits    = prometheus.NewDesc("reefcare_db_pool_empty_acquires_total", "Acquisitions that had to wait for a connection.", nil, nil)
	poolWaitTime = prometheus.NewDesc("reefcare_db_pool_acquire_wait_seconds_total", "Total time spent waiting for a connection.", nil, nil)
)

func (poolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{poolTotal, poolMax, poolAcquires, poolWaits, poolWaitTime} {
		ch <- d
	}
}

func (c poolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(poolTotal, prometheus.GaugeValue, float64(s.AcquiredConns()), "acquired")
	ch <- prometheus.MustNewConstMetric(poolTotal, prometheus.GaugeValue, float64(s.IdleConns()), "idle")
	ch <- prometheus.MustNewConstMetric(poolTotal, prometheus.GaugeValue, float64(s.ConstructingConns()), "constructing")
	ch <- prometheus.MustNewConstMetric(poolMax, prometheus.GaugeValue, float64(s.MaxConns()))
	ch <- prometheus.MustNewConstMetric(poolAcquires, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolWaits, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolWaitTime, prometheus.CounterValue, s.AcquireDuration().Seconds())
}
