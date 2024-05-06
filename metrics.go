package locache

import (
	"fmt"
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
	AddHits(method string, count int)
	AddErrors(method string, count int)
	AddMisses(method string, count int)
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
		Name: fmt.Sprintf("%s_requests_total", prefix),
		Help: "Cache request counter",
	}, []string{"method", "status"})

	requestsTimeHist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    fmt.Sprintf("%s_requests_time_ms", prefix),
		Help:    "Cache request timings",
		Buckets: prometheus.DefBuckets,
	}, []string{})

	itemsInCacheTotal := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: fmt.Sprintf("%s_items_total", prefix),
		Help: "Cache request counter",
	})

	return &DefaultMetrics{
		requestsCounter:   requestsCounter,
		requestsTimeHist:  requestsTimeHist,
		itemsInCacheTotal: itemsInCacheTotal,
	}
}

func (m *DefaultMetrics) MustRegister() {
	prometheus.MustRegister(m.requestsCounter, m.requestsTimeHist)
}

func (m *DefaultMetrics) AddHits(method string, count int) {
	m.requestsCounter.With(prometheus.Labels{
		"method": method,
		"status": "hits",
	}).Add(float64(count))
}

func (m *DefaultMetrics) AddMisses(method string, count int) {
	m.requestsCounter.With(prometheus.Labels{
		"method": method,
		"status": "misses",
	}).Add(float64(count))
}

func (m *DefaultMetrics) AddErrors(method string, count int) {
	m.requestsCounter.With(prometheus.Labels{
		"method": method,
		"status": "error",
	}).Add(float64(count))
}

func (m *DefaultMetrics) ObserveRequest(method string, timeStart time.Time) {
	m.requestsTimeHist.With(prometheus.Labels{"method": method}).Observe(float64(now().Sub(timeStart)))
}

func (m *DefaultMetrics) SetItemsCount(count int) {
	m.itemsInCacheTotal.Set(float64(count))
}

func NewNopMetrics() *NopMetrics {
	return &NopMetrics{}
}

type NopMetrics struct{}

func (n *NopMetrics) AddHits(_ string, _ int)              {}
func (n *NopMetrics) AddMisses(_ string, _ int)            {}
func (n *NopMetrics) AddErrors(_ string, _ int)            {}
func (n *NopMetrics) ObserveRequest(_ string, _ time.Time) {}
func (n *NopMetrics) SetItemsCount(_ int)                  {}
