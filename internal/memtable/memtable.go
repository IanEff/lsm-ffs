// Package memtable holds an LSM store's recent writes in memory, sorted by key.
package memtable

import (
	"bytes"
	"fmt"

	"github.com/ianeff/lsm-ffs/internal/wal"
)

// Memtable maps byte keys to entries in key order. Deleting a key means putting a tombstone; nothing is
// ever removed. Not safe for concurrent use.
type Memtable struct {
	data      *SkipList
	bytesUsed int // total length of keys and values held
	maxBytes  int
}

// New returns an empty memtable.
func New(maxBytes int) *Memtable {
	return &Memtable{data: NewSkipList(), maxBytes: maxBytes}
}

// Put stores e under key, replacing any earlier entry. It copies key and e.Value, so the caller may reuse
// both.
func (m *Memtable) Put(key []byte, e Entry) {
	if old, ok := m.data.Find(key); ok {
		m.bytesUsed -= len(old.Value)
	} else {
		m.bytesUsed += len(key)
	}
	m.bytesUsed += len(e.Value)

	e.Value = bytes.Clone(e.Value)
	m.data.Insert(bytes.Clone(key), e)
}

// Get returns key's entry, tombstone included, so the caller can tell "deleted here" from "not here".
// Don't modify the returned Value; the memtable still holds it.
func (m *Memtable) Get(key []byte) (Entry, bool) {
	return m.data.Find(key)
}

// Apply replays one WAL record into the memtable. It returns an error for an unknown op.
func (m *Memtable) Apply(rec wal.Record) error {
	switch rec.Op {
	case wal.OpSet:
		m.Put(rec.Key, Entry{Value: rec.Value})
	case wal.OpDelete:
		m.Put(rec.Key, Entry{Tombstone: true})
	default:
		return fmt.Errorf("unknown op %q for key %q", rec.Op, rec.Key)
	}
	return nil
}

// ApproximateSize returns the total length of keys and values held.
func (m *Memtable) ApproximateSize() int {
	return m.bytesUsed
}

func (m *Memtable) HasRoomForWrite(key, value []byte) bool {
	// this is for a "soft flush threshold"-- it allows a bigger write than the table strictly holds
	if m.bytesUsed == 0 {
		return true
	}
	return len(key)+len(value) <= m.maxBytes-m.bytesUsed
}

// Entry is what a [Memtable] holds for a key: a value, or a tombstone marking the key deleted.
type Entry struct {
	Value     []byte
	Tombstone bool
}
