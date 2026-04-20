package xcclient

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds all Prometheus metrics for the XC API client.
type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RateLimitHits   *prometheus.CounterVec
	RetriesTotal    *prometheus.CounterVec
	UpdatesSkipped  *prometheus.CounterVec
}

// NewMetrics creates all metrics and registers them with reg if reg is non-nil.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "f5xc_api_requests_total",
				Help: "Total number of F5 XC API requests.",
			},
			[]string{"endpoint", "method", "status_code"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "f5xc_api_request_duration_seconds",
				Help:    "Duration of F5 XC API requests in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"endpoint", "method"},
		),
		RateLimitHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "f5xc_api_rate_limit_hits_total",
				Help: "Total number of F5 XC API rate limit hits.",
			},
			[]string{"endpoint"},
		),
		RetriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "f5xc_api_retries_total",
				Help: "Total number of F5 XC API request retries.",
			},
			[]string{"endpoint", "reason"},
		),
		UpdatesSkipped: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "f5xc_api_updates_skipped_total",
				Help: "Total number of F5 XC API updates skipped due to no change.",
			},
			[]string{"endpoint"},
		),
	}

	if reg != nil {
		reg.MustRegister(
			m.RequestsTotal,
			m.RequestDuration,
			m.RateLimitHits,
			m.RetriesTotal,
			m.UpdatesSkipped,
		)
	}

	return m
}
