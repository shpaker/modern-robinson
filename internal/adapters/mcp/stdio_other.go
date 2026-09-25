//go:build !unix

package mcp

import (
	"io"
	"os"
)

// claimStdout takes stdout for the protocol and sends the process's own
// writes to stderr.
func claimStdout() (io.WriteCloser, error) {
	out := os.Stdout
	os.Stdout = os.Stderr
	return out, nil
}
