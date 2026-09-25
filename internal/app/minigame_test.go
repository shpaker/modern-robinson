package app

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/minigame/catalog"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
)

// emptyPacks is a resource layer with no minigame assets. Embedding the
// interface satisfies the type while leaving every other method nil, so a test
// that needs more than this fails loudly instead of silently reading real files.
type emptyPacks struct{ interfaces.IResources }

func (emptyPacks) ScreenPack(string) (map[string]*types.NGB, types.Palette) {
	return map[string]*types.NGB{}, types.Palette{}
}

func (emptyPacks) ScreenPalettes(string) []types.Palette { return nil }

func (emptyPacks) ScreenText(string, string) string { return "" }

// The launcher scripts put StartGame and the branches that test its result on
// one frame, so the frame has to be suspended and finished later. This drives
// that seam: the tail must be held back while the game runs, then re-judged
// against the result the minigame wrote — the success branch is unreachable
// otherwise, which is what left the hut unbuilt however well it was solved.
func TestStartGameSuspendsAndResumesTheFrame(t *testing.T) {
	frame := []types.Command{
		{Kw: "StartGame", Args: []string{"1", "House", "Find7"}},
		{Kw: "If", Args: []string{"House", "1"}},
		{Kw: "SetVar", Args: []string{"Built", "1"}},
		{Kw: "EndIf"},
		{Kw: "If", Args: []string{"House", "0"}},
		{Kw: "SetVar", Args: []string{"GaveUp", "1"}},
		{Kw: "EndIf"},
	}

	g := &Game{
		res:    emptyPacks{},
		parser: repositories.SceneParser{},
		gs:     types.NewGameState(),
	}
	g.applyEvents(frame)

	// No assets, so the game cannot open: startMinigame lets the quest through
	// with a win and must run the held-back tail itself, since nothing else
	// will.
	if g.mg != nil {
		t.Fatal("no assets: no minigame should be running")
	}
	if g.gs.Var("Built") != 1 {
		t.Errorf("Built = %d, want the success branch to have run",
			g.gs.Var("Built"))
	}
	if g.gs.Var("GaveUp") != 0 {
		t.Errorf("GaveUp = %d, want the failure branch skipped",
			g.gs.Var("GaveUp"))
	}
	if g.mgResume != nil {
		t.Error("the suspended tail must be cleared once it has run")
	}
}

// Giving up takes the other branch, and it must be decided after the game ends
// rather than before it starts.
func TestMinigameResultChoosesTheBranch(t *testing.T) {
	for _, tc := range []struct {
		name          string
		result        int
		built, gaveUp int
	}{
		{"solved", 1, 1, 0},
		{"gave up", 0, 0, 1},
	} {
		g := &Game{
			res:    emptyPacks{},
			parser: repositories.SceneParser{},
			gs:     types.NewGameState(),
		}
		// The frame suspends here; nothing after StartGame may run yet.
		g.mgVar = "House"
		g.mgResume = []types.Command{
			{Kw: "If", Args: []string{"House", "1"}},
			{Kw: "SetVar", Args: []string{"Built", "1"}},
			{Kw: "EndIf"},
			{Kw: "If", Args: []string{"House", "0"}},
			{Kw: "SetVar", Args: []string{"GaveUp", "1"}},
			{Kw: "EndIf"},
		}
		g.mg = stubGame{result: tc.result}

		if !g.updateMinigame(0.1) {
			t.Fatalf("%s: the minigame must own the frame", tc.name)
		}
		if g.mg != nil {
			t.Fatalf("%s: a finished minigame must be cleared", tc.name)
		}
		if got := g.gs.Var("Built"); got != tc.built {
			t.Errorf("%s: Built = %d, want %d", tc.name, got, tc.built)
		}
		if got := g.gs.Var("GaveUp"); got != tc.gaveUp {
			t.Errorf("%s: GaveUp = %d, want %d", tc.name, got, tc.gaveUp)
		}
	}
}

// Every game of the catalog, missing its pack, has to say so with a plain nil:
// a nil pointer in the interface would pass for a running game, and the quest
// would wait on a minigame that never draws.
func TestEveryMinigameWithoutAssetsLetsTheQuestThrough(t *testing.T) {
	for id := range catalog.Games {
		g := &Game{
			res:    emptyPacks{},
			parser: repositories.SceneParser{},
			gs:     types.NewGameState(),
		}
		g.startMinigame([]string{strconv.Itoa(id), "Result", "Param"})
		if g.mg != nil {
			t.Errorf("game %d: running without its assets", id)
		}
		if g.gs.Var("Result") != 1 {
			t.Errorf("game %d: Result = %d, want the quest let through", id,
				g.gs.Var("Result"))
		}
	}
}

