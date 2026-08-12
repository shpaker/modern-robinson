// Command questmap reports on the quest as the game's own data describes it:
// which variables gate what, which items can be obtained, how the scenes link
// up, and where a chain dead-ends. It reads the shipped scripts and prints a
// report — nothing about the remake's own code is involved, so a finding here is
// a finding about the data (or about how we read it).
//
//	go run ./tools/questmap [game-dir]
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
)

// scenes is every script container that holds scene logic.
var scenes = []string{
	"CAB_A1", "CAB_A2", "CAB_A3", "CAB_A4", "CAB_A5",
	"CAB_B1", "CAB_B2", "CAB_B3", "CAB_B4", "CAB_B5", "CAB_B6",
	"CAB_C1", "CAB_C2", "CAB_C3", "CAB_C3A", "CAB_C4", "CAB_C5",
	"CHESS", "INT0", "INT1", "INT2", "INT3", "INT4", "MAPSCR", "PALACE",
	"SCENA0", "SCENA1", "SCENA2", "SCENA3", "SCENA4", "SCENA5", "SCENA6",
	"SCENA7", "SCENA8", "SHIP1", "SHIP2", "SHIP3",
}

// where names a script inside a scene.
type where struct{ scene, script string }

func (w where) String() string { return w.scene + "/" + w.script }

// facts is everything the scan collects.
type facts struct {
	initial   map[string]int      // STARTUP.INF starting values
	writes    map[string][]string // "var=value" -> scripts that set it
	reads     map[string][]string // "var=value" -> scripts that test it
	adds      map[string][]string // item -> scripts that grant it
	drops     map[string][]string // item -> scripts that take it away
	edges     map[string][]string // scene -> scenes it can reach
	minigames []string
	items     []string // BAR.BAR order
	starting  []string // items a character holds from the start (.CHR Items)
	scripts   []where
}

func main() {
	root := "extracted/ROBINSON_ISO/ROBINSON"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	res := repositories.NewResources(root)
	if res.SceneContainer("STARTUP") == nil {
		fmt.Fprintf(os.Stderr, "no game data under %s\n", root)
		os.Exit(1)
	}
	f := scan(res)
	report(f)
}

// scan walks every scene's scripts and records what they read and change.
func scan(res *repositories.Resources) *facts {
	p := repositories.SceneParser{}
	f := &facts{
		initial: map[string]int{},
		writes:  map[string][]string{},
		reads:   map[string][]string{},
		adds:    map[string][]string{},
		drops:   map[string][]string{},
		edges:   map[string][]string{},
	}
	if c := res.SceneContainer("STARTUP"); c != nil {
		if d, err := c.ExtractName("STARTUP.INF"); err == nil {
			vars, _ := p.ParseStartup(string(d))
			for k, v := range vars {
				f.initial[k] = v
			}
		}
	}
	if c := res.SceneContainer("BAR"); c != nil {
		if d, err := c.ExtractName("BAR.BAR"); err == nil {
			f.items = p.ParseBar(string(d)).Items
		}
	}
	// A character's own Items are in hand from the start, so no script grants
	// them; without this they read as unobtainable.
	for _, chr := range []string{"ROBY", "FRID"} {
		c := res.SceneContainer(chr)
		if c == nil {
			continue
		}
		if d, err := c.ExtractName(chr + ".CHR"); err == nil {
			f.starting = append(f.starting, p.ParseChar(string(d)).Items...)
		}
	}
	for _, sc := range scenes {
		c := res.SceneContainer(sc)
		if c == nil {
			continue
		}
		for _, e := range c.Entries() {
			if !strings.HasSuffix(strings.ToUpper(e.Name), ".FS") {
				continue
			}
			d, err := c.Extract(e)
			if err != nil {
				continue
			}
			w := where{sc, strings.ToUpper(e.Name)}
			f.scripts = append(f.scripts, w)
			collect(f, w, p.ParseFrameScript(string(d)))
		}
	}
	return f
}

// collect records one script's commands.
func collect(f *facts, w where, fs *types.FrameScript) {
	for _, fr := range fs.Frames {
		for _, ev := range fr.Events {
			a := ev.Args
			switch strings.ToLower(ev.Kw) {
			case "setvar":
				if len(a) >= 2 {
					key := lower(a[0]) + "=" + strings.TrimSpace(a[1])
					f.writes[key] = append(f.writes[key], w.String())
				}
			case "addvar":
				if len(a) >= 1 {
					key := lower(a[0]) + "=+"
					f.writes[key] = append(f.writes[key], w.String())
				}
			case "if":
				if len(a) >= 2 {
					key := lower(a[0]) + "=" + strings.TrimSpace(a[1])
					f.reads[key] = append(f.reads[key], w.String())
				}
			case "additem":
				it := lower(a[len(a)-1])
				f.adds[it] = append(f.adds[it], w.String())
			case "deleteitem":
				it := lower(a[len(a)-1])
				f.drops[it] = append(f.drops[it], w.String())
			case "startgame":
				if len(a) >= 2 {
					f.minigames = append(f.minigames, fmt.Sprintf(
						"game %s -> %s at %s", a[0], lower(a[1]), w))
					// The result variable is written by the minigame itself.
					key := lower(a[1]) + "=1"
					f.writes[key] = append(f.writes[key], w.String()+" (win)")
					f.writes[lower(a[1])+"=0"] = append(
						f.writes[lower(a[1])+"=0"], w.String()+" (give up)")
				}
			case "goscene":
				if len(a) >= 1 {
					to := strings.ToUpper(strings.TrimSpace(a[0]))
					f.edges[w.scene] = appendUnique(f.edges[w.scene], to)
				}
			}
		}
	}
}

