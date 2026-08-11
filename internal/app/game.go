package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

type hotspot struct {
	key  string
	ob   *types.SceneObject
	rect image.Rectangle
}

// Game is the Ebiten game: composition root wiring resources, parser, grid,
// animation, and audio into the run loop.
type Game struct {
	res    interfaces.IResources
	parser interfaces.ISceneParser
	audio  *adapters.Audio

	sceneName string
	bg        *ebiten.Image
	sc        *types.Scene
	grid      interfaces.IGrid
	w, h      int
	zper      int
	objects   map[string]*types.SceneObject
	sceneObjs []*sceneObj
	hotspots  []hotspot
	exitL     types.Exit
	exitR     types.Exit
	stepWav   []byte

	idle      *adapters.Animation
	walkCache map[int]*adapters.Animation
	curDir    int
	cell      [2]int
	pos       [2]float64
	path      [][2]int
	moving    bool
	frameI    int
	animT     float64

	pending *types.Exit
	msg     string
	msgT    float64
	debug   bool
}

// NewGame builds a game over the given resources and starts at SCENA0.
func NewGame(res interfaces.IResources) *Game {
	g := &Game{
		res:       res,
		parser:    repositories.SceneParser{},
		audio:     adapters.NewAudio(SampleRate),
		walkCache: map[int]*adapters.Animation{},
		curDir:    6,
		debug:     DebugFlag == "true",
	}
	g.loadScene("SCENA0", nil)
	return g
}

// Size reports the current scene's logical dimensions.
func (g *Game) Size() (int, int) { return g.w, g.h }

func bgImage(n *types.NGB, pal types.Palette) *ebiten.Image {
	rgba := n.RGBA(pal)
	for i := 0; i < n.Width*n.Height; i++ {
		rgba[i*4+3] = 255 // background is opaque
	}
	img := ebiten.NewImage(n.Width, n.Height)
	img.WritePixels(rgba)
	return img
}

func (g *Game) loadScene(name string, spawn *[2]int) {
	bg, pal, _ := g.res.SceneBackground(name)
	g.sceneName = name
	g.bg = bgImage(bg, pal)
	c := g.res.SceneContainer(name)
	scnData, _ := c.ExtractName(name + ".SCN")
	g.sc = g.parser.ParseScene(string(scnData))
	if g.sc.Size == [2]int{0, 0} {
		g.sc.Size = [2]int{bg.Width, bg.Height}
	}
	g.w, g.h = g.sc.Size[0], g.sc.Size[1]
	g.zper = g.sc.ZPerGrid
	if g.zper == 0 {
		g.zper = 8
	}
	g.grid = use_cases.NewGrid(g.sc, g.w, g.h)
	g.objects = map[string]*types.SceneObject{}
	for _, e := range c.Entries() {
		if strings.HasSuffix(strings.ToUpper(e.Name), ".OB") {
			if d, err := c.Extract(e); err == nil {
				ob := g.parser.ParseObject(string(d))
				g.objects[strings.ToLower(ob.Name)] = ob
			}
		}
	}
	g.sceneObjs = loadSceneObjects(g.res, g.parser, c, g.sc, g.objects)
	g.buildHotspots()
	g.exitL, g.exitR = g.parser.SceneExits(c)

	g.idle = adapters.LoadAnimation(g.res, "Roby1.mv")
	g.walkCache = map[int]*adapters.Animation{}
	start := g.spawnCell(spawn)
	g.cell = start
	px, py := g.grid.ToScreen(start[0], start[1])
	g.pos = [2]float64{float64(px), float64(py)}
	g.path = nil
	g.frameI = 0

	g.stepWav = nil
	if sv, ok := g.sc.SoundVars["step"]; ok {
		g.stepWav = g.res.Sound(sv[0])
	}
}

// buildHotspots places click zones: rect = ActiveZone + FonScript.Shift - GridShift.
func (g *Game) buildHotspots() {
	g.hotspots = nil
	gsx, gsy := g.sc.GridShift[0], g.sc.GridShift[1]
	for _, s := range g.sceneObjs {
		az := s.ob.ActiveZone
		x, y := az[0]+s.shift[0]-gsx, az[1]+s.shift[1]-gsy
		w, h := max(az[2], 8), max(az[3], 8)
		g.hotspots = append(g.hotspots, hotspot{
			key: s.ref.Name, ob: s.ob, rect: image.Rect(x, y, x+w, y+h),
		})
	}
}

