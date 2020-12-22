package nq_test

import (
	"context"
	"log"
	"time"

	"github.com/aigent/nq"
)

func ExamplePub() {
	opts := nq.PubOpts{
		KeepaliveTimeout: 5 * time.Second,
		ConnectTimeout:   3 * time.Second,
		WriteTimeout:     3 * time.Second,
		FlushFrequency:   100 * time.Millisecond,
		NoDelay:          true,
		Printf:           log.Printf,
	}
	pub := nq.NewPub("tcp4://localhost:1234", opts, nq.NewDefaultMetrics())
	for {
		// Publish the message using 100 connections
		for i := 1; i <= 100; i++ {
			_ = pub.Publish(context.TODO(), []byte("Hello nanoQ"), i)
		}
	}
}

func ExampleSub() {
	opts := nq.SubOpts{
		KeepaliveTimeout: 5 * time.Second,
		Printf:           log.Printf,
	}
	sub := nq.NewSub("tcp4://:1234", opts, nq.NewDefaultMetrics())

	go func() {
		buf := make([]byte, maxPayload)
		for {
			if msg, stream, err := sub.Receive(context.TODO(), buf); err != nil {
				log.Println("Error while receiving:", err)
				continue
			} else {
				log.Printf("message from stream '%v' is: %s\n", stream, msg)
			}
		}
	}()
	if err := sub.Listen(context.TODO()); err != nil {
		log.Println("Listen error:", err)
	}
}
