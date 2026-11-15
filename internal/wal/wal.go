// Package wal implements a write-ahead log: an append-only file of records, one per write, that a store
// replays on startup to rebuild what it held in memory.
package wal

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// The operations a [Record] can carry.
const (
	OpSet    = "SET"
	OpDelete = "DELETE"
)

// Record is one entry in the write-ahead log.
type Record struct {
	Op         string
	Key, Value []byte
}

// WAL is an open write-ahead log. Records are JSON-encoded, one per line,
// with byte keys and values encoded as base64 strings (nil slices as null).
// Build with [Open]; the zero value is not usable.
type WAL struct {
	f  *os.File
	bw *bufio.Writer
}

// Open opens the log at path for appending, creating it if it doesn't exist.
func Open(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &WAL{f: f, bw: bufio.NewWriter(f)}, nil
}

// Append writes rec to the log and syncs it to disk before returning.
func (w *WAL) Append(rec Record) error {
	err := json.NewEncoder(w.bw).Encode(rec)
	if err != nil {
		return err
	}
	err = w.bw.Flush()
	if err != nil {
		return err
	}
	return w.f.Sync()
}

// Close flushes, syncs and closes the log.
func (w *WAL) Close() error {
	return errors.Join(w.bw.Flush(), w.f.Sync(), w.f.Close())
}

// Replay reads every record in the log at path, oldest first. A missing file is a fresh log, so it returns
// no records and no error.
func Replay(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []Record
	dec := json.NewDecoder(f)
	for {
		var rec Record
		err := dec.Decode(&rec)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return records, nil
			}
			panic(err) // TODO: Panicking on torn page for now.
		}
		records = append(records, rec)
	}
}
