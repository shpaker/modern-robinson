package repositories

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// SceneParser turns NGI text scripts (.SCN/.OB/.FS) into Domain entities and
// reads scene transitions. It is stateless and implements interfaces.ISceneParser.
type SceneParser struct{}

var _ interfaces.ISceneParser = SceneParser{}

var keywords = map[string]bool{
	"scenename": true, "screenname": true, "barname": true, "screensize": true,
	"scrollpar": true, "scrolldesc": true, "scrolldescret": true, "lefttopgrid": true,
	"gridsize": true, "gridlength": true, "gridshift": true, "zpergrid": true,
	"closedvert": true, "closeddir": true, "objectlist": true, "soundvariables": true,
	"textvariables": true, "intvariables": true, "charvariables": true, "music": true,
	"griddebug": true, "gridtoclose": true, "objectname": true, "fonscript": true,
	"zcoord": true, "activezone": true, "cursor": true, "text": true, "mousez": true,
	"scriptname": true, "moviename": true, "shift": true, "totalframes": true,
	"frame": true, "delay": true, "sound": true, "setrest": true, "set": true,
	"setvar": true, "setcharvar": true, "if": true, "endif": true, "goscene": true,
	"additem": true, "deleteitem": true, "createobject": true, "delobject": true,
	"deleteobject": true, "addvar": true, "setmap": true, "setmusic": true,
	"charactername": true, "movetype": true, "lookbox": true,
	"scenes": true, "characters": true,
	"lockbar": true, "showcursor": true, "interrupt": true, "clearscreen": true,
	"startgame": true, "endgame": true, "shownav": true,
	"aproach": true, "approach": true, "setvert": true, "shiftscreen": true,
	"setmouse": true, "hidechar": true, "showchar": true, "map": true, "mouse": true,
	"setbar": true, "setactive": true, "end": true, "delayfactor": true,
	// BAR.BAR layout fields
	"drawbar": true, "bardata": true, "barltwh": true, "characterbox": true,
	"textbox": true, "inventoryltwh": true, "invmasklt": true, "items": true,
	"advanceditems": true, "itemwh": true, "itemsdisplayed": true,
	"leftarrowbox": true, "rightarrowbox": true, "scisorsbox": true, "savebox": true,
}

var (
	wordRe  = regexp.MustCompile(`^([A-Za-z_]\w*)(.*)$`)
	intRe   = regexp.MustCompile(`-?\d+`)
	soundRe = regexp.MustCompile(
		`(\w+)\s*,\s*"([^"]+)"\s*,?\s*(\d*)\s*,?\s*(\*?)`,
	)
	goSceneRe = regexp.MustCompile(
		`(?is)GoScene\s+(\w+)\s*,\s*\w+\s*,\s*(\w+).*?,\s*(-?\d+)\s*,\s*(-?\d+)\s*;`,
	)
)

type stmt struct {
	kw   string
	args string
}

// statements tokenizes a script line-by-line, folding list continuations under
// the preceding keyword.
func statements(text string) []stmt {
	var out []stmt
	curKw := ""
	for _, line := range strings.Split(text, "\n") {
		for _, chunk := range strings.Split(line, ";") {
			s := strings.TrimSpace(chunk)
			if s == "" {
				continue
			}
			m := wordRe.FindStringSubmatch(s)
			head := ""
			if m != nil {
				head = strings.ToLower(m[1])
			}
			// A list row names its own item first ("map,1,2,*", "sound,0,0"),
			// and those names collide with script keywords. A real keyword
			// always separates its arguments with whitespace, so a comma stuck
			// straight onto the head word marks the line as data, not a new
			// statement — without this, an object called "map" or "sound"
			// silently drops itself and every row after it.
			dataRow := m != nil && strings.HasPrefix(m[2], ",")
			if m != nil && keywords[head] && !dataRow {
				curKw = head
				out = append(out, stmt{head, strings.TrimSpace(m[2])})
			} else if curKw != "" {
				out = append(out, stmt{curKw, s})
			}
		}
	}
	return out
}

func ints(s string) []int {
	fs := intRe.FindAllString(s, -1)
	out := make([]int, 0, len(fs))
	for _, f := range fs {
		v, _ := strconv.Atoi(f)
		out = append(out, v)
	}
	return out
}

func at(v []int, i int) int {
	if i < len(v) {
		return v[i]
	}
	return 0
}

