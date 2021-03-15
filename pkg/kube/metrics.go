package kube

import (
	"github.com/flant/shell-operator/pkg/metric_storage"
	"k8s.io/client-go/tools/metrics"
	"net/url"
	"time"
)

/**
 Prometheus implementation for metrics in kubernetes/client-go:

// LatencyMetric observes client latency partitioned by verb and url.
type LatencyMetric interface {
	Observe(verb string, u url.URL, latency time.Duration)
}

// ResultMetric counts response codes partitioned by method and host.
type ResultMetric interface {
	Increment(code string, method string, host string)
}

*/

func RegisterKubernetesClientMetrics(metricStorage *metric_storage.MetricStorage) {
	metricStorage.RegisterHistogramWithBuckets("{PREFIX}kubernetes_client_request_latency_seconds",
		map[string]string{
			"verb": "",
			"url":  "",
		},
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, // 10 seconds
		})

	metricStorage.RegisterHistogramWithBuckets("{PREFIX}kubernetes_client_rate_limiter_latency_seconds",
		map[string]string{
			"verb": "",
			"url":  "",
		},
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, // 10 seconds
		})

	metricStorage.RegisterCounter("{PREFIX}kubernetes_client_request_result_total",
		map[string]string{
			"code":   "",
			"method": "",
			"host":   "",
		})
}

func NewRequestLatencyMetric(metricStorage *metric_storage.MetricStorage) metrics.LatencyMetric {
	return ClientRequestLatencyMetric{metricStorage}
}

type ClientRequestLatencyMetric struct {
	metricStorage *metric_storage.MetricStorage
}

func (c ClientRequestLatencyMetric) Observe(verb string, u url.URL, latency time.Duration) {
	c.metricStorage.HistogramObserve(
		"{PREFIX}kubernetes_client_request_latency_seconds",
		latency.Seconds(),
		map[string]string{
			"verb": verb,
			"url":  u.String(),
		})
}

// RateLimiterLatency metric for versions v0.18.*
func NewRateLimiterLatencyMetric(metricStorage *metric_storage.MetricStorage) metrics.LatencyMetric {
	return ClientRateLimiterLatencyMetric{metricStorage}
}

type ClientRateLimiterLatencyMetric struct {
	metricStorage *metric_storage.MetricStorage
}

func (c ClientRateLimiterLatencyMetric) Observe(verb string, u url.URL, latency time.Duration) {
	c.metricStorage.HistogramObserve(
		"{PREFIX}kubernetes_client_rate_limiter_latency_seconds",
		latency.Seconds(),
		map[string]string{
			"verb": verb,
			"url":  u.String(),
		})
}

func NewRequestResultMetric(metricStorage *metric_storage.MetricStorage) metrics.ResultMetric {
	return ClientRequestResultMetric{metricStorage}
}

type ClientRequestResultMetric struct {
	metricStorage *metric_storage.MetricStorage
}

func (c ClientRequestResultMetric) Increment(code string, method string, host string) {
	c.metricStorage.CounterAdd("{PREFIX}kubernetes_client_request_result_total",
		1.0,
		map[string]string{
			"code":   code,
			"method": method,
			"host":   host,
		})
}
