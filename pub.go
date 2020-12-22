package nq

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MaxBuffer value limits internal buffers size to prevent memory blow up
// when there is no connection or network is too slow
var MaxBuffer = MaxPayload * 10

// PubOpts are publisher's options to tweak
type PubOpts struct {
	KeepaliveTimeout time.Duration // For how long to keep a stream alive if there are no new messages (default timeout is used on zero value)
	ConnectTimeout   time.Duration // Limit (re)-connects time (default timeout is used on zero value)
	WriteTimeout     time.Duration // Limit socket write operation time (zero means no deadline)
	FlushFrequency   time.Duration // Socket write frequency (default frequency is used on zero value)
	NoDelay          bool          // Disable or enable Nagle's algorithm (TCP only)

	// Printf optionally specifies a function used for logging
	Printf func(format string, v ...interface{})
}

// Pub is an asynchronous message publisher.
//
// Publish stores the payload into memory buffer and returns immediately. There is only best effort delivery guarantee,
// nil error only means that message was enqueued for sending.
// The streamKey ties the payload with a certain stream (connection). Each time Publish receives new streamKey it spawns new connection.
// Messages published using the same streamKey will be transferred over the same stream in the same order.
// Note that streamKey exists only on the publisher's side.
// Subscriber provides a key to differentiate streams but it doesn't match with the one passed to the Pub.
type Pub interface {
	Publish(ctx context.Context, payload []byte, streamKey interface{}) error
}

type pub struct {
	PubOpts
	pubMetrics
	addr        net.Addr
	streamByKey sync.Map
	pool        pool
}

const (
	defaultTimeout        = 5 * time.Second
	defaultFlushFrequency = 50 * time.Millisecond
)

// NewPub constructs a single destination Pub
func NewPub(url string, opts PubOpts, m *Metrics) Pub {
	addr := MustParseURL(url)
	l := prometheus.Labels{lAddr: addr.String()}

	if opts.KeepaliveTimeout == 0 {
		opts.KeepaliveTimeout = defaultTimeout
	}
	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = defaultTimeout
	}
	if opts.FlushFrequency == 0 {
		opts.FlushFrequency = defaultFlushFrequency
	}

	pm := pubMetrics{
		sentBytes:         m.sentBytes.With(l),
		sentMsgs:          m.sentMsgs.With(l),
		lostBytes:         m.lostBytes.With(l),
		lostMsgs:          m.lostMsgs.With(l),
		numReconnects:     m.numReconnects.With(l),
		numStreams:        m.numStreams.With(l),
		streamDurationSec: m.streamDurationSec.With(l),
	}

	return &pub{
		PubOpts:     opts,
		pubMetrics:  pm,
		addr:        addr,
		streamByKey: sync.Map{},
		pool:        pool{New: func() interface{} { return newOStream(addr, &opts, &pm) }},
	}
}

func (p *pub) Publish(ctx context.Context, payload []byte, streamKey interface{}) error {
	// pick connection by streamKey or create new one
	var s *ostream

	// before calling LoadOrStore we want to optimize for the case when ostream exists
	if val, ok := p.streamByKey.Load(streamKey); ok {
		s = val.(*ostream)
	} else if val, loaded := p.streamByKey.LoadOrStore(streamKey, p.pool.Get()); loaded {
		// someone has just created ostream
		s = val.(*ostream)
	} else {
		// create new ostream
		s = val.(*ostream)
		p.numStreams.Inc()
		go func() {
			start := time.Now()
			if err := s.Work(ctx); err != nil {
				out(p.Printf, err.Error())
			}
			p.streamByKey.Delete(streamKey)
			p.streamDurationSec.Observe(time.Since(start).Seconds())
			p.numStreams.Dec()
			// reuse ostream
			s.Reset()
			p.pool.Put(s)
		}()
	}

	// post to send queue
	return s.Post(payload)
}

type pubMetrics struct {
	sentBytes         prometheus.Counter
	sentMsgs          prometheus.Counter
	lostBytes         prometheus.Counter
	lostMsgs          prometheus.Counter
	numReconnects     prometheus.Counter
	numStreams        prometheus.Gauge
	streamDurationSec prometheus.Observer
}
