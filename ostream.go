package nq

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"syscall"
	"time"
)

// ostream is an output Pub's stream with asynchronous send loop
type ostream struct {
	*PubOpts
	addr net.Addr
	mu   sync.Mutex

	e encoder

	frontBuf *ringBuf // used by callers
	backBuf  *ringBuf // used by async send loop

	// metrics
	*pubMetrics
	bufferedMsgs  float64
	bufferedBytes float64
}

// newOStream constructor
func newOStream(addr net.Addr, opts *PubOpts, p *pubMetrics) *ostream {
	os := &ostream{
		PubOpts:    opts,
		addr:       addr,
		frontBuf:   &ringBuf{},
		backBuf:    &ringBuf{},
		pubMetrics: p,
	}

	return os
}

// Post a message into the outgoing queue.
func (os *ostream) Post(b []byte) error {
	os.mu.Lock()
	defer os.mu.Unlock()

	if err := os.e.EncodeTo(os.frontBuf, b); err != nil {
		out(os.Printf, "failed to write: %s", err)
		os.lostMsgs.Inc()
		os.lostBytes.Add(float64(len(b)))
		return err
	}
	os.bufferedMsgs++
	os.bufferedBytes += float64(len(b))

	// Do not let send buffer to grow indefinitely
	for os.frontBuf.Len() > MaxBuffer {
		size, err := decodeSize(os.frontBuf)
		if err != nil {
			panic("nanoq: failed to decode payload size from the send buffer: " + err.Error())
		}
		if err := os.frontBuf.Discard(size); err != nil {
			panic("nanoq: failed to discard bytes from the send buffer: " + err.Error())
		}
		os.bufferedMsgs--
		os.bufferedBytes -= float64(size)
	}

	return nil
}

// Work runs the background send loop.
// It blocks execution unil either ctx is done, no messages after KeepaliveTimeout published, or an unrecoverable error occurs.
func (os *ostream) Work(ctx context.Context) error {
	var (
		conn      net.Conn
		err       error
		gotMsgAt  = time.Now()
		flushTick = time.NewTicker(os.FlushFrequency)
	)

	defer func() {
		closeConn(os.Printf, conn, "to")
		flushTick.Stop()
	}()

	// (re-)connect loop
	for {
		if conn, err = os.dial(ctx); err != nil {
			return fmt.Errorf("nanoq: failed to dial %q: %w", os.addr.String(), err)
		}
		out(os.Printf, "connected to %s", conn.RemoteAddr().String())
	RETRY:
		// send loop
		for {
			select {
			case <-flushTick.C:
				var (
					bufferedBytes float64
					bufferedMsgs  float64
				)

				// flip front and back buffers
				os.backBuf.Reset()
				os.mu.Lock()
				os.frontBuf, os.backBuf = os.backBuf, os.frontBuf
				bufferedBytes = os.bufferedBytes
				os.bufferedBytes = 0
				bufferedMsgs = os.bufferedMsgs
				os.bufferedMsgs = 0
				os.mu.Unlock()

				if os.backBuf.Len() == 0 {
					// no new messages
					if time.Since(gotMsgAt) > os.KeepaliveTimeout {
						return nil
					}
					continue
				}

				gotMsgAt = time.Now()

				// flush write buffer
				if err := func() error {
					if os.WriteTimeout > 0 {
						if err := conn.SetWriteDeadline(time.Now().Add(os.WriteTimeout)); err != nil {
							return err
						}
					}
					_, err := os.backBuf.WriteTo(conn)
					return err
				}(); err != nil {
					out(os.Printf, "failed to send: %s", err)
					os.lostMsgs.Add(bufferedMsgs)
					os.lostBytes.Add(bufferedBytes)
					if temporary(err) {
						closeConn(os.Printf, conn, "to")
						break RETRY
					}
					return err
				}
				os.sentMsgs.Add(bufferedMsgs)
				os.sentBytes.Add(bufferedBytes)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		os.numReconnects.Inc()
	}
}

func (os *ostream) dial(ctx context.Context) (conn net.Conn, err error) {
	var (
		start = time.Now()
		delay time.Duration
	)
	for elapsed := time.Duration(0); elapsed < os.ConnectTimeout; elapsed = time.Since(start) {
		netDialer := net.Dialer{Timeout: os.ConnectTimeout - elapsed}

		if conn, err = netDialer.DialContext(ctx, os.addr.Network(), os.addr.String()); err != nil {
			if temporary(err) {
				if delay = increaseDelay(delay, os.ConnectTimeout-elapsed); delay > 0 {
					// sleep before retry
					select {
					case <-time.After(delay):
						continue
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
			}
			return nil, err
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			if err := tcp.SetNoDelay(os.NoDelay); err != nil {
				return nil, err
			}
		}
		return conn, nil
	}
	return nil, err
}

func (os *ostream) Reset() {
	os.mu.Lock()
	defer os.mu.Unlock()
	os.frontBuf.Reset()
	os.backBuf.Reset()
	os.bufferedMsgs = 0
	os.bufferedBytes = 0
}

// increaseDelay to not spam reconnects
func increaseDelay(d time.Duration, max time.Duration) time.Duration {
	const (
		min      = time.Millisecond
		minTimes = 2
		maxTimes = 10
	)
	if d < min {
		return min
	}
	// randomize delay to prevent accidental DDoS
	d *= time.Duration((minTimes + rand.Intn(maxTimes-minTimes)))
	if d > max {
		d = max
	}
	return d
}

// temporary errors can be retried
func temporary(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) {
		// we assume connection was refused because the subscriber is restarting or temporarily unavailable
		return true
	}
	if err, ok := err.(interface{ Temporary() bool }); ok {
		return err.Temporary()
	}
	return false
}
