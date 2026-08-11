package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
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
	barBG     *ebiten.Image
	sc        *types.Scene
	sceneC    interfaces.IContainer
	grid      interfaces.IGrid
	w, h      int // scene (world) size; the viewport is ViewW x PlayH
	camX      int // horizontal scroll offset into the scene
	zper      int
	objects   map[string]*types.SceneObject
	sceneObjs []*sceneObj
	fsByName  map[string][]byte
	hotspots  []hotspot
	exitL     types.Exit
	exitR     types.Exit
	stepWav   []byte

	gs         *types.GameState
	interp     use_cases.Interpreter
	charHidden bool

	fridCell   [2]int
	fridZ      int
	fridHidden bool
	fridIdle   *adapters.Animation
	fridFrame  int
	fridT      float64

	bar        *types.Bar
	itemIcons  map[string]*ebiten.Image
	barSprites map[string]*ebiten.Image
	texts      []string
	hover      string
	invScroll  int

	act        *actionPlay
	pendingAct *actionPlay

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

	mode     int // boot sequence: logo -> title -> play
	modeT    float64
	logoImg  *ebiten.Image
	titleImg *ebiten.Image
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
		gs:        types.NewGameState(),
	}
	ebiten.SetCursorMode(ebiten.CursorModeHidden) // we draw our own cursor
	g.seedStartup()
	g.loadBar()
	// Starting inventory per ROBY.CHR (Items hand, hat).
	g.gs.AddItem("hand")
	g.gs.AddItem("hat")
	g.gs.Active = "hand"
	// STARTUP.INF marks INT0 as the start scene (the home-room cutscene that
	// chains into the island); ROBINSON_SCENE overrides for direct entry.
	start := "INT0"
	if s := os.Getenv("ROBINSON_SCENE"); s != "" {
		start = strings.ToUpper(s)
	}
	g.loadScene(start, nil, "", "")
	g.loadScreens()
	if os.Getenv("ROBINSON_SCENE") != "" {
		g.mode = modePlay // direct scene entry skips the boot screens
	}
	if v := os.Getenv("ROBINSON_VARS"); v != "" {
		// Debug/test aid: comma-separated name=value quest flags.
		for _, kv := range strings.Split(v, ",") {
			if k, val, ok := strings.Cut(kv, "="); ok {
				g.gs.SetVar(k, atoiArg(val))
			}
		}
	}
	return g
}

// seedStartup loads STARTUP.INF's initial quest variables into the game state so
// early If-checks branch correctly (most flags are 0, some are not).
func (g *Game) seedStartup() {
	sc := g.res.SceneContainer("STARTUP")
	if sc == nil {
		return
	}
	d, err := sc.ExtractName("STARTUP.INF")
	if err != nil {
		return
	}
	vars, charVars := g.parser.ParseStartup(string(d))
	for k, v := range vars {
		g.gs.SetVar(k, v)
	}
	for k, v := range charVars {
		g.gs.SetCharVar(k, v)
	}
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

func (g *Game) loadScene(name string, spawn *[2]int, entry, entryFrid string) {
	if g.res.SceneContainer(name) == nil {
		// Unknown scene target (bad parse or missing container): stay put.
		g.msg, g.msgT = "?? "+name, 3
		return
	}
	bg, pal, _ := g.res.SceneBackground(name)
	g.sceneName = name
	if bg != nil {
		g.bg = bgImage(bg, pal)
	} else {
		g.bg = ebiten.NewImage(ViewW, PlayH) // interiors/cutscenes without a .DAT
	}
	c := g.res.SceneContainer(name)
	g.sceneC = c
	g.act, g.pendingAct = nil, nil
	scnData, _ := c.ExtractName(name + ".SCN")
	g.sc = g.parser.ParseScene(string(scnData))
	if g.sc.Size == [2]int{0, 0} {
		if bg != nil {
			g.sc.Size = [2]int{bg.Width, bg.Height}
		} else {
			g.sc.Size = [2]int{ViewW, PlayH}
		}
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
				// Key by file base name: ObjectList references the file, and
				// the inner ObjectName may differ (GORGHT.OB is "gobanan").
				key := strings.ToLower(e.Name[:len(e.Name)-3])
				g.objects[key] = ob
				if inner := strings.ToLower(ob.Name); inner != key {
					g.objects[inner] = ob
				}
			}
		}
	}
	g.charHidden = false
	g.fsByName = fonScripts(c)
	g.sceneObjs = loadSceneObjects(g.res, g.parser, g.sc, g.objects,
		g.fsByName, func(obj string) bool { return g.gs.IsGone(name, obj) })
	for _, sp := range g.gs.Spawns(name) {
		g.spawnObject(sp.Obj, sp.GX, sp.GY)
	}
	g.buildHotspots()
	g.exitL, g.exitR = g.parser.SceneExits(c)

	g.idle = adapters.LoadAnimation(g.res, "Roby1.mv")
	g.walkCache = map[int]*adapters.Animation{}
	start := g.spawnCell(spawn)
	g.cell = start
	px, py := g.grid.ToScreen(start[0], start[1])
	g.pos = [2]float64{float64(px), float64(py)}
	g.clampCamera()
	g.path = nil
	g.frameI = 0

	g.stepWav = nil
	if sv, ok := g.sc.SoundVars["step"]; ok {
		g.stepWav = g.res.Sound(sv[0])
	}
	g.fridInit()
	if entryFrid != "" {
		g.runFridEntry(entryFrid)
	}
	if entry != "" {
		g.startEntry(entry)
	}
}

