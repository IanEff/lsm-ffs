package lsm_test

import (
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
	got, ok := db.Get("a")
	if !ok {
		t.Fatal("key not found")
	}
	if got != want {
		t.Errorf("wrong value after Set (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestSet_ClobbersEarlierValueForSameKey(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)
	want := "2"

	mustSet(t, db, "a", "1")
	mustSet(t, db, "a", "2")
	got, ok := db.Get("a")
	if !ok {
		t.Fatal("key not found")
	}
	if got != want {
		t.Errorf("second Set didn't clobber the first (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestGet_DistinguishesEmptyValueFromMissingKey(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)

	mustSet(t, db, "a", "")
	got, ok := db.Get("a")
	if !ok {
		t.Fatal("key set to empty value not found")
	}
	if got != "" {
		t.Errorf("wrong value for a key set to empty (-want +got):\n%s", cmp.Diff("", got))
	}

	got, ok = db.Get("b")
	if ok {
		t.Fatal("missing key found")
	}
	if got != "" {
		t.Errorf("missing key returned a value (-want +got):\n%s", cmp.Diff("", got))
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
			if err := db.Set("a", "1"); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for range 100 {
		db.Get("a")
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

	err := db.Set("a", "1")
	if err == nil {
		t.Error("want error from Set on a closed store, got nil")
	}
}

// Durability: what survives closing the store and opening it again on the same directory.

func TestOpen_ReturnsEmptyStoreForEmptyDir(t *testing.T) {
	t.Parallel()
	db := newTestStore(t)

	_, ok := db.Get("a")
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
		v, ok := db.Get(k)
		if !ok {
			t.Errorf("key %q missing after reopen", k)
			continue
		}
		got[k] = v
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
	got, ok := db.Get("a")
	if !ok {
		t.Fatal("key missing after reopen")
	}
	if got != want {
		t.Errorf("replay didn't keep the last value (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestOpen_RestoresEmptyValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	db := openStore(t, dir)
	mustSet(t, db, "a", "")
	mustClose(t, db)

	db = openStore(t, dir)
	got, ok := db.Get("a")
	if !ok {
		t.Fatal("key set to empty value missing after reopen")
	}
	if got != "" {
		t.Errorf("wrong value after reopen (-want +got):\n%s", cmp.Diff("", got))
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
	if err := db.Set(key, value); err != nil {
		t.Fatal(err)
	}
}

func mustClose(t *testing.T, db *lsm.Store) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}
