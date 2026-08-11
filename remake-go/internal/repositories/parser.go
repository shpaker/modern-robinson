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
	"aproach": true, "approach": true, "setvert": true, "shiftscreen": true,
	"setmouse": true, "hidechar": true, "showchar": true, "map": true, "mouse": true,
	"setbar": true, "setactive": true, "end": true, "delayfactor": true,
}

var (
	wordRe    = regexp.MustCompile(`^([A-Za-z_]\w*)(.*)$`)
	intRe     = regexp.MustCompile(`-?\d+`)
	soundRe   = regexp.MustCompile(`(\w+)\s*,\s*"([^"]+)"\s*,?\s*(\d*)`)
	goSceneRe = regexp.MustCompile(`(?is)GoScene\s+(\w+)\s*,.*?,\s*(-?\d+)\s*,\s*(-?\d+)\s*;`)
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
			if m != nil && keywords[head] {
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
		case "closedvert":
			for i := 0; i+1 < len(v); i += 2 {
				sc.ClosedVert = append(sc.ClosedVert, [2]int{v[i], v[i+1]})
			}
		case "objectlist":
			name := strings.TrimSpace(strings.SplitN(st.args, ",", 2)[0])
			if name != "" && len(v) >= 2 {
				sc.Objects = append(sc.Objects,
					types.ObjectRef{Name: name, GX: v[0], GY: v[1], Flag: strings.Contains(st.args, "*")})
			}
		case "soundvariables":
			if m := soundRe.FindStringSubmatch(st.args); m != nil {
				sc.SoundVars[m[1]] = [2]string{m[2], m[3]}
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
			if len(v) >= 4 {
				ob.ActiveZone = [4]int{v[0], v[1], v[2], v[3]}
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

// ParseFrameScript parses a .FS.
func (SceneParser) ParseFrameScript(text string) *types.FrameScript {
	fs := &types.FrameScript{}
	var cur *types.Frame
	for _, st := range statements(text) {
		v := ints(st.args)
		switch st.kw {
		case "scriptname":
			fs.ScriptName = strings.TrimSpace(st.args)
		case "moviename":
			fs.MovieName = strings.Trim(strings.TrimSpace(st.args), `"`)
		case "shift":
			if len(v) >= 2 {
				fs.Shift = [2]int{v[0], v[1]}
			}
		case "totalframes":
			if len(v) > 0 {
				fs.Total = v[0]
			}
		case "frame":
			cur = &types.Frame{Index: at(v, 0), Sub: at(v, 1)}
			fs.Frames = append(fs.Frames, cur)
		case "delay":
			if cur != nil && len(v) > 0 {
				cur.Delay = v[0]
			}
		case "text":
			if cur != nil {
				cur.Texts = append(cur.Texts, [2]int{at(v, 0), at(v, 1)})
			}
		}
	}
	return fs
}

// SceneExits scans a scene container's *GOL/*GOR frame scripts for the GoScene
// targets that define its left/right neighbours.
func (SceneParser) SceneExits(c interfaces.IContainer) (left, right types.Exit) {
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
		m := goSceneRe.FindStringSubmatch(string(d))
		if m == nil {
			continue
		}
		gx, _ := strconv.Atoi(m[2])
		gy, _ := strconv.Atoi(m[3])
		ex := types.Exit{Scene: strings.ToUpper(m[1]), GX: gx, GY: gy, OK: true}
		if isL {
			left = ex
		} else {
			right = ex
		}
	}
	return left, right
}
