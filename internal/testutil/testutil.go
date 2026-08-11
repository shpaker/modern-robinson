// Package testutil holds shared test helpers.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// GameRoot returns the extracted game folder by walking up from the test's cwd
// until it finds extracted/ROBINSON_ISO/ROBINSON, or skips if not present.
func GameRoot(t *testing.T) string {
	t.Helper()
	rel := filepath.Join("extracted", "ROBINSON_ISO", "ROBINSON")
	dir := "."
	for i := 0; i < 8; i++ {
		root := filepath.Join(dir, rel)
		if _, err := os.Stat(filepath.Join(root, "DATA", "OPTIONS.DAT")); err == nil {
			return root
		}
		dir = filepath.Join(dir, "..")
	}
	t.Skip("game resources not present; skipping")
	return ""
}
