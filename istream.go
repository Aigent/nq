package nq

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type istream struct {
	r   *bufio.Reader
	buf []byte
}

func newIStream() *istream {
	return &istream{}
}

func (is *istream) Work(ctx context.Context, s *sub, streamID uint32, conn net.Conn) error {
	m := newIStreamMetrics(conn.RemoteAddr(), s.metrics)
	var scratchpad [4]byte // uint32
	binary.LittleEndian.PutUint32(scratchpad[:], streamID)
	if is.r == nil {
		is.r = bufio.NewReader(conn)
	} else {
		is.r.Reset(conn)
	}

	// read loop
	for {
		if s.KeepaliveTimeout > 0 {
			if err := conn.SetReadDeadline(time.Now().Add(s.KeepaliveTimeout)); err != nil {
				return err
			}
		}

		size, err := decodeSize(is.r)
		if err != nil {
			return err
		}

		if len(is.buf) < size {
			is.buf = make([]byte, size)
		}
		if _, err = io.ReadFull(is.r, is.buf[:size]); err != nil {
			return err
		}
		// synchronized write to the shared ring buffer
		s.enqueue(scratchpad[:], is.buf[:size])
		// update metrics
		m.receivedMsgs.Inc()
		m.receivedBytes.Add(float64(size))

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			continue
		}
	}
}

type istreamMetrics struct {
	receivedBytes prometheus.Counter
	receivedMsgs  prometheus.Counter
}

func newIStreamMetrics(addr net.Addr, m *Metrics) *istreamMetrics {
	var host string
	// strip remote port to reduce the amount of labels
	if h, _, err := net.SplitHostPort(addr.String()); err == nil {
		host = h
	} else {
		host = addr.String()
	}
	l := prometheus.Labels{lAddr: host}
	return &istreamMetrics{
		receivedBytes: m.receivedBytes.With(l),
		receivedMsgs:  m.receivedMsgs.With(l),
	}
}