// playSound resolves a Sound event's name (scene SoundVar or file) and plays it.
func (g *Game) playSound(args []string) {
	if len(args) == 0 {
		return
	}
	name, ch := args[0], 1
	if len(args) > 1 {
		if v, err := strconv.Atoi(args[1]); err == nil {
			ch = v
		}
	}
	file := name
	if sv, ok := g.sc.SoundVars[name]; ok {
		file = sv[0]
	} else if !strings.Contains(file, ".") {
		file += ".wav"
	}
	g.audio.Play(file, g.res.Sound(file), ch)
}

func (g *Game) spawnCell(spawn *[2]int) [2]int {
	if spawn != nil {
		x, y, _ := g.grid.NearestFree(spawn[0], spawn[1])
		return [2]int{x, y}
	}
	cx, cy := g.grid.ToCell(int(float64(g.w)*0.35), int(float64(g.h)*0.62))
	x, y, _ := g.grid.NearestFree(cx, cy)
	return [2]int{x, y}
}

func (g *Game) walkAnim(dir int) *adapters.Animation {
	if a, ok := g.walkCache[dir]; ok {
		return a
	}
	a := adapters.LoadAnimation(g.res, fmt.Sprintf("Rg_%d%d.mv", dir, dir))
	if !a.OK() {
		a = g.idle
	}
	g.walkCache[dir] = a
	return a
}

func (g *Game) click(mx, my int) {
	if mx < 40 && g.exitL.OK {
		g.pending = &g.exitL
		return
	}
	if mx > g.w-40 && g.exitR.OK {
		g.pending = &g.exitR
		return
	}
	for _, hs := range g.hotspots {
		if pointIn(hs.rect, mx, my) {
			switch strings.ToLower(hs.key) {
			case "goleft":
				if g.exitL.OK {
					g.pending = &g.exitL
					return
				}
			case "gorght":
				if g.exitR.OK {
					g.pending = &g.exitR
					return
				}
			}
		}
	}
	for _, hs := range g.hotspots {
		if pointIn(hs.rect, mx, my) {
			g.msg, g.msgT = hs.ob.Name, 3
			return
		}
	}
	cx, cy := g.grid.ToCell(mx, my)
	if tx, ty, ok := g.grid.NearestFree(cx, cy); ok {
		if p := g.grid.Path(g.cell, [2]int{tx, ty}); len(p) > 1 {
			g.path = p[1:]
		}
	}
}

// Update advances one tick: input, movement, animation, scene transition.
func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.debug = !g.debug
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.click(ebiten.CursorPosition())
	}
	dt := 1.0 / float64(ebiten.TPS())

	g.moving = len(g.path) > 0
	if g.moving {
		tx, ty := g.grid.ToScreen(g.path[0][0], g.path[0][1])
		dx, dy := float64(tx)-g.pos[0], float64(ty)-g.pos[1]
		if math.Abs(dx)+math.Abs(dy) > 1 {
			g.curDir = use_cases.ScreenToNumpad(dx, dy)
		}
		dist := math.Hypot(dx, dy)
		speed := 220 * dt
		if dist <= speed || dist == 0 {
			g.pos = [2]float64{float64(tx), float64(ty)}
			g.cell = g.path[0]
			g.path = g.path[1:]
			g.audio.Play("step", g.stepWav, 1)
		} else {
			g.pos[0] += dx / dist * speed
			g.pos[1] += dy / dist * speed
		}
	}

	// advance animated scene objects and fire their frame events
	for _, s := range g.sceneObjs {
		for _, ev := range s.update(dt) {
			if ev.Kw == "sound" {
				g.playSound(ev.Args)
			}
		}
	}

	a, rate := g.idle, 0.09
	if g.moving {
		a, rate = g.walkAnim(g.curDir), 0.07
	}
	g.animT += dt
	if a.OK() && g.animT >= rate {
		g.animT = 0
		g.frameI = (g.frameI + 1) % len(a.Frames)
	}
	if g.msgT > 0 {
		if g.msgT -= dt; g.msgT <= 0 {
			g.msg = ""
		}
	}
	if g.pending != nil {
		p := g.pending
		g.pending = nil
		g.loadScene(p.Scene, &[2]int{p.GX, p.GY})
	}
	return nil
}

