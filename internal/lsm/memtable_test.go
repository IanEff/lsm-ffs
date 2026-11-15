package lsm

import (
	"bytes"
	"testing"

	"github.com/ianeff/lsm-ffs/internal/wal"
)

func TestMemtable_ByteKeysAndAccounting(t *testing.T) {
	t.Parallel()
	m := NewMemtable(5)
	key, value := []byte{0xff, 0}, []byte{1, 2, 3}
	m.put(key, entry{value: value})
	key[0] = 0
	value[0] = 0

	e, ok := m.get([]byte{0xff, 0})
	if !ok || !bytes.Equal(e.value, []byte{1, 2, 3}) {
		t.Fatalf("entry changed with input buffers: got %+v, present %t", e, ok)
	}
	if m.bytes != 5 || !m.full() {
		t.Fatalf("after insertion: bytes %d, full %t", m.bytes, m.full())
	}

	m.put([]byte{0xff, 0}, entry{value: []byte{4}})
	if m.bytes != 3 || m.full() {
		t.Fatalf("after replacement: bytes %d, full %t", m.bytes, m.full())
	}

	if err := m.apply(wal.Record{Op: wal.OpDelete, Key: []byte{0xff, 0}}); err != nil {
		t.Fatal(err)
	}
	e, ok = m.get([]byte{0xff, 0})
	if !ok || !e.tombstone || m.bytes != 2 {
		t.Fatalf("after delete: entry %+v, present %t, bytes %d", e, ok, m.bytes)
	}

	if err := m.apply(wal.Record{Op: wal.OpSet, Key: []byte{0xff, 0}, Value: []byte{5, 6}}); err != nil {
		t.Fatal(err)
	}
	e, ok = m.get([]byte{0xff, 0})
	if !ok || e.tombstone || !bytes.Equal(e.value, []byte{5, 6}) || m.bytes != 4 {
		t.Fatalf("after reinsertion: entry %+v, present %t, bytes %d", e, ok, m.bytes)
	}
}
