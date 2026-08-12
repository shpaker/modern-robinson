package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/shpaker/modern-robinson/internal/types"
)

// Tracing writes every world-affecting script command to stderr when
// ROBINSON_TRACE is set. The quest is data, so when a step misbehaves the
// question is almost always "which script ran, and what did it decide" —
// answering that from a log beats guessing from the screen.
//
// ROBINSON_TRACE=1        every command
// ROBINSON_TRACE=goscene,setvar   only these keywords
var traceKw = parseTrace(os.Getenv("ROBINSON_TRACE"))

// parseTrace turns the variable into a keyword filter: nil means tracing is off,
// an empty (but non-nil) set means trace everything.
func parseTrace(v string) map[string]bool {
	v = strings.TrimSpace(v)
	if v == "" || v == "0" || strings.EqualFold(v, "false") {
		return nil
	}
	set := map[string]bool{}
	if v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "all") {
		return set
	}
	for _, kw := range strings.Split(v, ",") {
		if kw = strings.ToLower(strings.TrimSpace(kw)); kw != "" {
			set[kw] = true
		}
	}
	return set
}

// tracing reports whether commands should be logged at all.
func tracing() bool { return traceKw != nil }

// trace logs one command with the scene it ran in.
func (g *Game) trace(c types.Command) {
	if traceKw == nil {
		return
	}
	kw := strings.ToLower(c.Kw)
	if len(traceKw) > 0 && !traceKw[kw] {
		return
	}
	fmt.Fprintf(os.Stderr, "[%s] %s %s\n",
		g.sceneName, kw, strings.Join(c.Args, ","))
}

// traceState logs a line of the game's own making (a scene change, a minigame
// result) so the log reads as a sequence of quest steps.
func (g *Game) traceState(format string, args ...any) {
	if traceKw == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[%s] %s\n", g.sceneName,
		fmt.Sprintf(format, args...))
}
