//go:build unix

package mcp

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// claimStdout takes descriptor 1 for the protocol and points it at stderr, so
// whatever else writes to stdout — the engine, a C library — cannot break
// the stream.
func claimStdout() (io.WriteCloser, error) {
	fd, err := unix.Dup(1)
	if err != nil {
		return nil, err
	}
	if err := unix.Dup2(2, 1); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return os.NewFile(uintptr(fd), "mcp"), nil
}
