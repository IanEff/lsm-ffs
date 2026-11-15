package wal_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ianeff/lsm-ffs/internal/wal"
)

func TestReplay_ReturnsNoRecordsForMissingFile(t *testing.T) {
	t.Parallel()

	got, err := wal.Replay(filepath.Join(t.TempDir(), "wal.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("want no records from a missing log, got %+v", got)
	}
}

func TestReplay_ReturnsAppendedRecordsInOrder(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "wal.log")
	want := []wal.Record{
		{Op: wal.OpSet, Key: []byte("a"), Value: []byte("1")},
		{Op: wal.OpDelete, Key: []byte("a")},
		{Op: wal.OpSet, Key: []byte("a"), Value: []byte("2")},
	}

	appendAll(t, path, want...)
	got := mustReplay(t, path)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("wrong records after replay (-want +got):\n%s", diff)
	}
}

func TestReplay_PreservesBinaryEmptyAndNilBytes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "wal.log")
	want := []wal.Record{
		{Op: wal.OpSet, Key: []byte{0xff, 0, 'k'}, Value: []byte{0, 0xfe, '\n'}},
		{Op: wal.OpSet, Key: []byte{}, Value: []byte{}},
		{Op: wal.OpSet, Key: nil, Value: nil},
	}

	appendAll(t, path, want...)
	got := mustReplay(t, path)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("bytes changed in replay (-want +got):\n%s", diff)
	}
}

func TestOpen_AppendsToExistingLog(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "wal.log")
	first := wal.Record{Op: wal.OpSet, Key: []byte("a"), Value: []byte("1")}
	second := wal.Record{Op: wal.OpSet, Key: []byte("b"), Value: []byte("2")}

	appendAll(t, path, first)
	appendAll(t, path, second)
	got := mustReplay(t, path)
	if diff := cmp.Diff([]wal.Record{first, second}, got); diff != "" {
		t.Errorf("reopening lost or reordered records (-want +got):\n%s", diff)
	}
}

func TestAppend_RecordIsReplayableBeforeClose(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "wal.log")
	want := []wal.Record{{Op: wal.OpSet, Key: []byte("a"), Value: []byte("1")}}

	w, err := wal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := w.Append(want[0]); err != nil {
		t.Fatal(err)
	}

	got := mustReplay(t, path)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("appended record not in the log before Close (-want +got):\n%s", diff)
	}
}

func TestOpen_ReturnsErrorForUnwritablePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	w, err := wal.Open(filepath.Join(dir, "wal.log"))
	if err == nil {
		w.Close()
		t.Error("want error opening a log in a read-only dir, got nil")
	}
}

func TestReplay_PanicsOnCorruptRecord(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		corrupt func(t *testing.T, path string)
	}{
		{name: "garbage", corrupt: func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("not json\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "torn", corrupt: func(t *testing.T, path string) {
			appendAll(t, path, wal.Record{Op: wal.OpSet, Key: []byte("a"), Value: []byte("1")})
			tearLastRecord(t, path)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "wal.log")
			tc.corrupt(t, path)

			mustPanic(t, func() { wal.Replay(path) })
		})
	}
}

// appendAll opens the log at path, appends recs and closes it.
func appendAll(t *testing.T, path string, recs ...wal.Record) {
	t.Helper()
	w, err := wal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range recs {
		if err := w.Append(rec); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func mustReplay(t *testing.T, path string) []wal.Record {
	t.Helper()
	recs, err := wal.Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

// tearLastRecord appends the first half of the log's contents to itself, leaving a partial record at the
// end, as a crash during Append would.
func tearLastRecord(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, data[:len(data)/2]...), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("want panic, got none")
		}
	}()
	f()
}
