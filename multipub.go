package nq

import "context"

type multiPub []Pub

// NewMultiPub constructs a Pub that publishes simultaneously to several destination Subs
func NewMultiPub(urls []string, opts PubOpts, metr *Metrics) Pub {
	var mp multiPub
	for _, u := range urls {
		mp = append(mp, NewPub(u, opts, metr))
	}
	return mp
}

func (mp multiPub) Publish(ctx context.Context, payload []byte, streamID interface{}) error {
	var err error
	for _, p := range mp {
		if err2 := p.Publish(ctx, payload, streamID); err2 != nil && err == nil {
			err = err2
		}
	}
	return err
}
