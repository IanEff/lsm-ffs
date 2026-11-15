package lsm_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ianeff/lsm-ffs/internal/lsm"
)

// Behavior: what a single open store does.

func TestGet_ReturnsValueStoredBySet(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)
	want := "1"

	mustSet(t, db, "a", "1")
	got, ok := db.Get([]byte("a"))
	if !ok {
		t.Fatal("key not found")
	}
	if string(got) != want {
		t.Errorf("wrong value after Set (-want +got):\n%s", cmp.Diff(want, string(got)))
	}
}

func TestSet_ClobbersEarlierValueForSameKey(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)
	want := "2"

	mustSet(t, db, "a", "1")
	mustSet(t, db, "a", "2")
	got, ok := db.Get([]byte("a"))
	if !ok {
		t.Fatal("key not found")
	}
	if string(got) != want {
		t.Errorf("second Set didn't clobber the first (-want +got):\n%s", cmp.Diff(want, string(got)))
	}
}

func TestGet_DistinguishesEmptyValueFromMissingKey(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)

	mustSet(t, db, "a", "")
	got, ok := db.Get([]byte("a"))
	if !ok {
		t.Fatal("key set to empty value not found")
	}
	if len(got) != 0 {
		t.Errorf("wrong value for a key set to empty (-want +got):\n%s", cmp.Diff("", string(got)))
	}

	got, ok = db.Get([]byte("b"))
	if ok {
		t.Fatal("missing key found")
	}
	if len(got) != 0 {
		t.Errorf("missing key returned a value (-want +got):\n%s", cmp.Diff("", string(got)))
	}
}

// TestStore_IsRaceFree exercises the store with a concurrent reader and writer so the race detector can catch
// unsynchronized access. The channel is established so we wait on the writer to finish before exiting the
// test.
func TestStore_IsRaceFree(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			if err := db.Set([]byte("a"), []byte("1")); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for range 100 {
		db.Get([]byte("a"))
	}
	// Let the writer finish before we return.
	<-done
}

func TestSet_ReturnsErrorAfterClose(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	err := db.Set([]byte("a"), []byte("1"))
	if err == nil {
		t.Error("want error from Set on a closed store, got nil")
	}
}

func TestSet_CopiesKeyAndValue(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)
	key, value := []byte("key"), []byte("value")
	if err := db.Set(key, value); err != nil {
		t.Fatal(err)
	}
	key[0] = 'x'
	value[0] = 'x'

	got, ok := db.Get([]byte("key"))
	if !ok || !bytes.Equal(got, []byte("value")) {
		t.Fatalf("stored data changed with input buffers: got %q, present %t", got, ok)
	}
	if _, ok := db.Get(key); ok {
		t.Fatal("mutating the input key introduced a new key")
	}
}

func TestGet_ReturnsCopy(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)
	mustSet(t, db, "key", "value")
	got, ok := db.Get([]byte("key"))
	if !ok || len(got) == 0 {
		t.Fatal("stored value missing")
	}
	got[0] = 'x'

	got, ok = db.Get([]byte("key"))
	if !ok || !bytes.Equal(got, []byte("value")) {
		t.Fatalf("stored data changed with returned buffer: got %q, present %t", got, ok)
	}
}

// Durability: what survives closing the store and opening it again on the same directory.

func TestOpen_RestoresByteKeysAndValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		key, value []byte
	}{
		{name: "binary", key: []byte{0xff, 0, 'k'}, value: []byte{0, 0xfe, '\n'}},
		{name: "empty", key: []byte{}, value: []byte{}},
		{name: "nil", key: nil, value: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			db := openStore(t, dir)
			if err := db.Set(tc.key, tc.value); err != nil {
				t.Fatal(err)
			}
			got, ok := db.Get(tc.key)
			if !ok || !bytes.Equal(got, tc.value) {
				t.Fatalf("before reopen: got %x, present %t; want %x", got, ok, tc.value)
			}
			mustClose(t, db)

			db = openStore(t, dir)
			got, ok = db.Get(tc.key)
			if !ok || !bytes.Equal(got, tc.value) {
				t.Fatalf("after reopen: got %x, present %t; want %x", got, ok, tc.value)
			}
		})
	}
}

func TestOpen_ReturnsEmptyStoreForEmptyDir(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)

	_, ok := db.Get([]byte("a"))
	if ok {
		t.Error("fresh store found a key")
	}
}

func TestOpen_RestoresValuesSetBeforeClose(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	want := map[string]string{"a": "1", "b": "2"}

	db := openStore(t, dir)
	for k, v := range want {
		mustSet(t, db, k, v)
	}
	mustClose(t, db)

	db = openStore(t, dir)
	got := make(map[string]string)
	for k := range want {
		v, ok := db.Get([]byte(k))
		if !ok {
			t.Errorf("key %q missing after reopen", k)
			continue
		}
		got[k] = string(v)
	}
	if !cmp.Equal(want, got) {
		t.Errorf("wrong values after reopen (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestOpen_KeepsLastValueForKeySetTwice(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	want := "2"

	db := openStore(t, dir)
	mustSet(t, db, "a", "1")
	mustSet(t, db, "a", "2")
	mustClose(t, db)

	db = openStore(t, dir)
	got, ok := db.Get([]byte("a"))
	if !ok {
		t.Fatal("key missing after reopen")
	}
	if string(got) != want {
		t.Errorf("replay didn't keep the last value (-want +got):\n%s", cmp.Diff(want, string(got)))
	}
}

func TestOpen_RestoresEmptyValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	db := openStore(t, dir)
	mustSet(t, db, "a", "")
	mustClose(t, db)

	db = openStore(t, dir)
	got, ok := db.Get([]byte("a"))
	if !ok {
		t.Fatal("key set to empty value missing after reopen")
	}
	if len(got) != 0 {
		t.Errorf("wrong value after reopen (-want +got):\n%s", cmp.Diff("", string(got)))
	}
}

func TestOpen_ReturnsErrorForUnwritableDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	_, err := lsm.Open(dir)
	if err == nil {
		t.Error("want error opening a store in a read-only dir, got nil")
	}
}

// openStore opens the store in dir and closes it when the test ends. Tests that reopen the same dir close
// it themselves first; Close is safe to call twice.
func openStore(t *testing.T, dir string) *lsm.Store {
	t.Helper()
	db, err := lsm.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func newTestStore(t *testing.T) *lsm.Store {
	t.Helper()
	return openStore(t, t.TempDir())
}

func mustSet(t *testing.T, db *lsm.Store, key, value string) {
	t.Helper()
	if err := db.Set([]byte(key), []byte(value)); err != nil {
		t.Fatal(err)
	}
}

func mustClose(t *testing.T, db *lsm.Store) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}
