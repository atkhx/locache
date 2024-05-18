package locache

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	MethodGet = "get"
	MethodSet = "set"
	MethodDel = "del"

	MethodGetOrRefresh = "get_or_refresh"
	MethodPurge        = "purge"
)

type Metrics interface {
	IncHits(method string)
	IncErrors(method string)
	IncMisses(method string)
	ObserveRequest(method string, timeStart time.Time)
	SetItemsCount(count int)
}

type DefaultMetrics struct {
	requestsCounter   *prometheus.CounterVec
	requestsTimeHist  *prometheus.HistogramVec
	itemsInCacheTotal prometheus.Gauge
}

func NewDefaultMetrics(prefix string) *DefaultMetrics {
	requestsCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prefix + "_requests_total",
		Help: "Cache request counter",
	}, []string{"method", "status"})

	requestsTimeHist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    prefix + "_requests_time_ms",
		Help:    "Cache request timings",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	itemsInCacheTotal := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: prefix + "_items_total",
		Help: "Cache request counter",
	})

	return &DefaultMetrics{
		requestsCounter:   requestsCounter,
		requestsTimeHist:  requestsTimeHist,
		itemsInCacheTotal: itemsInCacheTotal,
	}
}

func (m *DefaultMetrics) MustRegister() {
	prometheus.MustRegister(m.requestsCounter, m.requestsTimeHist, m.itemsInCacheTotal)
}

func (m *DefaultMetrics) IncHits(method string) {
	m.requestsCounter.With(prometheus.Labels{
		"method": method,
		"status": "hits",
	}).Inc()
}

func (m *DefaultMetrics) IncMisses(method string) {
	m.requestsCounter.With(prometheus.Labels{
		"method": method,
		"status": "misses",
	}).Inc()
}

func (m *DefaultMetrics) IncErrors(method string) {
	m.requestsCounter.With(prometheus.Labels{
		"method": method,
		"status": "error",
	}).Inc()
}

func (m *DefaultMetrics) ObserveRequest(method string, timeStart time.Time) {
	m.requestsTimeHist.With(prometheus.Labels{"method": method}).Observe(float64(now().Sub(timeStart).Milliseconds()))
}

func (m *DefaultMetrics) SetItemsCount(count int) {
	m.itemsInCacheTotal.Set(float64(count))
}

func NewNopMetrics() *NopMetrics {
	return &NopMetrics{}
}

type NopMetrics struct{}

func (n *NopMetrics) IncHits(_ string)                     {}
func (n *NopMetrics) IncMisses(_ string)                   {}
func (n *NopMetrics) IncErrors(_ string)                   {}
func (n *NopMetrics) ObserveRequest(_ string, _ time.Time) {}
func (n *NopMetrics) SetItemsCount(_ int)                  {}
