package nq_test

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func TestMultiStreamsIDs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	urls := []string{"tcp4://localhost:1443"}
	pub, subs, wg := newPubSub(ctx, t, urls)

	const maxStreams = 100
	const maxTries = 10000000

	streamsIn := make(map[interface{}]int, maxStreams)
	streamsOut := make(map[interface{}]int, maxStreams)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < maxTries; i++ {
			streamID := strconv.Itoa(rand.Intn(maxStreams))
			inc := streamsIn[streamID] + 1
			if err := pub.Publish(ctx, []byte(strconv.Itoa(inc)), streamID); err != nil {
				t.Errorf("error while publishing %v", err)
			}
			streamsIn[streamID] = inc
		}
		time.Sleep(5 * time.Second)
		cancel()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, maxPayload)
		payload, streamID, err := subs[0].Receive(ctx, buf)
		if err != nil {
			return
		}
		currPlusOne := streamsOut[streamID] + 1
		want := strconv.Itoa(currPlusOne)
		got := string(payload)
		if want != got && want != "1" {
			t.Errorf("want=%v, got=%v", want, got)
		}
		streamsOut[streamID] = currPlusOne
	}()

	wg.Wait()
}
