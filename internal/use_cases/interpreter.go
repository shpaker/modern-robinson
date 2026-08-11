package use_cases

import (
	"strconv"
	"strings"

	"github.com/shpaker/modern-robinson/internal/types"
)

// Interpreter executes a frame's event commands against the quest GameState.
// It evaluates If/EndIf conditionals, applies pure state mutations
// (SetVar/AddVar/SetCharVar/AddItem/DeleteItem/SetActive), and returns the
// world- and presentation-affecting commands that survived the conditionals,
// for the caller to enact against the scene and engine. It is stateless: all
// mutable data lives in the passed *GameState.
type Interpreter struct{}

// Exec runs cmds against st and returns the commands the caller must enact
// (Sound, Text, CreateObject, DelObject, GoScene, HideChar/ShowChar, SetVert,
// SetRest, StartGame, Shift, Set, Aproach, and UI toggles). Conditionals and
// state commands are consumed here and never returned.
//
// If/EndIf nest as logical AND: a command runs only when every enclosing If is
// true. Unbalanced blocks (rare author edits) are tolerated — a missing EndIf
// simply ends with the run, a stray EndIf is ignored.
func (Interpreter) Exec(cmds []types.Command, st *types.GameState) []types.Command {
	var out []types.Command
	depth := 0  // current If nesting depth
	skipAt := 0 // depth at which skipping began (0 = not skipping)
	for _, c := range cmds {
		switch strings.ToLower(c.Kw) {
		case "if":
			depth++
			if skipAt == 0 && !condTrue(c.Args, st) {
				skipAt = depth // this block is false: skip until its EndIf
			}
		case "endif", "end_if":
			if skipAt == depth {
				skipAt = 0 // the failed block closes: resume
			}
			if depth > 0 {
				depth--
			}
		default:
			if skipAt != 0 {
				continue // inside a false block
			}
			if !applyState(c, st) {
				out = append(out, c) // world/presentation command
			}
		}
	}
	return out
}

// condTrue evaluates an If condition. Form: If var,value. Numeric value compares
// the flag Var; a non-numeric value compares the string CharVar.
func condTrue(args []string, st *types.GameState) bool {
	if len(args) < 2 {
		return false
	}
	name, want := args[0], args[1]
	if n, err := strconv.Atoi(want); err == nil {
		return st.Var(name) == n
	}
	return strings.EqualFold(st.CharVar(name), want)
}

// applyState applies a pure state-mutating command and reports whether it
// consumed it. Commands it does not recognise return false and flow on to the
// caller as world/presentation effects.
func applyState(c types.Command, st *types.GameState) bool {
	a := c.Args
	switch strings.ToLower(c.Kw) {
	case "setvar":
		if len(a) >= 2 {
			st.SetVar(a[0], atoi(a[1]))
		}
	case "addvar":
		if len(a) >= 2 {
			st.AddVar(a[0], atoi(a[1]))
		}
	case "setcharvar":
		if len(a) >= 2 {
			st.SetCharVar(a[0], a[1])
		}
	case "additem":
		// AddItem item | AddItem char,item
		if len(a) >= 1 {
			st.AddItem(a[len(a)-1])
		}
	case "deleteitem":
		if len(a) >= 1 {
			st.DelItem(a[len(a)-1])
		}
	case "setactive":
		if len(a) >= 1 {
			st.Active = a[0]
		}
	default:
		return false
	}
	return true
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
