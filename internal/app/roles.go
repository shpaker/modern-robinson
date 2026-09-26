package app

import (
	"io/fs"
	"os"
	"path/filepath"
)

// RolesDir is the folder of the MCP roles' texts beside the game.
const RolesDir = "roles"

// Roles are the folders of role texts beside the game, those that are there:
// the game folder's first, then the one beside the binary, as with
// config.yml. The MCP server lays them over the texts built into it.
func Roles(root string) []fs.FS {
	cands := []string{filepath.Join(root, RolesDir)}
	if exe, err := os.Executable(); err == nil {
		cands = append(cands, filepath.Join(filepath.Dir(exe), RolesDir))
	}
	var out []fs.FS
	seen := map[string]bool{}
	for _, c := range cands {
		abs, err := filepath.Abs(c)
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		if st, err := os.Stat(abs); err == nil && st.IsDir() {
			out = append(out, os.DirFS(abs))
		}
	}
	return out
}
