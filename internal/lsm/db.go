// Package lsm implements a concurrency-safe byte key-value store as a log-structured merge tree.
package lsm

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/ianeff/lsm-ffs/internal/memtable"
	"github.com/ianeff/lsm-ffs/internal/sstable"
	"github.com/ianeff/lsm-ffs/internal/storage"
	"github.com/ianeff/lsm-ffs/internal/wal"
)

const (
	memtableBytes          = 16
	memtableFlushThreshold = 8 << 10 // 8 KiB
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
	wal         *wal.WAL
	dataStorage *storage.Provider
	sstables    []*storage.FileMetadata
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

	dataStorage, err := storage.NewProvider(path)
	if err != nil {
		return nil, err
	}

	store := &Store{wal: w}
	store.memtables.mutable = mem
	store.memtables.queue = []*memtable.Memtable{mem}
	store.dataStorage = dataStorage
	return store, nil
}

// Get returns a copy of a key's value, or nil and false if the key isn't present.
// An empty or nil stored value is present: use the boolean to distinguish it from a missing key.
func (db *Store) Get(key []byte) ([]byte, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for i := len(db.memtables.queue) - 1; i >= 0; i-- {
		if e, ok := db.memtables.queue[i].Get(key); ok {
			if e.Tombstone {
				return nil, false
			}
			return entryValue(e)
		}
	}

	for j := len(db.sstables) - 1; j >= 0; j-- {
		meta := db.sstables[j]
		f, err := db.dataStorage.OpenFileForReading(meta)
		if err != nil {
			return nil, err
		}
		r := sstable.NewReader(f)
		defer r.Close()

		var entry *memtable.Entry
		entry , err = r.Get(key)
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

func (db *Store) Delete(key []byte) bool {
	m := db.memtables.mutable

	if !m.HasRoomForWrite(key, nil) {
		m = db.rotateMemtables()
	}
	m.PutTombstone(key)
	return true
}

func (db *Store) rotateMemtables() *memtable.Memtable {
	db.memtables.mutable = memtable.New(memtableBytes)
	db.memtables.queue = append(db.memtables.queue, db.memtables.mutable)
	return db.memtables.mutable
}

func (db *Store) maybeScheduleFlush() {
	var totalSize int

	for i := 0; i < len(db.memtables.queue); i++ {
		totalSize += db.memtables.queue[i].ApproximateSize()
	}

	if totalSize <= memtableFlushThreshold {
		return
	}

	err := db.flushMemtables()
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Store) flushMemtables() error {
	n := len(db.memtables.queue) - 1
	flushable := db.memtables.queue[:n]
	db.memtables.queue = db.memtables.queue[n:]

	for i := 0; i < len(flushable); i++ {
		meta := db.dataStorage.PrepareNewFile()
		f, err := db.dataStorage.OpenFileForWriting(meta)
		if err != nil {
			return err
		}

		w := sstable.NewWriter(f)
		err = w.Process(flushable[i])
		if err != nil {
			return err
		}
		err = w.Close()
		if err != nil {
			return err
		}
		db.sstables = append(db.sstables, meta)
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
