// Stringabase is a barebones concurrency-safe string key-value store served over HTTP on localhost:4000.
// It's unfussy about values, it clobbers, all the defaults are baked in, and it's strictly in-memory.
//
// It runs as an HTTP server and presents the Stringabase over a simple HTTP API documented in
// [server.NewMuxer].
//
//	go run ./cmd/lsm-ffs
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ianeff/lsm-ffs/internal/server"
	"github.com/ianeff/lsm-ffs/internal/stringabase"
)

func main() {
	addr := "localhost:4000"
	sb, err := stringabase.Open("testfile")
	if err != nil {
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = server.ListenAndServe(ctx, addr, sb)
	if err != nil {
		log.Fatal(err)
	}
}
