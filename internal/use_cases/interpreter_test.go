package use_cases_test

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

func cmd(kw string, args ...string) types.Command {
	return types.Command{Kw: kw, Args: args}
}

// kws extracts the keywords of the surviving commands, for order-sensitive
// comparison.
func kws(cs []types.Command) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Kw
	}
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len: got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

// execOut runs a batch and keeps only the commands the caller must enact; the
// suspended tail is exercised separately in TestExecStopsAtGoSceneAndStartGame.
func execOut(
	in use_cases.Interpreter, cmds []types.Command, st *types.GameState,
) []types.Command {
	out, _ := in.Exec(cmds, st)
	return out
}

func TestExecStateMutations(t *testing.T) {
	st := types.NewGameState()
	var in use_cases.Interpreter
	out := execOut(in, []types.Command{
		cmd("SetVar", "Findaxe", "1"),
		cmd("AddVar", "coins", "3"),
		cmd("AddVar", "coins", "2"),
		cmd("AddItem", "axe"),
		cmd("AddItem", "Roby", "rope"),
		cmd("SetActive", "hand"),
		cmd("Sound", "rr093.wav", "1"), // world command survives
	}, st)

	if st.Var("findaxe") != 1 {
		t.Fatalf("Findaxe=%d", st.Var("findaxe"))
	}
	if st.Var("coins") != 5 {
		t.Fatalf("coins=%d", st.Var("coins"))
	}
	if !st.HasItem("axe") || !st.HasItem("rope") {
		t.Fatalf("inventory=%v", st.Inventory)
	}
	if st.Active != "hand" {
		t.Fatalf("active=%q", st.Active)
	}
	eq(t, kws(out), []string{"Sound"})
}

func TestExecIfTrueFalse(t *testing.T) {
	st := types.NewGameState()
	st.SetVar("open", 1)
	var in use_cases.Interpreter

	// True branch: body runs.
	out := execOut(in, []types.Command{
		cmd("If", "open", "1"),
		cmd("Sound", "a.wav", "1"),
		cmd("EndIf"),
	}, st)
	eq(t, kws(out), []string{"Sound"})

	// False branch: body skipped.
	out = execOut(in, []types.Command{
		cmd("If", "open", "0"),
		cmd("Sound", "a.wav", "1"),
		cmd("EndIf"),
		cmd("Text", "1000", "1"),
	}, st)
	eq(t, kws(out), []string{"Text"})
}

func TestExecNestedIfIsAnd(t *testing.T) {
	st := types.NewGameState()
	st.SetVar("a", 1)
	st.SetVar("b", 1)
	var in use_cases.Interpreter

	prog := func() []types.Command {
		return []types.Command{
			cmd("If", "a", "1"),
			cmd("Sound", "outer.wav", "1"),
			cmd("If", "b", "1"),
			cmd("Sound", "inner.wav", "1"),
			cmd("EndIf"),
			cmd("Text", "after-inner", "1"),
			cmd("EndIf"),
			cmd("Sound", "always.wav", "1"),
		}
	}

	// Both true: everything runs.
	eq(t, kws(execOut(in, prog(), st)),
		[]string{"Sound", "Sound", "Text", "Sound"})

	// Inner false: outer body runs, inner skipped; outer resumes after inner.
	st.SetVar("b", 0)
	eq(t, kws(execOut(in, prog(), st)),
		[]string{"Sound", "Text", "Sound"})

	// Outer false: whole nested block skipped, only the trailing command runs.
	st.SetVar("a", 0)
	st.SetVar("b", 1)
	out := execOut(in, prog(), st)
	eq(t, kws(out), []string{"Sound"})
}

