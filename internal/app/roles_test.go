package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// The role texts are found in the game folder; without them there is no
// folder to lay over the built-in ones (nor beside the test binary).
func TestRolesBesideTheGame(t *testing.T) {
	dir := t.TempDir()
	if got := Roles(dir); len(got) != 0 {
		t.Fatalf("Roles = %d folders, want none", len(got))
	}
	if err := os.Mkdir(filepath.Join(dir, RolesDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, RolesDir, "robinson.md"), []byte("свой Роби"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	got := Roles(dir)
	if len(got) != 1 {
		t.Fatalf("Roles = %d folders, want one", len(got))
	}
	b, err := fs.ReadFile(got[0], "robinson.md")
	if err != nil || string(b) != "свой Роби" {
		t.Errorf("robinson.md = %q, %v", b, err)
	}
}
