// Package testutil holds shared test helpers.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// GameRoot returns the extracted game folder, trying a couple of repo-relative
// depths, or skips the test if the (proprietary) resources aren't present.
func GameRoot(t *testing.T) string {
	t.Helper()
	rel := filepath.Join("extracted", "ROBINSON_ISO", "ROBINSON")
	for _, up := range []string{
		filepath.Join("..", "..", "..", rel),       // internal/<layer>/
		filepath.Join("..", "..", "..", "..", rel), // internal/<layer>/<sub>/
	} {
		if _, err := os.Stat(filepath.Join(up, "DATA", "OPTIONS.DAT")); err == nil {
			return up
		}
	}
	t.Skip("game resources not present; skipping")
	return ""
}
