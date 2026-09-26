package mcp

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// builtin are the roles the binary carries: the texts it hands out where no
// folder beside the game has its own — the binary alone is enough to play.
//
//go:embed roles/*.md
var builtin embed.FS

// instructionsName is the text every client gets as it connects: short,
// for Claude Code cuts a server's instructions at 2048 characters.
const instructionsName = "instructions"

// shared are the texts every role gets after its own: how the party plays,
// the world the game shows, the puzzles as the manual tells them.
var shared = []string{"common", "world", "puzzles"}

// roles are the texts the server hands out, each file taken from the first
// layer that has it: the folders beside the game, then the ones built in.
type roles struct{ layers []fs.FS }

// newRoles lays the given folders over the built-in texts, the first on top.
func newRoles(dirs ...fs.FS) roles {
	in, err := fs.Sub(builtin, "roles")
	if err != nil {
		panic(err) // the embedded tree is fixed at build time
	}
	return roles{layers: append(append([]fs.FS{}, dirs...), in)}
}

// file is the text of name.md from the first layer that has it.
func (r roles) file(name string) (string, error) {
	for _, l := range r.layers {
		b, err := fs.ReadFile(l, name+".md")
		if err == nil {
			return strings.TrimSpace(string(b)), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("нет файла роли %s.md", name)
}

// instructions is what the server says to every client as it connects.
func (r roles) instructions() (string, error) {
	return r.file(instructionsName)
}

// role is who's own text followed by the shared ones.
func (r roles) role(who string) (string, error) {
	names := r.names()
	if !slices.Contains(names, who) {
		return "", fmt.Errorf("нет роли %q; есть: %s",
			who, strings.Join(names, ", "))
	}
	parts := make([]string, 0, 1+len(shared))
	for _, name := range append([]string{who}, shared...) {
		t, err := r.file(name)
		if err != nil {
			return "", err
		}
		parts = append(parts, t)
	}
	return strings.Join(parts, "\n\n"), nil
}

// names are the roles there are, sorted: every text in any layer but the
// instructions and the shared ones.
func (r roles) names() []string {
	seen := map[string]bool{instructionsName: true}
	for _, s := range shared {
		seen[s] = true
	}
	var out []string
	for _, l := range r.layers {
		files, _ := fs.Glob(l, "*.md")
		for _, f := range files {
			name := strings.TrimSuffix(f, ".md")
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	slices.Sort(out)
	return out
}
