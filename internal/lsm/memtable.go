package lsm

import (
	"bytes"
	"fmt"

	"github.com/ianeff/lsm-ffs/internal/wal"
)

const (
	defaultMemtableBytes = 16
)

// Memtable is 
type Memtable struct {
	// Strings adapt byte keys to the map; they are not a text encoding.
	// TODO: replace this with a skiplist or something legible and ordered.
	data     map[string]entry
	bytes    int
	maxBytes int
}

func NewMemtable(maxBytes int) *Memtable {
	return &Memtable{data: make(map[string]entry), maxBytes: maxBytes}
}

// put copies key and value so callers can reuse their buffers after insertion.
func (m *Memtable) put(key []byte, e entry) {
	mapKey := string(key)
	// Let's update the byte count
	if old, ok := m.data[mapKey]; ok {
		m.bytes -= len(old.value)
	} else {
		m.bytes += len(key)
	}
	m.bytes += len(e.value)
	// ordered insert needs to go here
	e.value = bytes.Clone(e.value)
	m.data[mapKey] = e
}

// get returns the raw entry for key, tombstone included, so the caller can tell "deleted here" from "not
// here".
func (m *Memtable) get(key []byte) (entry, bool) {
	e, ok := m.data[string(key)]
	return e, ok
}

// apply replays one write-ahead log record into the memtable.
func (m *Memtable) apply(rec wal.Record) error {
	switch rec.Op {
	case wal.OpSet:
		m.put(rec.Key, entry{value: rec.Value})
	case wal.OpDelete:
		m.put(rec.Key, entry{tombstone: true})
	default:
		return fmt.Errorf("unknown op %q for key %q", rec.Op, rec.Key)
	}
	return nil
}

func (m *Memtable) full() bool {
	return m.bytes >= m.maxBytes
}

type entry struct {
	value     []byte
	tombstone bool
}
