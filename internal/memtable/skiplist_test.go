package memtable_test

import (
	"math/rand/v2"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ianeff/lsm-ffs/internal/memtable"
)

func TestFind_ReturnsFalseOnEmptyList(t *testing.T) {
	t.Parallel()
	sl := memtable.NewSkipList()

	if _, ok := sl.Find([]byte("a")); ok {
		t.Error("empty list found a key")
	}
}

func TestFind_ReturnsEntryStoredByInsert(t *testing.T) {
	t.Parallel()
	sl := memtable.NewSkipList()
	want := memtable.Entry{Value: []byte("1")}

	sl.Insert([]byte("a"), want)
	got, ok := sl.Find([]byte("a"))
	if !ok {
		t.Fatal("key not found")
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("wrong entry after Insert (-want +got):\n%s", diff)
	}
}

func TestInsert_ReplacesEntryForSameKey(t *testing.T) {
	t.Parallel()
	sl := memtable.NewSkipList()
	want := memtable.Entry{Tombstone: true}

	sl.Insert([]byte("a"), memtable.Entry{Value: []byte("1")})
	sl.Insert([]byte("a"), want)
	got, ok := sl.Find([]byte("a"))
	if !ok {
		t.Fatal("key not found")
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("second Insert didn't replace the first (-want +got):\n%s", diff)
	}
}

func TestFind_DistinguishesPrefixKeys(t *testing.T) {
	t.Parallel()
	sl := memtable.NewSkipList()
	for _, k := range []string{"abc", "a", "ab"} {
		sl.Insert([]byte(k), memtable.Entry{Value: []byte(k)})
	}

	for _, k := range []string{"a", "ab", "abc"} {
		got, ok := sl.Find([]byte(k))
		if !ok {
			t.Errorf("key %q not found", k)
			continue
		}
		if string(got.Value) != k {
			t.Errorf("Find(%q) returned the entry for %q", k, got.Value)
		}
	}
}

func TestFind_ReturnsFalseForMissingKeyBeforeBetweenAndAfter(t *testing.T) {
	t.Parallel()
	sl := memtable.NewSkipList()
	sl.Insert([]byte("b"), memtable.Entry{Value: []byte("1")})
	sl.Insert([]byte("d"), memtable.Entry{Value: []byte("2")})

	for _, k := range []string{"a", "c", "e"} {
		if _, ok := sl.Find([]byte(k)); ok {
			t.Errorf("missing key %q found", k)
		}
	}
}

func TestFind_HandlesEmptyKey(t *testing.T) {
	t.Parallel()
	sl := memtable.NewSkipList()
	want := memtable.Entry{Value: []byte("empty")}

	sl.Insert([]byte("a"), memtable.Entry{Value: []byte("1")})
	sl.Insert([]byte{}, want)
	got, ok := sl.Find(nil)
	if !ok {
		t.Fatal("empty key not found")
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("wrong entry for empty key (-want +got):\n%s", diff)
	}
}

// TestSkipList_MatchesMapForManyRandomKeys inserts enough keys to build towers taller than one level, with
// repeats, and checks every key against a map. The seed is fixed so a failure reproduces.
func TestSkipList_MatchesMapForManyRandomKeys(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 2))
	sl := memtable.NewSkipList()
	want := make(map[string]memtable.Entry)

	for i := range 2000 {
		key := make([]byte, rng.IntN(4))
		for j := range key {
			key[j] = byte(rng.IntN(16))
		}
		e := memtable.Entry{Value: []byte{byte(i), byte(i >> 8)}}
		sl.Insert(key, e)
		want[string(key)] = e
	}

	for k, w := range want {
		got, ok := sl.Find([]byte(k))
		if !ok {
			t.Errorf("key %x not found", k)
			continue
		}
		if diff := cmp.Diff(w, got); diff != "" {
			t.Errorf("wrong entry for key %x (-want +got):\n%s", k, diff)
		}
	}
}