// stubGame finishes on its first update with a fixed result.
type stubGame struct{ result int }

func (s stubGame) Update(float64) (bool, int) { return true, s.result }

func (stubGame) Draw(*ebiten.Image) {}

// Every sound a minigame plays reaches a driver as a word, never as its file,
// so the table has to know them all: each PlaySound of the games, read from
// their source, is in it, and nothing in it is a sound no game plays. Only the
// organ picks its notes at run time, from the files its source names; they
// are heard as notes.
func TestEveryPuzzleSoundIsHeardAsAWord(t *testing.T) {
	played := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(filepath.Join("..", "minigame"),
		func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") ||
				strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && f.Name.Name == "pipe" &&
					lit.Kind == token.STRING {
					if file, _ := strconv.Unquote(lit.Value); strings.HasSuffix(
						file, ".wav") {
						played[strings.ToLower(file)] = true
					}
				}
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "PlaySound" {
					return true
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					if f.Name.Name != "pipe" {
						t.Errorf("%s: a sound named at run time",
							fset.Position(call.Pos()))
					}
					return true
				}
				file, _ := strconv.Unquote(lit.Value)
				played[strings.ToLower(file)] = true
				return true
			})
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(played) == 0 {
		t.Fatal("no PlaySound found in the minigames")
	}
	for file := range played {
		if _, ok := puzzleSounds[file]; !ok {
			t.Errorf("%s: no word for it", file)
		}
	}
	for file, word := range puzzleSounds {
		if !played[file] {
			t.Errorf("%s: no game plays it", file)
		}
		if !plainWord(word) && !heardNotes(word) {
			t.Errorf("%s: %q is no word the ear would use", file, word)
		}
	}
	if got := soundLabel("h_take.WAV"); got != "взял" {
		t.Errorf("the table is read case aside: %q", got)
	}
	if got := soundLabel("unknown.wav"); got != "звук" {
		t.Errorf("a sound the table misses is heard as %q", got)
	}
}

// The organ is heard by ear, as a player with perfect pitch would hear it:
// each tube's note by its name, the dud by how it sounds, and Friday's aria
// as one sound — the notes he whistles in order, to be laid against the
// organ's phrase. The two tubes the engine lets swap, PIPE02 and PIPE08, are
// one note.
func TestTheOrganIsHeardByItsNotes(t *testing.T) {
	for i := range 9 {
		file := fmt.Sprintf("PIPE%02d.WAV", i)
		got := soundLabel(file)
		switch {
		case i == 0 && (!plainWord(got) || noteName.MatchString(got)):
			t.Errorf("%s: the dud heard as %q, want how it sounds", file, got)
		case i > 0 && !noteName.MatchString(got):
			t.Errorf("%s: heard as %q, want a note", file, got)
		}
	}
	if a, b := soundLabel("pipe02.wav"), soundLabel("pipe08.wav"); a != b {
		t.Errorf("the tubes that swap sound %q and %q", a, b)
	}
	if len(ariaNotes) == 0 {
		t.Fatal("the aria has no notes")
	}
	for _, n := range ariaNotes {
		if !noteName.MatchString(n) {
			t.Errorf("the aria: %q is no note", n)
		}
	}
	want := "ария: " + strings.Join(ariaNotes, " ")
	if got := soundLabel("MELODY.WAV"); got != want || !heardNotes(got) {
		t.Errorf("the aria is heard as %q, want %q", got, want)
	}
}

// noteName is a note as the ear names it: its Russian name, a sharp, and the
// octave numbered as in scientific pitch notation (до4 is middle C).
var noteName = regexp.MustCompile(`^(до|ре|ми|фа|соль|ля|си)(-диез)?[0-9]$`)

// heardNotes reports notes named one after another, led by a plain word and
// a colon when they are one sound, as Friday's aria is.
func heardNotes(s string) bool {
	if head, tail, ok := strings.Cut(s, ": "); ok {
		if !plainWord(head) {
			return false
		}
		s = tail
	}
	notes := strings.Split(s, " ")
	for _, n := range notes {
		if !noteName.MatchString(n) {
			return false
		}
	}
	return len(notes) > 0
}

// plainWord reports a word in Russian letters and spaces, nothing a file
// name would carry.
func plainWord(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	for _, r := range s {
		if r != ' ' && !unicode.Is(unicode.Cyrillic, r) {
			return false
		}
	}
	return true
}
