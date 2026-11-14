package lsm

import (
	"bufio"
	"encoding/json"
	"errors"
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
	f   *os.File
	bw  *bufio.Writer
	buf []byte
}

func openWAL(path string) (*wal, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		f.Close()
		return nil, err
	}
	
	return &wal{f: f, bw: bufio.NewWriter(f)}, nil
}

func (w *wal) Close() error {
	return errors.Join(w.bw.Flush(), w.f.Sync(), w.f.Close())
}

func (w *wal) append(rec record) error {
	err := json.NewEncoder(w.bw).Encode(rec)
	if err != nil {
		return err
	}
	return nil
}
