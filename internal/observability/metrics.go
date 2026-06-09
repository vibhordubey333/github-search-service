package observability

import "github.com/prometheus/client_golang/prometheus"

var (
	RPCRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests by method and status code.",
		},
		[]string{"method", "code"},
	)

	/*
		RPCLatency measures end-to-end RPC latency.
		fast (50ms), normal (200ms), slow (1s), very slow (5s).
	*/
	RPCLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds.",
			Buckets: []float64{0.05, 0.1, 0.2, 0.5, 1, 2, 5},
		},
		[]string{"method"},
	)

	// GithubAPILatency measures time spent calling the GitHub API. Separate from RPC latency to isolate the external dependency.
	GithubAPILatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "github_api_duration_seconds",
			Help:    "GitHub API call duration in seconds.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5},
		},
		[]string{"endpoint", "status"},
	)

	// GithubRateLimitErrors tracks how often we hit GitHub rate limits. Alert when this is non-zero for more than 5 minutes.
	GithubRateLimitErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "github_rate_limit_errors_total",
		Help: "Total number of GitHub rate limit errors.",
	})
)

func init() {
	prometheus.MustRegister(
		RPCRequestsTotal,
		RPCLatency,
		GithubAPILatency,
		GithubRateLimitErrors,
	)
}
