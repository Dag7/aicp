package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aicp",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "aicp",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "aicp",
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes.",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 6),
		},
		[]string{"method", "path"},
	)
)

// Metrics returns middleware that records Prometheus HTTP metrics.
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(rw.statusCode)
			path := r.URL.Path

			httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
			httpResponseSize.WithLabelValues(r.Method, path).Observe(float64(rw.written))
		})
	}
}
