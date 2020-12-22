package nq_test

import (
	"net/http"
	_ "net/http/pprof"

	"log"
)

func init() {
	go func() {
		if err := http.ListenAndServe(":6060", nil); err != http.ErrServerClosed {
			log.Printf("Failed to start diagnostic HTTP server: %s\n", err)
		}
	}()
}
