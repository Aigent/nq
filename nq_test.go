package nq_test

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aigent/nq"
)

var pubOpts = nq.PubOpts{
	//Printf:           log.Printf,
}

var subOpts = nq.SubOpts{
	//Printf:           log.Printf,
}

var metrics = nq.NewDefaultMetrics()

func newPubSub(ctx context.Context, t testing.TB, urls []string) (nq.Pub, []nq.Sub, *sync.WaitGroup) {
	t.Helper()
	if len(urls) == 0 {
		panic("empty URL list")
	}

	// subscriber(s)
	var subs []nq.Sub
	wg := &sync.WaitGroup{}
	for _, url := range urls {
		sub := nq.NewSub(url, subOpts, metrics)
		wg.Add(1)
		go func(url string, sub nq.Sub) {
			if err := sub.Listen(ctx); err != context.Canceled {
				t.Errorf("Listen %q: %s", url, err)
			}
			wg.Done()
		}(url, sub)
		subs = append(subs, sub)
	}

	// publisher
	var pub nq.Pub
	if len(urls) == 1 {
		pub = nq.NewPub(urls[0], pubOpts, metrics)
	} else {
		pub = nq.NewMultiPub(urls, pubOpts, metrics)
	}

	return pub, subs, wg
}

func TestOneMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pub, subs, wg := newPubSub(ctx, t, []string{"tcp4://localhost:1441"})

	defer func() {
		cancel()
		wg.Wait()
	}()

	want := []byte("Hello, world")

	// publisher
	if err := pub.Publish(ctx, want, 0); err != nil {
		t.Error("Failed to Publish:", err)
		return
	}

	buf := make([]byte, maxPayload)

	have, _, err := subs[0].Receive(ctx, buf)
	if err != nil {
		if err != context.Canceled {
			t.Error(err)
		}
		return
	}
	if !bytes.Equal(want, have) {
		t.Errorf("%q != %q", string(have), string(want))
		return
	}
}

func TestMultiPub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	urls := []string{"tcp4://localhost:1443", "tcp4://localhost:1444"}
	pub, subs, wg := newPubSub(ctx, t, urls)

	defer func() {
		cancel()
		wg.Wait()
	}()

	msg := []byte("Hello, world")

	// publisher
	if err := pub.Publish(ctx, msg, 0); err != nil {
		t.Errorf("error while publishing %v", err)
		return
	}

	buf := make([]byte, maxPayload)
	for i := range urls {
		payload, _, err := subs[i].Receive(ctx, buf)
		if err != nil {
			if err != context.Canceled {
				t.Error(err)
			}
			return
		}
		if !bytes.Equal(payload, msg) {
			t.Fail()
		}
	}
}

func TestManyMessages(t *testing.T) {
	transferMany(t, 1, 3*time.Second)
}

// transferMany parallel streams. Each stream publishes messages during the `span` perion.
func transferMany(t testing.TB, streams int, span time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	pub, subs, wg := newPubSub(ctx, t, []string{"tcp4://localhost:1442"})

	go func() {
		time.Sleep(span)
		cancel()
	}()
	defer wg.Wait()

	if b, ok := t.(*testing.B); ok {
		b.ResetTimer()
	}

	var checkMsgIntegrity bool
	if _, ok := t.(*testing.T); ok {
		checkMsgIntegrity = true
	}

	// publisher(s)
	start := time.Now()
	go func() {
		for time.Since(start) < span {
			for s := 0; s < streams; s++ {
				m := testMsgPool.Get().(*testMsg)
				m.SetTimestamp()
				if err := pub.Publish(ctx, m.Raw(), s); err != nil {
					_ = err
				}
				testMsgPool.Put(m)
			}
		}
	}()

	var (
		numRcv    int
		totalSize int64

		maxLatency, sumLatency time.Duration
		minLatency             = 99999 * time.Hour
		numMeasurements        int
		buf                    = make([]byte, maxPayload*2)
	)

	for {
		payload, _, err := subs[0].Receive(ctx, buf)
		if err != nil {
			if err == io.EOF {
				continue
			}
			if err != context.Canceled {
				t.Errorf("Receive: %s", err)
			}
			break
		}
		var m testMsg = testMsg(payload)

		latency := m.Age()

		if latency > maxLatency {
			maxLatency = latency
		}
		if latency < minLatency {
			minLatency = latency
		}
		sumLatency += latency
		numMeasurements++

		if checkMsgIntegrity && !m.Ok() {
			t.Error("Invalid message received")
			break
		}
		totalSize += int64(len(m.Raw()))
		numRcv++
	}

	if b, ok := t.(*testing.B); ok {
		b.StopTimer()
		b.SetBytes(totalSize)
		b.ReportMetric(float64(minLatency.Microseconds())/1000.0, "min_ms")
		b.ReportMetric(float64(maxLatency.Microseconds())/1000.0, "max_ms")
		if numMeasurements == 0 {
			numMeasurements = 1
		}
		b.ReportMetric(float64(sumLatency.Microseconds())/float64(numMeasurements)/1000, "avg_ms")
		b.ReportMetric(float64(totalSize)/1024/1024/time.Since(start).Seconds(), "MB/s")
	}

}