// ParseScene parses a .SCN.
func (SceneParser) ParseScene(text string) *types.Scene {
	sc := &types.Scene{SoundVars: map[string][2]string{}}
	for _, st := range statements(text) {
		v := ints(st.args)
		switch st.kw {
		case "scenename":
			sc.Name = strings.TrimSpace(st.args)
		case "screenname":
			sc.Screen = strings.TrimSpace(st.args)
		case "barname":
			sc.Bar = strings.TrimSpace(st.args)
		case "screensize":
			if len(v) >= 2 {
				sc.Size = [2]int{v[0], v[1]}
			}
		case "lefttopgrid":
			if len(v) >= 2 {
				sc.LeftTopGrid = [2]int{v[0], v[1]}
			}
		case "gridsize":
			if len(v) >= 2 {
				sc.GridSize = [2]int{v[0], v[1]}
			}
		case "gridlength":
			if len(v) >= 2 {
				sc.GridLength = [2]int{v[0], v[1]}
			}
		case "gridshift":
			if len(v) >= 2 {
				sc.GridShift = [2]int{v[0], v[1]}
			}
		case "zpergrid":
			if len(v) > 0 {
				sc.ZPerGrid = v[0]
			}
		case "scrollpar":
			if len(v) >= 2 {
				sc.ScrollPar = [2]int{v[0], v[1]}
			}
		case "scrolldesc":
			if len(v) >= 2 {
				sc.ScrollDesc = [2]int{v[0], v[1]}
			}
		case "closedvert":
			for i := 0; i+1 < len(v); i += 2 {
				sc.ClosedVert = append(sc.ClosedVert, [2]int{v[i], v[i+1]})
			}
		case "objectlist":
			// Read the row by fields, never by scanning it for numbers: half
			// the objects are named with a digit (br0, bt3, map1, iva0), and a
			// digit in the name would be taken for the cell's x.
			a := argSplit(st.args)
			if len(a) >= 3 {
				sc.Objects = append(
					sc.Objects,
					types.ObjectRef{
						Name: a[0],
						GX:   atoiSafe(a[1]),
						GY:   atoiSafe(a[2]),
						Flag: len(a) >= 4 && strings.Contains(a[3], "*"),
					},
				)
			}
		case "soundvariables":
			if m := soundRe.FindStringSubmatch(st.args); m != nil {
				sc.SoundVars[m[1]] = [2]string{m[2], m[3]}
				sc.Sounds = append(sc.Sounds, types.SoundVar{
					Name:    m[1],
					Wav:     m[2],
					Voices:  m[3],
					Ambient: m[4] == "*",
				})
			}
		case "music":
			sc.Music = strings.Trim(strings.TrimSpace(st.args), `"`)
		}
	}
	return sc
}

// ParseObject parses a .OB.
func (SceneParser) ParseObject(text string) *types.SceneObject {
	ob := &types.SceneObject{}
	for _, st := range statements(text) {
		v := ints(st.args)
		switch st.kw {
		case "objectname":
			ob.Name = strings.TrimSpace(st.args)
		case "fonscript":
			ob.FonScript = strings.TrimSpace(st.args)
		case "zcoord":
			if len(v) > 0 {
				ob.Z = v[0]
			}
		case "activezone":
			// An object may declare several hit rectangles, one per row: the
			// bridge log answers both to its trunk and to the plank on it.
			if len(v) >= 4 {
				ob.ActiveZones = append(
					ob.ActiveZones, [4]int{v[0], v[1], v[2], v[3]},
				)
			}
		case "cursor":
			if len(v) > 0 {
				ob.Cursor = v[0]
			}
		case "text":
			if len(v) > 0 {
				ob.Text = v[0]
			}
		}
	}
	return ob
}