func lower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func appendUnique(xs []string, v string) []string {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}

// report prints the findings, worst first.
func report(f *facts) {
	fmt.Printf("scanned %d scripts in %d scenes\n\n", len(f.scripts), len(scenes))

	// 1. Conditions no script and no starting value can ever satisfy.
	var stuck []string
	for key, readers := range f.reads {
		if len(f.writes[key]) > 0 {
			continue
		}
		name, val, _ := strings.Cut(key, "=")
		if v, ok := f.initial[name]; ok && fmt.Sprint(v) == val {
			continue // already true at the start
		}
		if len(f.writes[name+"=+"]) > 0 {
			continue // an AddVar can reach any value
		}
		stuck = append(stuck, fmt.Sprintf("  %-22s tested by %d script(s): %s",
			key, len(readers), strings.Join(trim(readers, 3), ", ")))
	}
	sort.Strings(stuck)
	fmt.Printf("conditions that can never become true (%d):\n", len(stuck))
	printLines(stuck)

	// 2. Items a script needs but nothing grants.
	var noItem []string
	for _, it := range f.items {
		if len(f.adds[lower(it)]) > 0 || held(f, it) {
			continue
		}
		if users := scriptsUsingItem(f, it); len(users) > 0 {
			noItem = append(noItem, fmt.Sprintf(
				"  %-10s never granted, but %d script(s) act with it: %s",
				it, len(users), strings.Join(trim(users, 3), ", ")))
		}
	}
	fmt.Printf("\nitems with scripts but no AddItem (%d):\n", len(noItem))
	printLines(noItem)

	// 3. Scene graph: what INT0 can reach, and what it cannot.
	seen := map[string]bool{}
	walk(f, "INT0", seen)
	var unreachable, traps []string
	for _, sc := range scenes {
		if !seen[sc] {
			unreachable = append(unreachable, "  "+sc)
		}
		// INT4 is the ending: the hero is home again and the game stops there.
		if len(f.edges[sc]) == 0 && sc != "INT4" {
			traps = append(traps, "  "+sc+" (no GoScene leaves it)")
		}
	}
	fmt.Printf("\nscenes unreachable from INT0 (%d):\n", len(unreachable))
	printLines(unreachable)
	fmt.Printf("\nscenes with no way out, ending aside (%d):\n", len(traps))
	printLines(traps)

	// 4. The minigames and their result variables.
	sort.Strings(f.minigames)
	fmt.Printf("\nminigames (%d):\n", len(f.minigames))
	for _, m := range f.minigames {
		fmt.Println("  " + m)
	}

	// 5. The variables that actually gate progress, with where they move.
	fmt.Println("\nquest variables that gate something:")
	var keys []string
	for key := range f.reads {
		name, _, _ := strings.Cut(key, "=")
		keys = appendUnique(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		var setters []string
		for key, ws := range f.writes {
			if n, _, _ := strings.Cut(key, "="); n == name {
				setters = append(setters, ws...)
			}
		}
		if len(setters) == 0 {
			continue
		}
		sort.Strings(setters)
		fmt.Printf("  %-16s start=%d set by %d: %s\n",
			name, f.initial[name], len(setters),
			strings.Join(trim(dedupe(setters), 2), ", "))
	}
}

// held reports whether a character starts with the item in hand.
func held(f *facts, item string) bool {
	for _, s := range f.starting {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// scriptsUsingItem finds action scripts named after an item's three letters.
func scriptsUsingItem(f *facts, item string) []string {
	tok := strings.ToUpper(item)
	if len(tok) > 3 {
		tok = tok[:3]
	}
	var out []string
	for _, w := range f.scripts {
		n := w.script
		if len(n) < 8 || (!strings.HasPrefix(n, "RO") &&
			!strings.HasPrefix(n, "FR")) {
			continue
		}
		if n[2:5] == tok {
			out = append(out, w.String())
		}
	}
	return out
}

func walk(f *facts, from string, seen map[string]bool) {
	if seen[from] {
		return
	}
	seen[from] = true
	for _, to := range f.edges[from] {
		walk(f, to, seen)
	}
}

func dedupe(xs []string) []string {
	var out []string
	for _, x := range xs {
		out = appendUnique(out, x)
	}
	return out
}

func trim(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(xs[:n:n], fmt.Sprintf("+%d more", len(xs)-n))
}

func printLines(xs []string) {
	if len(xs) == 0 {
		fmt.Println("  none")
		return
	}
	for _, x := range xs {
		fmt.Println(x)
	}
}
