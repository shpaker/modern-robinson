//go:build !js

package app

import (
	"os"
	"path/filepath"
)

// dirSaves keeps the slots as files in the game folder, which is where the
// original put them and where players expect to find them.
type dirSaves struct{ dir string }

func (d dirSaves) Read(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(d.dir, name))
}

func (d dirSaves) Write(name string, data []byte) error {
	return os.WriteFile(filepath.Join(d.dir, name), data, 0o644)
}

// defaultSaveStore is the platform's place for saves: files beside the game
// data everywhere but the browser.
func defaultSaveStore(root string) saveStore { return dirSaves{dir: root} }
