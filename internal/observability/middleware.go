package observability

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// statusResponseWriter wraps http.ResponseWriter to capture the status code.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// HTTPMetrics returns HTTP middleware that records request metrics.
// It measures request duration, counts total requests, and counts error
// responses (status >= 400). Metrics are tagged with method, path, and status.
//
// Usage:
//
//	handler := observability.HTTPMetrics(metrics)(yourHandler)
func HTTPMetrics(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &statusResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			duration := float64(time.Since(start).Milliseconds())
			status := wrapped.statusCode

			attrs := otelmetric.WithAttributes(
				attribute.String("method", r.Method),
				attribute.String("path", r.URL.Path),
				attribute.String("status", strconv.Itoa(status)),
			)

			metrics.HTTPRequestDuration.Record(r.Context(), duration, attrs)
			metrics.HTTPRequestTotal.Add(r.Context(), 1, attrs)

			if status >= 400 {
				metrics.HTTPRequestErrors.Add(r.Context(), 1, attrs)
			}
		})
	}
}
