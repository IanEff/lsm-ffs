package memtable_test

import (
	"bytes"
	"testing"

	"github.com/ianeff/lsm-ffs/internal/memtable"
	"github.com/ianeff/lsm-ffs/internal/wal"
)

func TestMemtable_ByteKeysAndAccounting(t *testing.T) {
	t.Parallel()
	m := memtable.New()
	key, value := []byte{0xff, 0}, []byte{1, 2, 3}
	m.Put(key, memtable.Entry{Value: value})
	key[0] = 0
	value[0] = 0

	e, ok := m.Get([]byte{0xff, 0})
	if !ok || !bytes.Equal(e.Value, []byte{1, 2, 3}) {
		t.Fatalf("entry changed with input buffers: got %+v, present %t", e, ok)
	}
	if got := m.ApproximateSize(); got != 5 {
		t.Fatalf("after insertion: size %d, want 5", got)
	}

	m.Put([]byte{0xff, 0}, memtable.Entry{Value: []byte{4}})
	if got := m.ApproximateSize(); got != 3 {
		t.Fatalf("after replacement: size %d, want 3", got)
	}

	if err := m.Apply(wal.Record{Op: wal.OpDelete, Key: []byte{0xff, 0}}); err != nil {
		t.Fatal(err)
	}
	e, ok = m.Get([]byte{0xff, 0})
	if !ok || !e.Tombstone || m.ApproximateSize() != 2 {
		t.Fatalf("after delete: entry %+v, present %t, size %d", e, ok, m.ApproximateSize())
	}

	if err := m.Apply(wal.Record{Op: wal.OpSet, Key: []byte{0xff, 0}, Value: []byte{5, 6}}); err != nil {
		t.Fatal(err)
	}
	e, ok = m.Get([]byte{0xff, 0})
	if !ok || e.Tombstone || !bytes.Equal(e.Value, []byte{5, 6}) || m.ApproximateSize() != 4 {
		t.Fatalf("after reinsertion: entry %+v, present %t, size %d", e, ok, m.ApproximateSize())
	}
}

func TestGet_ReturnsFalseForMissingKey(t *testing.T) {
	t.Parallel()
	m := memtable.New()
	m.Put([]byte("a"), memtable.Entry{Value: []byte("1")})

	if e, ok := m.Get([]byte("b")); ok {
		t.Errorf("missing key found: %+v", e)
	}
}

func TestPut_ValueReplacesTombstone(t *testing.T) {
	t.Parallel()
	m := memtable.New()

	m.Put([]byte("a"), memtable.Entry{Tombstone: true})
	m.Put([]byte("a"), memtable.Entry{Value: []byte("1")})
	e, ok := m.Get([]byte("a"))
	if !ok || e.Tombstone || !bytes.Equal(e.Value, []byte("1")) {
		t.Fatalf("value didn't replace tombstone: entry %+v, present %t", e, ok)
	}
}

func TestApply_ReturnsErrorForUnknownOp(t *testing.T) {
	t.Parallel()
	m := memtable.New()

	err := m.Apply(wal.Record{Op: "BOGUS", Key: []byte("a"), Value: []byte("1")})
	if err == nil {
		t.Fatal("want error for unknown op, got nil")
	}
	if e, ok := m.Get([]byte("a")); ok || m.ApproximateSize() != 0 {
		t.Errorf("unknown op changed the memtable: entry %+v, present %t, size %d", e, ok, m.ApproximateSize())
	}
}