func TestExecCharVarCondition(t *testing.T) {
	st := types.NewGameState()
	st.SetCharVar("greet", "hello2")
	var in use_cases.Interpreter
	out := execOut(in, []types.Command{
		cmd("If", "greet", "hello2"),
		cmd("Text", "match", "1"),
		cmd("EndIf"),
		cmd("If", "greet", "bye"),
		cmd("Text", "nomatch", "1"),
		cmd("EndIf"),
	}, st)
	eq(t, kws(out), []string{"Text"})
	if out[0].Args[0] != "match" {
		t.Fatalf("wrong branch: %v", out[0].Args)
	}
}

func TestExecDeleteItem(t *testing.T) {
	st := types.NewGameState()
	st.AddItem("axe")
	st.AddItem("rope")
	var in use_cases.Interpreter
	execOut(in, []types.Command{cmd("DeleteItem", "axe")}, st)
	if st.HasItem("axe") {
		t.Fatal("axe should be gone")
	}
	if !st.HasItem("rope") {
		t.Fatal("rope should remain")
	}
}

func TestExecUnbalancedTolerated(t *testing.T) {
	st := types.NewGameState()
	st.SetVar("x", 0)
	var in use_cases.Interpreter
	// Missing EndIf: false block simply skips to the end, no panic.
	out := execOut(in, []types.Command{
		cmd("If", "x", "1"),
		cmd("Sound", "skipped.wav", "1"),
	}, st)
	eq(t, kws(out), nil)
	// Stray EndIf: ignored.
	out = execOut(in, []types.Command{
		cmd("EndIf"),
		cmd("Sound", "kept.wav", "1"),
	}, st)
	eq(t, kws(out), []string{"Sound"})
}

// The shipped scripts put a StartGame and the If branches that test its result
// on one frame, and the map-travel scripts put several GoScene on one frame with
// the unconditional default last. Both have to stop the batch: the branch cannot
// be judged before the minigame writes its result, and a later GoScene must not
// overwrite the one that already fired.
func TestExecStopsAtGoSceneAndStartGame(t *testing.T) {
	st := types.NewGameState()
	var in use_cases.Interpreter

	// GoScene ends the script: the trailing default never reaches the caller.
	out, rest := in.Exec([]types.Command{
		cmd("SetVar", "InScene4", "1"),
		cmd("If", "InScene4", "1"),
		cmd("GoScene", "SCENA4", "Roby", "Roret4", "1", "0"),
		cmd("EndIf"),
		cmd("GoScene", "SCENA4", "Roby", "Roin4", "0", "0"),
	}, st)
	if len(out) != 1 || out[0].Args[2] != "Roret4" {
		t.Fatalf("first GoScene must win, got %v", out)
	}
	if rest != nil {
		t.Fatalf("GoScene ends the script, rest = %v", rest)
	}

	// StartGame suspends: the branches come back untouched so they can be
	// re-judged once the result is in.
	st = types.NewGameState()
	out, rest = in.Exec([]types.Command{
		cmd("StartGame", "1", "House", "Find7"),
		cmd("If", "House", "1"),
		cmd("CreateObject", "SCENA5", "home"),
		cmd("EndIf"),
		cmd("If", "House", "0"),
		cmd("GoScene", "SCENA5", "Roby", "nohome", "4", "2"),
		cmd("EndIf"),
	}, st)
	if len(out) != 1 || out[0].Kw != "StartGame" {
		t.Fatalf("StartGame must be the only command enacted, got %v", out)
	}
	if len(rest) != 6 {
		t.Fatalf("the tail must survive for the resume, got %v", rest)
	}
	// Resuming with the puzzle solved takes the success branch.
	st.SetVar("House", 1)
	out, _ = in.Exec(rest, st)
	if len(out) != 1 || out[0].Kw != "CreateObject" {
		t.Fatalf("solved resume must build the hut, got %v", out)
	}
	// Resuming after giving up takes the failure branch instead.
	st.SetVar("House", 0)
	out, _ = in.Exec(rest, st)
	if len(out) != 1 || out[0].Kw != "GoScene" {
		t.Fatalf("failed resume must take the nohome branch, got %v", out)
	}
}