// argSplit splits a comma-separated argument list, stripping quotes and spaces.
func argSplit(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
			p = p[1 : len(p)-1]
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ParseFrameScript parses a .FS into a header plus per-frame event commands.
func (SceneParser) ParseFrameScript(text string) *types.FrameScript {
	fs := &types.FrameScript{}
	var cur *types.Frame
	for _, st := range statements(text) {
		switch st.kw {
		case "scriptname":
			fs.ScriptName = strings.TrimSpace(st.args)
		case "moviename":
			fs.MovieName = strings.Trim(strings.TrimSpace(st.args), `"`)
		case "totalframes":
			if v := ints(st.args); len(v) > 0 {
				fs.Total = v[0]
			}
		case "frame":
			v := ints(st.args)
			cur = &types.Frame{Index: at(v, 0), Sub: at(v, 1)}
			fs.Frames = append(fs.Frames, cur)
		case "delay":
			if cur != nil {
				if v := ints(st.args); len(v) > 0 {
					cur.Delay = v[0]
				}
			}
		case "shift":
			// header Shift (before any frame) vs in-frame Shift event
			if cur == nil {
				if v := ints(st.args); len(v) >= 2 {
					fs.Shift = [2]int{v[0], v[1]}
				}
			} else {
				cur.Events = append(cur.Events, types.Command{Kw: "shift", Args: argSplit(st.args)})
			}
		case "end":
			// no-op
		default:
			if cur != nil {
				cur.Events = append(
					cur.Events,
					types.Command{Kw: st.kw, Args: argSplit(st.args)},
				)
			}
		}
	}
	return fs
}

// ParseChar parses a .CHR character definition.
func (SceneParser) ParseChar(text string) *types.Character {
	c := &types.Character{}
	idle := 0
	for _, st := range statements(text) {
		switch st.kw {
		case "charactername":
			c.Name = strings.TrimSpace(st.args)
		case "movetype":
			c.MoveType = strings.TrimSpace(st.args)
		case "fonscript":
			if v := strings.TrimSpace(st.args); v != "" && idle < len(c.Idle) {
				c.Idle[idle] = v
				idle++
			}
		case "items":
			if a := argSplit(st.args); len(a) > 0 {
				c.Items = append(c.Items, a[0])
			}
		}
	}
	return c
}

// ParseBar parses BAR.BAR, the inventory-panel layout.
func (SceneParser) ParseBar(text string) *types.Bar {
	b := &types.Bar{}
	rect4 := func(s string) [4]int {
		v := ints(s)
		var r [4]int
		for i := 0; i < 4 && i < len(v); i++ {
			r[i] = v[i]
		}
		return r
	}
	for _, st := range statements(text) {
		switch st.kw {
		case "barltwh":
			b.Rect = rect4(st.args)
		case "inventoryltwh":
			b.Inventory = rect4(st.args)
		case "itemwh":
			if v := ints(st.args); len(v) >= 2 {
				b.ItemW, b.ItemH = v[0], v[1]
			}
		case "itemsdisplayed":
			if v := ints(st.args); len(v) > 0 {
				b.ItemsShown = v[0]
			}
		case "characterbox":
			b.CharBox = rect4(st.args)
		case "textbox":
			b.TextBox = rect4(st.args)
		case "leftarrowbox":
			b.LeftArrow = rect4(st.args)
		case "rightarrowbox":
			b.RightArrow = rect4(st.args)
		case "scisorsbox":
			b.ScisorsBox = rect4(st.args)
		case "savebox":
			b.SaveBox = rect4(st.args)
		case "invmasklt":
			if v := ints(st.args); len(v) >= 2 {
				b.InvMask = [2]int{v[0], v[1]}
			}
		case "bardata":
			b.Data = strings.TrimSpace(st.args)
		case "items":
			if a := argSplit(st.args); len(a) > 0 {
				b.Items = append(b.Items, a[0])
			}
		}
	}
	return b
}

// ParseStartup reads STARTUP.INF's IntVariables and CharVariables declarations
// into the quest namespace with their initial values (most flags start 0, but a
// few — TreeIs=1, MapParts=4, Find6=30 — do not; dialogue selectors point at
// their first variant script). Seeding these is required for correct If-branching.
func (SceneParser) ParseStartup(
	text string,
) (vars map[string]int, charVars map[string]string) {
	vars = map[string]int{}
	charVars = map[string]string{}
	for _, st := range statements(text) {
		switch st.kw {
		case "intvariables":
			a := argSplit(st.args)
			if len(a) >= 2 {
				vars[strings.ToLower(a[0])] = atoiSafe(a[1])
			}
		case "charvariables":
			a := argSplit(st.args)
			if len(a) >= 2 {
				charVars[strings.ToLower(a[0])] = a[1]
			}
		}
	}
	return vars, charVars
}

func atoiSafe(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

// SceneExits scans a scene container's *GOL/*GOR frame scripts for the GoScene
// targets that define its left/right neighbours.
func (SceneParser) SceneExits(
	c interfaces.IContainer,
) (left, right types.Exit) {
	for _, e := range c.Entries() {
		up := strings.ToUpper(e.Name)
		if !strings.HasSuffix(up, ".FS") {
			continue
		}
		base := up[:len(up)-3]
		isL := strings.HasSuffix(base, "GOL")
		isR := strings.HasSuffix(base, "GOR")
		if !isL && !isR {
			continue
		}
		d, err := c.Extract(e)
		if err != nil {
			continue
		}
		// The last GoScene is the script's unconditional exit; earlier ones
		// sit inside If guards (the island-discovery Disc4/Disc6 branch) and
		// must not name the bare-jump entry.
		ms := goSceneRe.FindAllStringSubmatch(string(d), -1)
		if len(ms) == 0 {
			continue
		}
		m := ms[len(ms)-1]
		gx, _ := strconv.Atoi(m[3])
		gy, _ := strconv.Atoi(m[4])
		ex := types.Exit{
			Scene: strings.ToUpper(m[1]),
			Entry: m[2],
			GX:    gx,
			GY:    gy,
			OK:    true,
		}
		if isL {
			left = ex
		} else {
			right = ex
		}
	}
	return left, right
}
