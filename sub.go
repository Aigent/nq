package nq

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// StreamDescriptor is used to separate messages from the different incoming streams
type StreamDescriptor uint32

// Sub is a subscriber.
//
// Listen should be called for Sub to bind port and to start listen for messages.
// It will block until the ctx will signal Done, and will return ctx.Err().
//
// Receive reads the next available message into the byte slice p.
// The payload returned by Receive alias the same memory region as p.
// If the buffer p is too short to hold the message Receive returns io.ErrShortBuffer and discards the message.
// Receive will block until either new message is available for readind or ctx signals Done.
// The stream descriptor returned by Receive is not the same as streamKey in Publish, it is an arbitrary key to distinguish
// messages sent through different streams (connections).
type Sub interface {
	Listen(ctx context.Context) error
	Receive(ctx context.Context, p []byte) (payload []byte, stream StreamDescriptor, err error)
}

// SubOpts to tweak
type SubOpts struct {
	KeepaliveTimeout time.Duration                         // how long to keep a stream alive if there are no new messages
	Printf           func(format string, v ...interface{}) // Printf optionally specifies a function used for logging
}

type sub struct {
	SubOpts
	addr              net.Addr
	queue             *ringBuf // in-memory buffer to accumulate incoming messages
	mu                sync.Mutex
	hasData           *sync.Cond
	e                 encoder
	pool              pool
	scratchpad        [4]byte // used to prepend messages with the uint32 streamID
	metrics           *Metrics
	numStreams        prometheus.Gauge
	streamDurationSec prometheus.Observer
	lostBytes         prometheus.Counter
	lostMsgs          prometheus.Counter
}

// NewSub constructs a Sub
func NewSub(url string, opts SubOpts, metr *Metrics) Sub {
	addr := MustParseURL(url)
	l := prometheus.Labels{lAddr: addr.String()}

	s := &sub{
		SubOpts: opts,
		addr:    addr,
		queue:   &ringBuf{},
		pool:    pool{New: func() interface{} { return newIStream() }},
		// metrics
		metrics:           metr,
		numStreams:        metr.numStreams.With(l),
		streamDurationSec: metr.streamDurationSec.With(l),
		lostBytes:         metr.lostBytes.With(l),
		lostMsgs:          metr.lostMsgs.With(l),
	}
	s.hasData = sync.NewCond(&s.mu)

	return s
}

func (s *sub) enqueue(streamID []byte, payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n, err := s.queue.Write(streamID); err != nil || n != len(streamID) {
		panic("nanoq: failed to write streamID into the internal queue")
	}
	if err := s.e.EncodeTo(s.queue, payload); err != nil {
		panic("nanoq: failed to write payload into the internal queue")
	}
	s.hasData.Signal()
}

func (s *sub) dequeue(p []byte) ([]byte, StreamDescriptor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.queue.Len() == 0 {
		return nil, StreamDescriptor(0), nil
	}

	// decode stream descriptor
	if _, err := io.ReadFull(s.queue, s.scratchpad[:]); err != nil {
		panic("nanoq: failed to read streamID from the internal queue")
	}
	stream := StreamDescriptor(binary.LittleEndian.Uint32(s.scratchpad[:]))
	// decode payload size
	size, err := decodeSize(s.queue)
	if err != nil {
		panic("nanoq: failed to decode payload size from the internal queue: " + err.Error())
	}
	// decode payload and store data to byte slice p
	if cap(p) < size {
		if err := s.queue.Discard(size); err != nil {
			panic("nanoq: failed to discard payload from the internal queue")
		}
		s.lostMsgs.Inc()
		s.lostBytes.Add(float64(size))
		return nil, StreamDescriptor(0), io.ErrShortBuffer
	}
	p = p[:cap(p)] // reslice for max capacity

	n, err := io.ReadFull(s.queue, p[:size])
	if err != nil {
		panic("nanoq: failed to read payload from the internal queue: " + err.Error())
	}
	if n != size {
		panic("nanoq: invalid read from the internal queue")
	}
	return p[:n], stream, nil
}

func (s *sub) Receive(ctx context.Context, p []byte) (payload []byte, stream StreamDescriptor, err error) {
	// wait in a loop for data
	for payload == nil {
		if payload, stream, err = s.dequeue(p); err != nil || payload != nil {
			return // either we get data or read error
		}

		// check for cancelation
		select {
		case <-ctx.Done():
			return nil, StreamDescriptor(0), ctx.Err()
		default:
		}

		// wait on Cond variable
		s.mu.Lock()
		s.hasData.Wait()
		s.mu.Unlock()
	}
	return
}

func (s *sub) Listen(ctx context.Context) error {
	var (
		wg         sync.WaitGroup
		listenConf net.ListenConfig
		ln         net.Listener
		err        error
	)

	defer wg.Wait()

	if ln, err = listenConf.Listen(ctx, s.addr.Network(), s.addr.String()); err != nil {
		return fmt.Errorf("nanoq: failed to listen: %w", err)
	}

	out(s.Printf, "listen %s", ln.Addr().String())

	// monitor shutdown request to force close the listener
	var shutdown int32
	const graceful int32 = 1
	go func() {
		<-ctx.Done()
		atomic.StoreInt32(&shutdown, graceful)
		ln.Close()
		// Notify receivers
		s.mu.Lock()
		s.hasData.Broadcast()
		s.mu.Unlock()
	}()

	var streamID uint32
	// accept connections
	for {
		conn, err := ln.Accept()
		if err != nil {
			if atomic.LoadInt32(&shutdown) == graceful {
				return ctx.Err()
			}
			return fmt.Errorf("nanoq: failed to accept: %w", err)
		}
		s.numStreams.Inc()
		out(s.Printf, "new connection from %s", conn.RemoteAddr())
		streamID++
		wg.Add(1)
		// connection
		go func(streamID uint32) {
			start := time.Now()
			// Reader buffer for conn
			is := s.pool.Get().(*istream)
			// receive loop
			if err := is.Work(ctx, s, streamID, conn); err != nil {
				out(s.Printf, "connection from %s: %s", conn.RemoteAddr(), err)
			}
			// tear down
			closeConn(s.Printf, conn, "from")
			s.pool.Put(is)
			s.numStreams.Dec()
			s.streamDurationSec.Observe(time.Since(start).Seconds())
			wg.Done()
		}(streamID)
	}
}
