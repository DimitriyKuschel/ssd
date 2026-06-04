package providers

import (
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// knownEndpoints is the allowlist of routed API paths. Anything else (random
// scans, 404s) is bucketed into a single label to avoid Prometheus cardinality
// blow-up, since the outer mux forwards every unmatched path through this handler.
var knownEndpoints = map[string]struct{}{
	"/":             {},
	"/list":         {},
	"/fingerprints": {},
	"/fingerprint":  {},
	"/channels":     {},
	"/hit":          {},
}

func normalizeEndpoint(path string) string {
	if _, ok := knownEndpoints[path]; ok {
		return path
	}
	return "other"
}

func MetricsMiddleware(metrics MetricsProviderInterface, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		endpoint := normalizeEndpoint(r.URL.Path)
		metrics.IncRequestsTotal(endpoint, sw.status)
		metrics.ObserveRequestDuration(endpoint, duration)
	})
}
