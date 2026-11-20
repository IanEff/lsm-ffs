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
	mu        sync.RWMutex
	memtables struct {
		// pointer to the mutable memtable, which is also on our queue!
		mutable *memtable.Memtable
		queue   []*memtable.Memtable
	}
	wal *wal.WAL
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

	store := &Store{wal: w}
	store.memtables.mutable = mem
	store.memtables.queue = []*memtable.Memtable{mem}
	return store, nil
}

// Get returns a copy of a key's value, or nil and false if the key isn't present.
// An empty or nil stored value is present: use the boolean to distinguish it from a missing key.
func (db *Store) Get(key []byte) ([]byte, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for i := len(db.memtables.queue) - 1; i >= 0; i-- {
		if e, ok := db.memtables.queue[i].Get(key); ok {
			return entryValue(e)
		}
	}

	return nil, false // TODO: check segments, newest first
}

// entryValue turns the newest entry found for a key into Get's result. A tombstone means deleted: stop
// looking, even if an older table has the key.
func entryValue(e memtable.Entry) ([]byte, bool) {
	if e.Tombstone {
		return nil, false
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
	if !db.memtables.mutable.HasRoomForWrite(key, value) {
		db.rotateMemtables()
	}
	db.memtables.mutable.Put(key, memtable.Entry{Value: value})
	return nil
}

func (db *Store) rotateMemtables() *memtable.Memtable {
	db.memtables.mutable = memtable.New(memtableBytes)
	db.memtables.queue = append(db.memtables.queue, db.memtables.mutable)
	return db.memtables.mutable
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
