// Package lsm implements a concurrency-safe string key-value store as a log-structured merge tree.
package lsm

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/ianeff/lsm-ffs/internal/wal"
)

var errClosed = errors.New("lsm: store is closed")

// Store is a concurrency-safe string key-value store. It is accessed through Get(), and Set(). Set() will
// clobber existing stored values and allows for empty string values.
// The zero value is not usable: build with [Open].
type Store struct {
	mu       sync.RWMutex
	memtable *Memtable
	segments []*Memtable
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

	mem := NewMemtable(defaultMemtableBytes)
	for _, rec := range records {
		err := mem.apply(rec)
		if err != nil {
			return nil, err
		}
	}

	w, err := wal.Open(path)
	if err != nil {
		return nil, err
	}

	return &Store{memtable: mem, wal: w}, nil
}

// Get returns a key's value from the DB. If the key isn't present, it returns an empty string and false.
func (db *Store) Get(key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	e, ok := db.memtable.get(key)
	if !ok {
		return "", false // TODO: check segments, newest first
	}
	if e.tombstone {
		return "", false // deleted: stop looking, even if a segment has it
	}
	return e.value, true
}

// Set sets the key-value pair in the DB. If the key is present, its existing value is clobbered.
func (db *Store) Set(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.wal == nil {
		return errClosed
	}
	err := db.wal.Append(wal.Record{Op: wal.OpSet, Key: key, Value: value})
	if err != nil {
		return err
	}

	db.memtable.put(key, entry{value: value})
	if db.memtable.full() {
		// TODO: flush the memtable to a segment
	}
	return nil
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
