// Package lsm implements a concurrency-safe string key-value store as a log-structured merge tree.
package lsm

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Store is a concurrency-safe string key-value store. It is accessed through Get(), and Set(). Set() will
// clobber existing stored values and allows for empty string values.
// The zero value is not usable: build with [Open] or [New].
type Store struct {
	mu       sync.RWMutex
	memtable *Memtable
	segments []*Memtable
	wal      *wal
}

// New returns the pointer to a freshly initialized DB.
func NewStore() *Store {
	return &Store{
		memtable: NewMemtable(16),
	}
}

// Get returns a key's value from the DB. If the key isn't present, it returns an empty string and false.
func (db *Store) Get(key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// TODO: better implement this!
	return "", true
}

// Set sets the key-value pair in the DB. If the key is present, its existing value is clobbered.
func (db *Store) Set(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	err := db.wal.append(record{Op: opSet, Key: key, Value: value})
	if err != nil {
		return err
	}

	if !db.memtable.full() {
		db.memtable.put(key, entry{value: value})
	} else {
		// TODO: better do something!
	}
	return nil
}

func (db *Store) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.wal.Close()
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "wal.log")

	records, err := replay(path)
	if err != nil {
		return nil, err
	}

	mem := NewMemtable(defaultMemtableBytes)
	for _, record := range records {
		err := mem.apply(record)
		if err != nil {
			return nil, err
		}
	}

	w, err := openWAL(path)
	if err != nil {
		return nil, err
	}

	return &Store{memtable: mem, wal: w}, nil
}

func replay(path string) ([]record, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil // fresh file
		}
		return nil, err
	}
	defer f.Close()

	var records []record
	dec := json.NewDecoder(f)
	for {
		var rec record
		err := dec.Decode(&rec)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return records, nil
			}
			return nil, err
		}
		records = append(records, rec)
	}
}
