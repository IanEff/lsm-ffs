// Package lsm implements a concurrency-safe byte key-value store as a log-structured merge tree.
package lsm

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/ianeff/lsm-ffs/internal/memtable"
	"github.com/ianeff/lsm-ffs/internal/wal"
)

const (
	memtableBytes = 16
)

var errClosed = errors.New("lsm: store is closed")

// Store is a concurrency-safe byte key-value store. Set replaces existing values
// and allows empty keys and values.
// The zero value is not usable: build with [Open].
type Store struct {
	mu       sync.RWMutex
	memtable *memtable.Memtable
	queue    []*memtable.Memtable
	wal      *wal.WAL
}

// Open opens the store in dir, creating dir if needed, and replays its write-ahead log.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "wal.log")

	records, err := wal.Replay(path)
	if err != nil {
		return nil, err
	}

	mem := memtable.New(memtableBytes)
	for _, rec := range records {
		if err := mem.Apply(rec); err != nil {
			return nil, err
		}
	}

	w, err := wal.Open(path)
	if err != nil {
		return nil, err
	}

	return &Store{memtable: mem, wal: w}, nil
}

// Get returns a copy of a key's value, or nil and false if the key isn't present.
// An empty or nil stored value is present: use the boolean to distinguish it from a missing key.
func (db *Store) Get(key []byte) ([]byte, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	e, ok := db.memtable.Get(key)
	if !ok {
		return nil, false // TODO: check segments, newest first
	}
	if e.Tombstone {
		return nil, false // deleted: stop looking, even if a segment has it
	}
	return bytes.Clone(e.Value), true
}

// Set stores a copy of key and value, replacing any existing value for key.
// Callers may reuse the slices after Set returns, but must not modify them during the call.
func (db *Store) Set(key, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.wal == nil {
		return errClosed
	}
	err := db.wal.Append(wal.Record{Op: wal.OpSet, Key: key, Value: value})
	if err != nil {
		return err
	}
	if !db.memtable.HasRoomForWrite(key, value) {
		db.rotateMemtables()
	}
	db.memtable.Put(key, memtable.Entry{Value: value})
	return nil
}

func (db *Store) rotateMemtables() *memtable.Memtable {
	db.queue = append(db.queue, db.memtable)
	db.memtable = memtable.New(memtableBytes)
	return db.memtable
}

// Close closes the store's write-ahead log. It's safe to call more than once; Set fails after the first.
func (db *Store) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.wal == nil {
		return nil
	}
	err := db.wal.Close()
	db.wal = nil
	return err
}
