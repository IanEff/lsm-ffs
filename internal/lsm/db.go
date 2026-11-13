// Package lsm implements a concurrency-safe string key-value store as a log-structured merge tree.
package lsm

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
)

// Store is a concurrency-safe string key-value store. It is accessed through Get(), and Set(). Set() will
// clobber existing stored values and allows for empty string values.
// The zero value is not usable: build with [Open] or [New].
type Store struct {
	mu       sync.RWMutex
	memtable Memtable
	segments []Memtable
	wal      wal
}

// New returns the pointer to a freshly initialized DB.
func NewStore() *Store {
	return &Store{
		memtable: *NewMemtable(16),
	}
}

// Get returns a key's value from the DB. If the key isn't present, it returns an empty string and false.
func (db *Store) Get(key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	value, ok := db.memtable[key]
	return value, ok
}

// Set sets the key-value pair in the DB. If the key is present, its existing value is clobbered.
func (db *Store) Set(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	err := db.wal.append(record{Op: opSet, Key: key, Value: value})
	if err != nil {
		return err
	}
	db.memtable[key] = value // clobber existing
	return nil
}

func Open(path string) (*Store, error) {
	db := New()
	db.wal.path = path
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return db, nil
		}
		return &Store{}, err
	}
	dec := json.NewDecoder(f)
	var rec record
	for {
		err := dec.Decode(&rec)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return &Store{}, nil
		}
		db.memtable[rec.Key] = rec.Value
	}
	return db, nil
}
