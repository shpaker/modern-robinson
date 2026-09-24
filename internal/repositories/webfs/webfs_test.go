package webfs

import (
	"errors"
	"io"
	"io/fs"
	"testing"
)

func testFS() *FS {
	return New(map[string][]byte{
		"DATA/WAVE/WAVE.DAN": []byte("sound bank"),
		"LOGO.DAT":           []byte("logo"),
	})
}

func TestReadFileHandsBackTheStoredSlice(t *testing.T) {
	want := []byte("sound bank")
	f := New(map[string][]byte{"a": want})
	got, err := f.ReadFile("a")
	if err != nil {
		t.Fatal(err)
	}
	// Not merely equal: the same backing array. Copying the resource set would
	// double the browser build's peak memory, which is the whole constraint.
	if &got[0] != &want[0] {
		t.Error("ReadFile copied the file instead of sharing it")
	}
}

// The resource layer opens a large container and reads entries out of it at
// play time, which needs random access, not a stream.
func TestOpenedFilesSupportReadAt(t *testing.T) {
	f, err := testFS().Open("DATA/WAVE/WAVE.DAN")
	if err != nil {
		t.Fatal(err)
	}
	ra, ok := f.(io.ReaderAt)
	if !ok {
		t.Fatal(
			"an opened file must be an io.ReaderAt, or codec falls back to " +
				"reading the whole 115 MB bank into a second copy",
		)
	}
	buf := make([]byte, 4)
	if _, err := ra.ReadAt(buf, 6); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "bank" {
		t.Errorf("ReadAt = %q, want %q", buf, "bank")
	}
}

func TestStatReportsSize(t *testing.T) {
	st, err := testFS().Stat("LOGO.DAT")
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != 4 {
		t.Errorf("Size = %d, want 4", st.Size())
	}
	if st.IsDir() {
		t.Error("a manifest entry is never a directory")
	}
}

// fs.ErrNotExist is what codec.OpenFS turns into "no such container", which the
// resource layer treats as a harmless miss; any other error shape would leak.
func TestMissingFileIsErrNotExist(t *testing.T) {
	for _, call := range []struct {
		name string
		fn   func() error
	}{
		{"Open", func() error { _, err := testFS().Open("NOPE.DAT"); return err }},
		{"ReadFile", func() error { _, err := testFS().ReadFile("NOPE.DAT"); return err }},
		{"Stat", func() error { _, err := testFS().Stat("NOPE.DAT"); return err }},
	} {
		err := call.fn()
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s: err = %v, want fs.ErrNotExist", call.name, err)
		}
	}
}

func TestInvalidPathRejected(t *testing.T) {
	_, err := testFS().Open("../escape")
	if !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("err = %v, want fs.ErrInvalid", err)
	}
}
