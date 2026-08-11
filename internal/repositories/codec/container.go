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
type Container struct {
	data    []byte
	count   int
	key     uint32
	entries []types.Entry
	byName  map[string]int
}

var _ interfaces.IContainer = (*Container)(nil)

// Open reads and parses an NL container from disk.
func Open(path string) (*Container, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return New(data)
}

// New parses an NL container from bytes.
func New(data []byte) (*Container, error) {
	if len(data) < 0x20 || data[0] != 'N' || data[1] != 'L' {
		return nil, fmt.Errorf("not an NL container")
	}
	c := &Container{data: data}
	c.count = int(binary.LittleEndian.Uint16(data[4:6]))
	c.key = binary.LittleEndian.Uint32(data[0x14:0x18])
	c.readDirectory()
	return c, nil
}

// decryptDirectory reverses the 8-bit stream cipher (NGI32.DLL @0x2333B).
func (c *Container) decryptDirectory() []byte {
	ct := c.data[0x20 : 0x20+c.count*32]
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

func (c *Container) readDirectory() {
	dec := c.decryptDirectory()
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

// Raw returns the stored (still-compressed) bytes for an entry.
func (c *Container) Raw(e types.Entry) []byte {
	return c.data[e.Offset : e.Offset+e.CSize]
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
