package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:      "http_request_duration_seconds",
			Help:      "Duration of HTTP requests in seconds",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			Namespace: "totalcomp",
		},
		[]string{"method", "path", "status"},
	)

	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
			Namespace: "totalcomp",
		},
		[]string{"method", "path", "status"},
	)

	ActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      "http_requests_active",
			Help:      "Number of active HTTP requests",
			Namespace: "totalcomp",
		},
	)

	// Database Metrics
	DbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:      "db_query_duration_seconds",
			Help:      "Duration of database queries in seconds",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			Namespace: "totalcomp",
		},
		[]string{"query_type"},
	)

	DbOpenConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      "db_open_connections",
			Help:      "Number of established database connections",
			Namespace: "totalcomp",
		},
	)

	DbInUseConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      "db_in_use_connections",
			Help:      "Number of database connections currently in use",
			Namespace: "totalcomp",
		},
	)

	DbIdleConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      "db_idle_connections",
			Help:      "Number of idle database connections",
			Namespace: "totalcomp",
		},
	)

	// NEW: Simple Up/Down gauge for Postgres (1=Up, 0=Down)
	DbUp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      "db_up",
			Help:      "1 if the database is pingable, 0 if not",
			Namespace: "totalcomp",
		},
	)

	// Business Metrics
	TotalCompCalculations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name:      "calculations_total",
			Help:      "Total number of compensation calculations performed",
			Namespace: "totalcomp",
		},
	)

	CalculationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:      "calculation_duration_seconds",
			Help:      "Duration of compensation calculations",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2},
			Namespace: "totalcomp",
		},
	)

	ActiveUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      "active_users",
			Help:      "Number of currently active users (based on session activity or similar)",
			Namespace: "totalcomp",
		},
	)
)

var (
	AuthErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "http_auth_errors_total",
			Help:      "Total number of authentication errors",
			Namespace: "totalcomp",
		},
		[]string{"reason"},
	)

	ValidationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "http_validation_errors_total",
			Help:      "Total number of validation errors",
			Namespace: "totalcomp",
		},
		[]string{"field"},
	)

	InternalErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name:      "http_internal_errors_total",
			Help:      "Total number of internal server errors (panics or unhandled)",
			Namespace: "totalcomp",
		},
	)
)