// buildHotspots places click zones: rect = ActiveZone + FonScript.Shift - GridShift.
func (g *Game) buildHotspots() {
	g.hotspots = nil
	gsx, gsy := g.sc.GridShift[0], g.sc.GridShift[1]
	for _, s := range g.sceneObjs {
		if s.removed {
			continue // taken objects are no longer clickable
		}
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
	if g.act != nil || g.pendingAct != nil {
		return // ignore input while an action is walking/playing
	}
	if my >= PlayH {
		g.clickBar(mx, my) // inventory-bar click
		return
	}
	wx, wy := mx+g.camX, my // viewport -> world
	if wx < 40 && g.exitL.OK {
		g.pending = &g.exitL
		return
	}
	if wx > g.w-40 && g.exitR.OK {
		g.pending = &g.exitR
		return
	}
	for _, hs := range g.hotspots {
		if pointIn(hs.rect, wx, wy) {
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
		if pointIn(hs.rect, wx, wy) {
			if !g.startObjectAction(hs.ob.Name) {
				// No action script: examine — say the object's name.
				if s := g.textLine(hs.ob.Text); s != "" {
					g.msg, g.msgT = s, 2.5
				}
			}
			return
		}
	}
	cx, cy := g.grid.ToCell(wx, wy)
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
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		g.save()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		g.load()
	}
	dt := 1.0 / float64(ebiten.TPS())
	if g.updateScreens(dt) {
		return nil // boot screens own the frame
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.click(ebiten.CursorPosition())
	}
	g.updateHover(ebiten.CursorPosition())

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

	g.clampCamera()

	// advance animated scene objects and run their frame events through the
	// interpreter (ambient loops mostly fire Sound)
	for _, s := range g.sceneObjs {
		g.applyEvents(s.update(dt))
	}
	g.updateAction(dt)
	g.updateFrid(dt)

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
		g.loadScene(p.Scene, &[2]int{p.GX, p.GY}, p.Entry, p.EntryFrid)
	}
	return nil
}

// charZCoord gives the character a mid/front sub-slot within its grid row.
const charZCoord = 7

// Draw renders one frame: the scrolled scene background, then objects and the
// character in back-to-front Z order (gy*ZPerGrid + ZCoord), then the inventory
// bar over the bottom 80px, then HUD/debug/cursor. Everything in world space is
// shifted left by camX; the bar and cursor are in viewport space.
func (g *Game) Draw(screen *ebiten.Image) {
	if g.mode != modePlay {
		g.drawScreens(screen)
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(g.camX), 0)
	screen.DrawImage(g.bg, op)

	xoff := -g.camX
	type drawable struct {
		z  int
		fn func()
	}
	items := make([]drawable, 0, len(g.sceneObjs)+2)
	for _, s := range g.sceneObjs {
		if s.visible {
			s := s
			items = append(items, drawable{s.z, func() { s.draw(screen, xoff) }})
		}
	}
	items = append(items, drawable{g.cell[1]*g.zper + charZCoord, func() { g.drawCharacter(screen) }})
	if g.fridVisible() {
		items = append(items, drawable{g.fridCell[1]*g.zper + g.fridZ, func() { g.drawFrid(screen) }})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].z < items[j].z })
	for _, it := range items {
		it.fn()
	}

	g.drawBar(screen)
	if g.debug {
		g.drawDebug(screen)
	}
	g.drawCursor(screen)
}

// cursorType returns the cursor for the hovered zone: 1..4 = arrows
// (left/right/up/down), 0 = hand (object action), -1 = default pointer.
func (g *Game) cursorType(mx, my int) int {
	if my >= PlayH {
		return -1 // bar area
	}
	wx, wy := mx+g.camX, my
	if wx < 40 && g.exitL.OK {
		return 1
	}
	if wx > g.w-40 && g.exitR.OK {
		return 2
	}
	for _, hs := range g.hotspots {
		if pointIn(hs.rect, wx, wy) {
			switch strings.ToLower(hs.key) {
			case "goleft":
				return 1
			case "gorght":
				return 2
			}
			return hs.ob.Cursor
		}
	}
	return -1
}

