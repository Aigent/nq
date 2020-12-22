package nq

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics provide insights to the current state of the app
type Metrics struct {
	numStreams        *prometheus.GaugeVec
	streamDurationSec *prometheus.HistogramVec
	receivedBytes     *prometheus.CounterVec
	receivedMsgs      *prometheus.CounterVec
	sentBytes         *prometheus.CounterVec
	sentMsgs          *prometheus.CounterVec
	lostBytes         *prometheus.CounterVec
	lostMsgs          *prometheus.CounterVec
	numReconnects     *prometheus.CounterVec
}

const (
	lAddr = "addr"
)

// NewMetrics constructs and registers Metrics
func NewMetrics(reg prometheus.Registerer) *Metrics {
	const ns = "nanoq"
	l := []string{lAddr}

	metrics := &Metrics{
		receivedBytes:     prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "received_bytes"}, l),
		receivedMsgs:      prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "received_msgs"}, l),
		sentBytes:         prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "sent_bytes"}, l),
		sentMsgs:          prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "sent_msgs"}, l),
		lostBytes:         prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "lost_bytes"}, l),
		lostMsgs:          prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "lost_msgs"}, l),
		numReconnects:     prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: ns, Name: "reconnects"}, l),
		numStreams:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Namespace: ns, Name: "streams"}, l),
		streamDurationSec: prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: ns, Name: "stream_duration_sec", Buckets: dayOfSecondsBuckets()}, l),
	}
	reg.MustRegister(metrics.receivedBytes)
	reg.MustRegister(metrics.receivedMsgs)
	reg.MustRegister(metrics.sentBytes)
	reg.MustRegister(metrics.sentMsgs)
	reg.MustRegister(metrics.lostBytes)
	reg.MustRegister(metrics.lostMsgs)
	reg.MustRegister(metrics.numReconnects)
	reg.MustRegister(metrics.numStreams)
	reg.MustRegister(metrics.streamDurationSec)
	return metrics
}

// NewDefaultMetrics constructs and registers Metrics with the prometheus.DefaultRegisterer
func NewDefaultMetrics() *Metrics {
	return NewMetrics(prometheus.DefaultRegisterer)
}

// dayOfSecondsBuckets for streamDurationSec histogram
func dayOfSecondsBuckets() []float64 {
	var (
		unit    float64 = 1
		buckets []float64
	)
	buckets = append(buckets, []float64{1 * unit, 2 * unit, 5 * unit, 10 * unit, 15 * unit, 30 * unit, 45 * unit}...) // sec
	unit *= 60
	buckets = append(buckets, []float64{1 * unit, 2 * unit, 5 * unit, 10 * unit, 15 * unit, 30 * unit, 45 * unit}...) // min
	unit *= 60
	buckets = append(buckets, []float64{1 * unit, 2 * unit, 5 * unit, 12 * unit, 18 * unit, 24 * unit}...) // hr
	return buckets
}
