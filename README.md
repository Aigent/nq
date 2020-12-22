# nanoQ — high-performance brokerless Pub/Sub for streaming real-time data

nanoQ is a very minimalistic (opinionated/limited) Pub/Sub transport library.

Instant "fire and forget" publish with only best-effort delivery guarantee.

## Do I need it?

For telecommunications, media, IoT, gaming, metrics, clicks, etc.: ***it's okay to loose data*** to get the most up to date messages.

Brokerless means no central broker server; publishers connect directly to subscribers.

No open Internet. nanoQ is for the private backend infrastructure networks only. There is no integrity / loop detections and others safety mechanisms.

Bandwidth vs latency is configurable, but ultimately nanoQ prefers bandwidth — it is designed to carry hundreds and thousands parallel streams through the backend infrastructure.

## In a Nutshell

* tiny: under 1K LOC
* high-performance: no allocations on critical paths, granular locking, and other optimizations
* low overhead simple protocol: 1-5 bytes of metadata (varint length prefix) per message, depending on the message size
* battle-tested: running in production
* data oblivious: JSON, protobuf, thrift, gzip, or XML - all goes
* kubernetes and cloud-native: easy to integrate with existing load-balancers
* scalable: no coordinator or a central server
* simple URL address scheme: `tcp4://localhost:1234` or `unix:///var/run/sockety.sock`
* go native: context, modules and errors wrapping
* transparent: with Prometheus metrics and pluggable logging

## Quick start

### Subscriber

```go
import (
    "context"
    "log"
    "time"

    "github.com/aigent/nq"
)

func main() {
    opts := nq.SubOpts{
        KeepaliveTimeout: 5 * time.Second,
        Printf:           log.Printf,
    }
    sub := nq.NewSub("tcp4://:1234", opts, nq.NewDefaultMetrics())

    go func() {
        buf := make([]byte, 4096)
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
```

### Publisher

```go
import (
    "context"
    "log"
    "time"

    "github.com/aigent/nq"
)

func main() {
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
            if err := pub.Publish(context.TODO(), []byte("Hello nanoQ"), i); err != nil {
                log.Println("Error while publishing:", err)
            }
        }
    }
}
```
