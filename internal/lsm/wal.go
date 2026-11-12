package lsm

import (
	"encoding/json"
	"os"
)

const (
	opSet    = "SET"
	opDelete = "DELETE"
)

// record is one entry in the write-ahead log.
type record struct {
	Op, Key, Value string
}

// wal is the write-ahead log: an append-only file of JSON-encoded records, one per line.
type wal struct {
	path string
}

func (w *wal) append(rec record) error {
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(rec)
	if err != nil {
		return err
	}
	return nil
}
