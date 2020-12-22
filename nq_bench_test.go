package nq_test

import (
	"testing"
	"time"
)

func Benchmark1Stream1s(b *testing.B) {
	transferMany(b, 1, time.Second)
}

func Benchmark1Stream3s(b *testing.B) {
	transferMany(b, 1, 3*time.Second)
}

func Benchmark10Streams3s(b *testing.B) {
	transferMany(b, 10, 3*time.Second)
}

func Benchmark50Streams30s(b *testing.B) {
	transferMany(b, 50, 30*time.Second)
}

func Benchmark100Streams1m(b *testing.B) {
	transferMany(b, 100, time.Minute)
}
