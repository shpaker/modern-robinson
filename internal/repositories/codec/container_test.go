package codec

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// nlPack builds a one-entry NL container holding payload DEFLATE-compressed
// (method 0x100), with the entry's stored size overstated by over bytes.
func nlPack(t *testing.T, payload []byte, over int) []byte {
	t.Helper()
	var z bytes.Buffer
	w, err := flate.NewWriter(&z, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	const key = 0x5A17
	head := make([]byte, 0x20)
	head[0], head[1] = 'N', 'L'
	binary.LittleEndian.PutUint16(head[4:6], 1)
	binary.LittleEndian.PutUint32(head[0x14:0x18], key)

	dir := make([]byte, 32)
	copy(dir, "PALETTE.COL")
	off := len(head) + len(dir)
	binary.LittleEndian.PutUint16(dir[0x10:0x12], 0x100)
	binary.LittleEndian.PutUint32(dir[0x14:0x18], uint32(len(payload)))
	binary.LittleEndian.PutUint32(dir[0x18:0x1C], uint32(off))
	binary.LittleEndian.PutUint32(dir[0x1C:0x20], uint32(z.Len()+over))
	// The directory cipher is a keystream XOR, so it encrypts as it decrypts.
	dir = (&Container{key: key}).decryptDirectory(dir)

	return append(append(head, dir...), z.Bytes()...)
}

// The minigame packs store their last entry one byte longer than the file:
// the entry has to come out whole all the same, resident or streamed, or the
// hut and the map puzzles lose their palettes and turn black.
func TestLastEntryPastTheEndIsCutNotDropped(t *testing.T) {
	payload := bytes.Repeat([]byte{1, 2, 3, 4}, 256)
	data := nlPack(t, payload, 1)

	resident, err := New(data)
	if err != nil {
		t.Fatal(err)
	}
	// Only the sound bank is big enough to stream for real; the same container
	// reading from a file on disk takes that path.
	path := filepath.Join(t.TempDir(), "PACK.DAT")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	streamed := *resident
	streamed.data, streamed.f = nil, f
	for name, c := range map[string]*Container{
		"resident": resident, "streamed": &streamed,
	} {
		got, err := c.ExtractName("PALETTE.COL")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(got, payload) {
			t.Errorf("%s: got %d bytes, want the %d stored", name, len(got),
				len(payload))
		}
	}
}

// An entry that starts past the end of the file is damage, not the packer's
// slip: it reports empty rather than reading out of bounds.
func TestEntryStartingPastTheEndIsEmpty(t *testing.T) {
	data := nlPack(t, []byte("x"), 0)
	c, err := New(data)
	if err != nil {
		t.Fatal(err)
	}
	e := c.Entries()[0]
	e.Offset = uint32(len(data) + 1)
	if raw := c.Raw(e); raw != nil {
		t.Errorf("Raw = %d bytes, want nil", len(raw))
	}
}