// charZCoord gives the character a mid/front sub-slot within its grid row.
const charZCoord = 7

// Draw renders the scene: background, then objects and the character in
// back-to-front Z order (gy*ZPerGrid + ZCoord), then HUD/debug.
func (g *Game) Draw(screen *ebiten.Image) {
	screen.DrawImage(g.bg, nil)

	type drawable struct {
		z  int
		fn func()
	}
	items := make([]drawable, 0, len(g.sceneObjs)+1)
	for _, s := range g.sceneObjs {
		if s.visible {
			s := s
			items = append(items, drawable{s.z, func() { s.draw(screen) }})
		}
	}
	items = append(items, drawable{g.cell[1]*g.zper + charZCoord, func() { g.drawCharacter(screen) }})
	sort.SliceStable(items, func(i, j int) bool { return items[i].z < items[j].z })
	for _, it := range items {
		it.fn()
	}

	if g.msg != "" {
		ebitenutil.DebugPrintAt(screen, g.msg, 12, 10)
	}
	if g.debug {
		g.drawDebug(screen)
	}
}

func (g *Game) drawCharacter(screen *ebiten.Image) {
	a := g.idle
	if g.moving {
		a = g.walkAnim(g.curDir)
	}
	if !a.OK() {
		return
	}
	fi := g.frameI % len(a.Frames)
	frame, anch := a.Frames[fi], a.Anchors[fi]
	px, py := g.pos[0], g.pos[1]
	vector.DrawFilledCircle(screen, float32(px), float32(py-3), 16, rgba(0, 0, 0, 70), true)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(px-float64(anch[0]), py-float64(anch[1]))
	screen.DrawImage(frame, op)
}

func (g *Game) drawDebug(screen *ebiten.Image) {
	nx, ny := g.grid.Dims()
	for gy := 0; gy < ny; gy++ {
		for gx := 0; gx < nx; gx++ {
			x, y := g.grid.ToScreen(gx, gy)
			var col color.Color
			switch {
			case g.grid.Blocked(gx, gy):
				col = rgba(255, 60, 60, 200)
			case g.grid.Valid(gx, gy):
				col = rgba(60, 255, 120, 200)
			default:
				continue
			}
			vector.DrawFilledCircle(screen, float32(x), float32(y), 3, col, true)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d,%d", gx, gy), x+4, y-6)
		}
	}
	for _, c := range g.path {
		x, y := g.grid.ToScreen(c[0], c[1])
		vector.DrawFilledCircle(screen, float32(x), float32(y), 4, rgba(255, 230, 0, 230), true)
	}
	for _, hs := range g.hotspots {
		r := hs.rect
		vector.StrokeRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()),
			1, rgba(255, 230, 0, 200), false)
		ebitenutil.DebugPrintAt(screen, hs.ob.Name, r.Min.X, r.Min.Y-12)
	}
	rx, ry := g.grid.ToScreen(g.cell[0], g.cell[1])
	vector.StrokeCircle(screen, float32(rx), float32(ry), 9, 2, rgba(0, 200, 255, 255), true)

	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(
		"DEBUG (F1)  v=%s  scene=%s  cell=%v  dir=%d  moving=%v  fps=%.0f",
		Version, g.sceneName, g.cell, g.curDir, g.moving, ebiten.ActualFPS()), 8, g.h-32)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(
		"exitL=%s(%d,%d) exitR=%s(%d,%d)  objects=%d hotspots=%d",
		g.exitL.Scene, g.exitL.GX, g.exitL.GY, g.exitR.Scene, g.exitR.GX, g.exitR.GY,
		len(g.objects), len(g.hotspots)), 8, g.h-18)
}

// Layout returns the current scene's logical size.
func (g *Game) Layout(_, _ int) (int, int) {
	if g.w == 0 {
		return 1024, 400
	}
	return g.w, g.h
}

func pointIn(r image.Rectangle, x, y int) bool {
	return x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y
}

func rgba(r, g, b, a uint8) color.Color { return color.RGBA{r, g, b, a} }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
