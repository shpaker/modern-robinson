// Package codec decodes NGI ("Nikita Game Interface") resource containers:
// the NL container format, its encrypted directory, and the LZSS/LZHUF/NGB
// payloads. It lives in the Infrastructure layer. See ../../../../docs.
package codec

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// Container is a parsed NL resource file. It implements interfaces.IContainer.
//
// Small containers are held in memory; a large one keeps only its directory and
// reads entries from disk on demand. The sound bank is the reason: WAVE.DAN is
// 115 MB of uncompressed PCM, so holding it resident costs that much for the
// whole session while any one sound needs a few dozen kilobytes.
type Container struct {
	data    []byte   // whole file, or nil when entries stream from f
	f       *os.File // open file for streaming containers
	size    int64
	count   int
	key     uint32
	entries []types.Entry
	byName  map[string]int
}

// residentLimit is the largest container kept fully in memory.
const residentLimit = 16 << 20

var _ interfaces.IContainer = (*Container)(nil)

// Open parses an NL container from disk, streaming it when it is large.
func Open(path string) (*Container, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() <= residentLimit {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return New(data)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	head := make([]byte, 0x20)
	if _, err := f.ReadAt(head, 0); err != nil {
		_ = f.Close()
		return nil, err
	}
	c := &Container{f: f, size: st.Size()}
	if err := c.parseHeader(head); err != nil {
		_ = f.Close()
		return nil, err
	}
	dir := make([]byte, c.count*32)
	if _, err := f.ReadAt(dir, 0x20); err != nil {
		_ = f.Close()
		return nil, err
	}
	c.readDirectoryFrom(dir)
	return c, nil
}

// New parses an NL container from bytes.
func New(data []byte) (*Container, error) {
	c := &Container{data: data, size: int64(len(data))}
	if err := c.parseHeader(data); err != nil {
		return nil, err
	}
	// The directory is 32 bytes per entry and the count is an unchecked field
	// of the header, so a truncated file would otherwise slice past the end.
	end := 0x20 + c.count*32
	if end > len(data) {
		return nil, fmt.Errorf(
			"NL directory needs %d bytes, file has %d", end, len(data),
		)
	}
	c.readDirectoryFrom(data[0x20:end])
	return c, nil
}

// parseHeader validates the magic and reads the entry count and cipher key.
func (c *Container) parseHeader(head []byte) error {
	if len(head) < 0x20 || head[0] != 'N' || head[1] != 'L' {
		return fmt.Errorf("not an NL container")
	}
	c.count = int(binary.LittleEndian.Uint16(head[4:6]))
	c.key = binary.LittleEndian.Uint32(head[0x14:0x18])
	return nil
}

// decryptDirectory reverses the 8-bit stream cipher (NGI32.DLL @0x2333B).
func (c *Container) decryptDirectory(ct []byte) []byte {
	al := c.key & 0xFF
	dl := (c.key >> 8) & 0xFF
	out := make([]byte, len(ct))
	for i, cb := range ct {
		al = ((al << 1) & 0xFF) ^ dl
		dl = (dl >> 1) & 0xFF
		out[i] = byte(uint32(cb) ^ al)
		dl = (dl ^ al) & 0xFF
	}
	return out
}

func (c *Container) readDirectoryFrom(ct []byte) {
	dec := c.decryptDirectory(ct)
	c.entries = make([]types.Entry, c.count)
	c.byName = make(map[string]int, c.count)
	for i := 0; i < c.count; i++ {
		e := dec[i*32 : (i+1)*32]
		name := cstr(e[:12])
		c.entries[i] = types.Entry{
			Name:   name,
			Method: binary.LittleEndian.Uint16(e[0x10:0x12]),
			ID:     binary.LittleEndian.Uint16(e[0x12:0x14]),
			USize:  binary.LittleEndian.Uint32(e[0x14:0x18]),
			Offset: binary.LittleEndian.Uint32(e[0x18:0x1C]),
			CSize:  binary.LittleEndian.Uint32(e[0x1C:0x20]),
		}
		c.byName[strings.ToUpper(name)] = i
	}
}

// Entries returns the container's directory.
func (c *Container) Entries() []types.Entry { return c.entries }

// Raw returns the stored (still-compressed) bytes for an entry, reading them
// from disk when the container streams. Offsets and sizes come from the file
// itself, so they are range-checked: a truncated or damaged container reports an
// empty entry instead of taking the process down.
func (c *Container) Raw(e types.Entry) []byte {
	start, end := int64(e.Offset), int64(e.Offset)+int64(e.CSize)
	if start < 0 || end < start || end > c.size {
		return nil
	}
	if c.data != nil {
		return c.data[start:end]
	}
	buf := make([]byte, e.CSize)
	if _, err := c.f.ReadAt(buf, start); err != nil {
		return nil
	}
	return buf
}

// Extract returns the fully decoded bytes for an entry.
func (c *Container) Extract(e types.Entry) ([]byte, error) {
	blob := c.Raw(e)
	if !e.Compressed() {
		return blob, nil
	}
	switch {
	case e.Method&0x80 != 0:
		return lzhufDecompress(blob, int(e.USize)), nil
	case e.Method&0x1E0 == 0x40:
		return lzssDecompress(blob, int(e.USize)), nil
	case e.Method&0x1E0 == 0x100:
		// graphics variant = raw DEFLATE (RFC 1951, no zlib header)
		r := flate.NewReader(bytes.NewReader(blob))
		defer func() { _ = r.Close() }()
		return io.ReadAll(r)
	default:
		return nil, fmt.Errorf(
			"method %#x for %q not implemented",
			e.Method,
			e.Name,
		)
	}
}

// Find returns the entry with the given (case-insensitive) name.
func (c *Container) Find(name string) (types.Entry, bool) {
	if i, ok := c.byName[strings.ToUpper(name)]; ok {
		return c.entries[i], true
	}
	return types.Entry{}, false
}

// FindExt returns the first entry whose name ends with ext (case-insensitive).
func (c *Container) FindExt(ext string) (types.Entry, bool) {
	ext = strings.ToUpper(ext)
	for _, e := range c.entries {
		if strings.HasSuffix(strings.ToUpper(e.Name), ext) {
			return e, true
		}
	}
	return types.Entry{}, false
}

// ExtractName extracts an entry by name.
func (c *Container) ExtractName(name string) ([]byte, error) {
	e, ok := c.Find(name)
	if !ok {
		return nil, fmt.Errorf("entry %q not found", name)
	}
	return c.Extract(e)
}

func cstr(b []byte) string {
	for i, ch := range b {
		if ch == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
