// Package webfs serves the game's resources out of memory. The browser has no
// filesystem the engine could read, and no way to block on a fetch from inside
// the game loop, so the browser build downloads everything up front (see
// package webdata) and hands it to the resource layer through this fs.FS.
//
// Nothing here is browser-specific, so it builds and is tested everywhere.
package webfs

import (
	"bytes"
	"io/fs"
	"time"
)

// FS is a read-only fs.FS over a fixed set of files keyed by slash-separated
// path. Its files are *bytes.Reader, which is an io.ReaderAt, so codec.OpenFS
// keeps treating the 115 MB sound bank as a streaming container exactly as it
// does on the desktop — the reads just land in memory instead of on a disk.
type FS struct{ files map[string][]byte }

// New wraps an already-downloaded set of files. The byte slices are kept, not
// copied: they are the game's resources and there is no room to duplicate them.
func New(files map[string][]byte) *FS { return &FS{files: files} }

var (
	_ fs.FS         = (*FS)(nil)
	_ fs.ReadFileFS = (*FS)(nil)
	_ fs.StatFS     = (*FS)(nil)
)

func (f *FS) lookup(op, name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	b, ok := f.files[name]
	if !ok {
		return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}
	return b, nil
}

// Open returns the named file. Every entry is a regular file: the manifest
// carries a flat list of paths, so there are no directories to open.
func (f *FS) Open(name string) (fs.File, error) {
	b, err := f.lookup("open", name)
	if err != nil {
		return nil, err
	}
	return &file{
		Reader: bytes.NewReader(b),
		info:   info{name: name, size: int64(len(b))},
	}, nil
}

// ReadFile hands back the stored slice itself rather than a copy. Callers in
// this codebase only ever read it, and copying would mean holding two copies of
// a resource set that already fills most of the browser's budget.
func (f *FS) ReadFile(name string) ([]byte, error) {
	return f.lookup("readfile", name)
}

// Stat reports the size codec.OpenFS uses to decide whether a container is
// small enough to hold whole or should be read entry by entry.
func (f *FS) Stat(name string) (fs.FileInfo, error) {
	b, err := f.lookup("stat", name)
	if err != nil {
		return nil, err
	}
	return info{name: name, size: int64(len(b))}, nil
}

type file struct {
	*bytes.Reader
	info info
}

func (f *file) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *file) Close() error               { return nil }

type info struct {
	name string
	size int64
}

func (i info) Name() string       { return i.name }
func (i info) Size() int64        { return i.size }
func (i info) Mode() fs.FileMode  { return 0o444 }
func (i info) ModTime() time.Time { return time.Time{} }
func (i info) IsDir() bool        { return false }
func (i info) Sys() any           { return nil }
