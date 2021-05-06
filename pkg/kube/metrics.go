package kube

import (
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/metrics"
)

// Backends to use Prometheus client to export metrics from kubernetes/client-go.
//
// There are 2 steps to setup exporting:
// 1. Register metrics in Prometheus client with RegisterKubernetesClientMetrics
// 2. Register backends in client-go.
//
// Backends are used to send metrics from client-go to a Prometheus client.
// Backends are implemented interfaces from https://github.com/kubernetes/client-go/blob/master/tools/metrics/metrics.go

// Extraction of methods from metric_storage.go to prevent cycle dependencies.
type MetricStorage interface {
	RegisterCounter(metric string, labels map[string]string) *prometheus.CounterVec
	CounterAdd(metric string, value float64, labels map[string]string)
	RegisterHistogram(metric string, labels map[string]string) *prometheus.HistogramVec
	RegisterHistogramWithBuckets(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec
	HistogramObserve(metric string, value float64, labels map[string]string)
}

// RegisterKubernetesClientMetrics defines metrics in Prometheus client.
func RegisterKubernetesClientMetrics(metricStorage MetricStorage, metricLabels map[string]string) {
	labels := map[string]string{}
	for k := range metricLabels {
		labels[k] = ""
	}
	labels["verb"] = ""
	labels["url"] = ""

	metricStorage.RegisterHistogramWithBuckets("{PREFIX}kubernetes_client_request_latency_seconds",
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, 20, 50, // 10,20,50 seconds
		})

	// TODO update client-go to v.0.18.*
	//metricStorage.RegisterHistogramWithBuckets("{PREFIX}kubernetes_client_rate_limiter_latency_seconds",
	//	map[string]string{
	//		"verb": "",
	//		"url":  "",
	//	},
	//	[]float64{
	//		0.0,
	//		0.001, 0.002, 0.005, // 1,2,5 milliseconds
	//		0.01, 0.02, 0.05, // 10,20,50 milliseconds
	//		0.1, 0.2, 0.5, // 100,200,500 milliseconds
	//		1, 2, 5, // 1,2,5 seconds
	//		10, // 10 seconds
	//	})

	labels = map[string]string{}
	for k := range metricLabels {
		labels[k] = ""
	}
	labels["code"] = ""
	labels["method"] = ""
	labels["host"] = ""

	metricStorage.RegisterCounter("{PREFIX}kubernetes_client_request_result_total", labels)
}

func NewRequestLatencyMetric(metricStorage MetricStorage, labels map[string]string) metrics.LatencyMetric {
	return ClientRequestLatencyMetric{metricStorage, labels}
}

type ClientRequestLatencyMetric struct {
	metricStorage MetricStorage
	labels        map[string]string
}

func (c ClientRequestLatencyMetric) Observe(verb string, u url.URL, latency time.Duration) {
	labels := map[string]string{}
	for k, v := range c.labels {
		labels[k] = v
	}
	labels["verb"] = verb
	labels["url"] = u.String()

	c.metricStorage.HistogramObserve(
		"{PREFIX}kubernetes_client_request_latency_seconds",
		latency.Seconds(),
		labels,
	)
}

// RateLimiterLAtenct metric for versions v0.18.*
//func NewRateLimiterLatencyMetric(metricStorage *metric_storage.MetricStorage) metrics.LatencyMetric {
//	return ClientRateLimiterLatencyMetric{metricStorage}
//}
//
//type ClientRateLimiterLatencyMetric struct {
//	metricStorage *metric_storage.MetricStorage
//}
//
//func (c ClientRateLimiterLatencyMetric) Observe(verb string, u url.URL, latency time.Duration) {
//	c.metricStorage.HistogramObserve(
//		"{PREFIX}kubernetes_client_rate_limiter_latency_seconds",
//		latency.Seconds(),
//		map[string]string{
//			"verb": verb,
//			"url":  u.String(),
//		})
//}

func NewRequestResultMetric(metricStorage MetricStorage, labels map[string]string) metrics.ResultMetric {
	return ClientRequestResultMetric{metricStorage, labels}
}

type ClientRequestResultMetric struct {
	metricStorage MetricStorage
	labels        map[string]string
}

func (c ClientRequestResultMetric) Increment(code string, method string, host string) {
	labels := map[string]string{}
	for k, v := range c.labels {
		labels[k] = v
	}
	labels["code"] = code
	labels["method"] = method
	labels["host"] = host

	c.metricStorage.CounterAdd("{PREFIX}kubernetes_client_request_result_total",
		1.0,
		labels,
	)
}
