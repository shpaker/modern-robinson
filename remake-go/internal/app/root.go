package app

import (
	"os"
	"path/filepath"
)

// isGameDir reports whether p looks like an extracted game folder.
func isGameDir(p string) bool {
	_, err := os.Stat(filepath.Join(p, "DATA", "WAVE", "WAVE.DAN"))
	return err == nil
}

// FindRoot locates the game resource folder: an explicit arg, the current
// directory, the binary's directory, or a dev fallback inside the repo.
func FindRoot(arg string) (string, bool) {
	var cands []string
	if arg != "" {
		cands = append(cands, arg)
	}
	cands = append(cands, ".")
	if exe, err := os.Executable(); err == nil {
		cands = append(cands, filepath.Dir(exe))
	}
	cands = append(cands,
		filepath.Join("..", "extracted", "ROBINSON_ISO", "ROBINSON"),
		filepath.Join("extracted", "ROBINSON_ISO", "ROBINSON"))
	for _, c := range cands {
		if isGameDir(c) {
			return c, true
		}
	}
	return "", false
}
