package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	// service layer — бизнес операции
	CartServiceRequestsTotal   *prometheus.CounterVec
	CartServiceRequestDuration *prometheus.HistogramVec

	// grpc transport layer
	CartGRPCRequestsTotal   *prometheus.CounterVec
	CartGRPCRequestDuration *prometheus.HistogramVec

	// mongodb layer
	CartMongoRequestsTotal   *prometheus.CounterVec
	CartMongoRequestDuration *prometheus.HistogramVec
}

var (
	metricsInstance *Metrics
	metricsOnce     sync.Once
)

func MustMetrics() *Metrics {
	metricsOnce.Do(func() {
		metricsInstance = newMetrics()
	})
	return metricsInstance
}

func newMetrics() *Metrics {
	m := &Metrics{
		CartServiceRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "cart",
				Subsystem: "service",
				Name:      "requests_total",
				Help:      "Total number of cart service requests.",
			},
			[]string{"method", "status"},
		),
		CartServiceRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "cart",
				Subsystem: "service",
				Name:      "request_duration_seconds",
				Help:      "Cart service request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		CartGRPCRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "cart",
				Subsystem: "grpc",
				Name:      "requests_total",
				Help:      "Total number of gRPC requests handled by cart transport.",
			},
			[]string{"method", "code"},
		),
		CartGRPCRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "cart",
				Subsystem: "grpc",
				Name:      "request_duration_seconds",
				Help:      "gRPC request duration in seconds for cart transport.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		CartMongoRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "cart",
				Subsystem: "mongo",
				Name:      "requests_total",
				Help:      "Total number of MongoDB requests.",
			},
			[]string{"operation", "status"},
		),
		CartMongoRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "cart",
				Subsystem: "mongo",
				Name:      "request_duration_seconds",
				Help:      "MongoDB request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
	}

	prometheus.MustRegister(
		m.CartServiceRequestsTotal,
		m.CartServiceRequestDuration,
		m.CartGRPCRequestsTotal,
		m.CartGRPCRequestDuration,
		m.CartMongoRequestsTotal,
		m.CartMongoRequestDuration,
	)

	return m
}
