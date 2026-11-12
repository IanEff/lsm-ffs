// lsm-ffs is a barebones concurrency-safe string key-value store served over HTTP on localhost:4000.
// It's unfussy about values, it clobbers, and all the defaults are baked in.
//
// It runs as an HTTP server and presents an [lsm.Store] over a simple HTTP API documented in
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

	"github.com/ianeff/lsm-ffs/internal/lsm"
	"github.com/ianeff/lsm-ffs/internal/server"
)

func main() {
	addr := "localhost:4000"
	db, err := lsm.Open("testfile")
	if err != nil {
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = server.ListenAndServe(ctx, addr, db)
	if err != nil {
		log.Fatal(err)
	}
}