// drawCursor draws our own cursor (the game's are proprietary) by zone type.
func (g *Game) drawCursor(screen *ebiten.Image) {
	mx, my := ebiten.CursorPosition()
	x, y := float32(mx), float32(my)
	white := rgba(255, 255, 255, 255)
	dark := rgba(0, 0, 0, 200)
	switch g.cursorType(mx, my) {
	case 1: // ◄
		drawTriangle(screen, x-10, y, x+4, y-8, x+4, y+8, white, dark)
	case 2: // ►
		drawTriangle(screen, x+10, y, x-4, y-8, x-4, y+8, white, dark)
	case 3: // ▲
		drawTriangle(screen, x, y-10, x-8, y+4, x+8, y+4, white, dark)
	case 4: // ▼
		drawTriangle(screen, x, y+10, x-8, y-4, x+8, y-4, white, dark)
	case 0: // hand / action
		vector.FillCircle(screen, x, y, 6, white, true)
		vector.StrokeCircle(screen, x, y, 6, 1.5, dark, true)
		vector.FillCircle(screen, x, y, 2, dark, true)
	default: // pointer
		vector.StrokeCircle(screen, x, y, 5, 1.5, white, true)
		vector.FillCircle(screen, x, y, 1.5, white, true)
	}
}

func drawTriangle(dst *ebiten.Image, ax, ay, bx, by, cx, cy float32, fill, outline color.Color) {
	vector.StrokeLine(dst, ax, ay, bx, by, 3, outline, true)
	vector.StrokeLine(dst, bx, by, cx, cy, 3, outline, true)
	vector.StrokeLine(dst, cx, cy, ax, ay, 3, outline, true)
	vector.StrokeLine(dst, ax, ay, bx, by, 1.5, fill, true)
	vector.StrokeLine(dst, bx, by, cx, cy, 1.5, fill, true)
	vector.StrokeLine(dst, cx, cy, ax, ay, 1.5, fill, true)
}

func (g *Game) drawCharacter(screen *ebiten.Image) {
	if g.charHidden {
		return // HideChar: hero not on stage (cutscene / off-screen)
	}
	if g.drawAction(screen) {
		return // an action movie is playing in place of idle/walk
	}
	a := g.idle
	if g.moving {
		a = g.walkAnim(g.curDir)
	}
	if !a.OK() {
		return
	}
	fi := g.frameI % len(a.Frames)
	frame, anch := a.Frames[fi], a.Anchors[fi]
	px, py := g.pos[0]-float64(g.camX), g.pos[1]
	vector.FillCircle(screen, float32(px), float32(py-3), 16, rgba(0, 0, 0, 70), true)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(px-float64(anch[0]), py-float64(anch[1]))
	screen.DrawImage(frame, op)
}

func (g *Game) drawDebug(screen *ebiten.Image) {
	xoff := -g.camX
	nx, ny := g.grid.Dims()
	for gy := 0; gy < ny; gy++ {
		for gx := 0; gx < nx; gx++ {
			x, y := g.grid.ToScreen(gx, gy)
			x += xoff
			var col color.Color
			switch {
			case g.grid.Blocked(gx, gy):
				col = rgba(255, 60, 60, 200)
			case g.grid.Valid(gx, gy):
				col = rgba(60, 255, 120, 200)
			default:
				continue
			}
			vector.FillCircle(screen, float32(x), float32(y), 3, col, true)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d,%d", gx, gy), x+4, y-6)
		}
	}
	for _, c := range g.path {
		x, y := g.grid.ToScreen(c[0], c[1])
		vector.FillCircle(screen, float32(x+xoff), float32(y), 4, rgba(255, 230, 0, 230), true)
	}
	for _, hs := range g.hotspots {
		r := hs.rect
		vector.StrokeRect(screen, float32(r.Min.X+xoff), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()),
			1, rgba(255, 230, 0, 200), false)
		ebitenutil.DebugPrintAt(screen, hs.ob.Name, r.Min.X+xoff, r.Min.Y-12)
	}
	rx, ry := g.grid.ToScreen(g.cell[0], g.cell[1])
	vector.StrokeCircle(screen, float32(rx+xoff), float32(ry), 9, 2, rgba(0, 200, 255, 255), true)

	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(
		"DEBUG (F1)  v=%s  scene=%s  cell=%v  cam=%d  dir=%d  moving=%v  fps=%.0f",
		Version, g.sceneName, g.cell, g.camX, g.curDir, g.moving, ebiten.ActualFPS()), 8, PlayH-32)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(
		"exitL=%s(%d,%d) exitR=%s(%d,%d)  objects=%d hotspots=%d",
		g.exitL.Scene, g.exitL.GX, g.exitL.GY, g.exitR.Scene, g.exitR.GX, g.exitR.GY,
		len(g.objects), len(g.hotspots)), 8, PlayH-18)
}

// Layout is the fixed original window: a 640x400 scene viewport plus the 80px
// inventory bar. Wide scenes scroll horizontally within it.
func (g *Game) Layout(_, _ int) (int, int) {
	return ViewW, ViewH
}

// clampCamera centres the camera on the character, clamped to the scene width.
func (g *Game) clampCamera() {
	px := int(g.pos[0])
	g.camX = clampInt(px-ViewW/2, 0, maxInt(0, g.w-ViewW))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func pointIn(r image.Rectangle, x, y int) bool {
	return x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y
}

func rgba(r, g, b, a uint8) color.Color { return color.RGBA{r, g, b, a} }
