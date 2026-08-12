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
	w, h      int     // scene (world) size; the viewport is ViewW x PlayH
	camX      int     // horizontal scroll offset into the scene (drawn)
	camXf     float64 // the same offset before rounding, so easing stays smooth
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
	fridPos    [2]float64
	fridPath   [][2]int
	fridWalk   walker
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

	idle       *adapters.Animation
	idleAct    *idlePlay
	restSlots  [3]string
	idleT      float64
	cycleCache map[string]*walkCycle
	roby       walker
	curDir     int
	robyZ      int // Set Roby,Z sub-slot; STARTUP.INF starts him at 7
	cell       [2]int
	pos        [2]float64
	path       [][2]int
	moving     bool
	frameI     int

	pending *types.Exit
	msg     string
	msgT    float64
	debug   bool

	mode     int // logo -> title -> play; Esc -> options/save/load
	modeT    float64
	logoImg  *ebiten.Image
	titleImg *ebiten.Image
	quit     bool

	// options menu, save/load screens
	optSprites map[string]*ebiten.Image
	optHover   int
	optDrag    int
	slotHover  int
	slotSel    int
	slotCache  map[int]*ebiten.Image
	slotInfo   map[int]string
	thumb      *ebiten.Image
	scratch    *ebiten.Image
	volSound   float64
	volMusic   float64
	speed      float64 // 0..1 game speed slider (0.5 = original pace)

	mg       minigame        // the minigame currently taking over the screen
	mgVar    string          // quest variable its result goes into
	mgParam  int             // paramVar value the script passed in
	mgResume []types.Command // frame tail waiting on the minigame's result

	// scene transition fade driven by the scene's .FAD table
	fadeCurve []float64
	fadeStep  int
	fadeOut   bool
	fadeT     float64
	fadeTo    *types.Exit
}

// NewGame builds a game over the given resources and starts at SCENA0.
func NewGame(res interfaces.IResources) *Game {
	g := &Game{
		res:        res,
		parser:     repositories.SceneParser{},
		audio:      adapters.NewAudio(SampleRate),
		cycleCache: map[string]*walkCycle{},
		curDir:     6,
		robyZ:      charZCoord,
		debug:      DebugFlag == "true",
		gs:         types.NewGameState(),
	}
	ebiten.SetCursorMode(ebiten.CursorModeHidden) // we draw our own cursor
	g.optHover, g.optDrag = -1, -1
	g.slotHover, g.slotSel = -1, -1
	g.slotCache = map[int]*ebiten.Image{}
	g.slotInfo = map[int]string{}
	g.volSound, g.volMusic, g.speed = 1, 0.7, 0.5
	g.seedStartup()
	g.loadBar()
	g.loadOptions()
	g.loadCharacter()
	// Starting inventory per ROBY.CHR (Items hand, hat).
	g.gs.AddItem("hand")
	g.gs.AddItem("hat")
	g.gs.Active, g.gs.ActiveChar = "hand", "Roby"
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
	if mg := os.Getenv("ROBINSON_MINIGAME"); mg != "" {
		// Debug/test aid: jump straight into a minigame by id.
		g.mode = modePlay
		g.startMinigame([]string{mg, "DebugResult", "Find6"})
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
	g.act, g.pendingAct, g.mgResume = nil, nil, nil
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
	// The intro scenes (INT0..INT4) are cutscene bridges driven by their own
	// objects (START.FS shows the note, then GoScene); the controllable hero is
	// not on stage there, so he starts hidden and a ShowChar can still reveal
	// him. Every other scene shows him by default.
	g.charHidden = isIntroScene(name)
	g.fsByName = fonScripts(c)
	g.sceneObjs = loadSceneObjects(g.res, g.parser, g.sc, g.objects,
		g.fsByName, func(obj string) bool { return g.gs.IsGone(name, obj) })
	for _, sp := range g.gs.Spawns(name) {
		g.spawnObject(sp.Obj, sp.GX, sp.GY)
	}
	g.applyObjectBlocking()
	for _, v := range g.gs.Verts(name) {
		g.grid.SetVert(v.GX, v.GY, v.Open) // replay this run's SetVert edits
	}
	g.buildHotspots()
	g.exitL, g.exitR = g.parser.SceneExits(c)

	g.roby = walker{prefix: "RG", chr: "ROBY"}
	g.fridWalk = walker{prefix: "FG", chr: "FRID"}
	g.fridPath = nil
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
	// Scene music: "continue" keeps the current track playing across scenes.
	if m := strings.ToLower(strings.TrimSpace(g.sc.Music)); m != "" &&
		m != "continue" {
		g.startMusic(m)
	}
	g.fridInit()
	if entryFrid != "" {
		g.runFridEntry(entryFrid)
	}
	if entry != "" {
		g.startEntry(entry)
	}
}

// isIntroScene reports whether a scene is one of the opening cutscene bridges
// (INT0..INT4) that carry no controllable character.
func isIntroScene(name string) bool {
	u := strings.ToUpper(name)
	return len(u) == 4 && strings.HasPrefix(u, "INT") && u[3] >= '0' &&
		u[3] <= '4'
}

// buildHotspots places click zones. The engine anchors a zone to the bare cell
// corner (not the sprite): rect = Corner(gx,gy) + ActiveZone.xy, size AZ.wh, in
// world space (the camera offset is applied when the zones are tested).
func (g *Game) buildHotspots() {
	g.hotspots = nil
	for _, s := range g.sceneObjs {
		if s.removed {
			continue // taken objects are no longer clickable
		}
		az := s.ob.ActiveZone
		cx, cy := g.grid.Corner(s.ref.GX, s.ref.GY)
		x, y := cx+az[0], cy+az[1]
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

func (g *Game) click(mx, my int) {
	g.idleAct = nil // any click cuts the idle chatter short
	if g.act != nil && g.skipCutscene() {
		return // Interrupt ON: the click fast-forwards the cutscene
	}
	if g.act != nil || g.pendingAct != nil {
		return // ignore input while an action is walking/playing
	}
	if my >= PlayH {
		g.clickBar(mx, my) // inventory-bar click
		return
	}
	wx, wy := mx+g.camX, my // viewport -> world
	for _, hs := range g.hotspots {
		if !pointIn(hs.rect, wx, wy) {
			continue
		}
		// Leaving is an action like any other: the hero walks to the edge, plays
		// his departure movie, says his line and the script's own GoScene takes
		// him across. Only fall back to a bare jump when there is no script.
		if g.startObjectAction(hs.key) {
			return
		}
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
		// No action script: examine — say the object's name.
		if s := g.textLine(hs.ob.Text); s != "" {
			g.msg, g.msgT = s, 2.5
		}
		return
	}
	// The exit zones sit off the edge of the scene, so once the view has
	// scrolled inward they are no longer clickable; clicking the very edge of
	// the viewport at the end of the scene means the same thing.
	if e := g.edgeExit(mx); e != nil {
		if g.startObjectAction(exitKey(e == &g.exitL)) {
			return
		}
		g.pending = e
		return
	}
	cx, cy := g.grid.ToCell(wx, wy)
	if tx, ty, ok := g.grid.NearestFree(cx, cy); ok {
		if p := g.grid.Path(g.cell, [2]int{tx, ty}); len(p) > 1 {
			if evs, ok := g.startWalk(&g.roby, g.cell, p[1:]); ok {
				g.path = p[1:]
				g.applyWalkEvents(evs)
			}
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
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && g.mg == nil {
		g.toggleOptions() // inside a minigame Esc is the game's own quit
	}
	if g.quit {
		return ebiten.Termination
	}
	dt := 1.0 / float64(ebiten.TPS())
	if g.updateScreens(dt) {
		return nil // boot screens own the frame
	}
	if g.updateOptions() {
		return nil // options / save / load own the frame
	}
	if g.updateMinigame(dt) {
		return nil // a minigame owns the frame
	}
	// The speed slider scales the whole simulation, like the original's
	// DelayFactor: 0.5 on the slider is the authored pace.
	dt *= 0.5 + g.speed
	if g.updateFade(dt) {
		return nil // a scene transition is fading
	}
	if clickedThisTick() {
		g.click(ebiten.CursorPosition())
	}
	g.updateHover(ebiten.CursorPosition())

	// Walking is the cycle chain playing out: the animation carries the motion
	// and its frame events move the cell (see walk.go).
	g.updateWalk(dt)
	g.moving = g.roby.walking()
	g.updateFridWalk(dt)

	g.followCamera(dt)

	// advance animated scene objects and run their frame events through the
	// interpreter (ambient loops mostly fire Sound)
	for _, s := range g.sceneObjs {
		g.applyEvents(s.update(dt))
	}
	g.updateAction(dt)
	g.updateFrid(dt)
	g.updateIdle(dt)

	if !g.moving {
		// Standing still: the head follows the cursor (HEAD.MV is a pose table,
		// not a loop — see idle.go).
		g.lookAtCursor()
	}
	if g.msgT > 0 {
		if g.msgT -= dt; g.msgT <= 0 {
			g.msg = ""
		}
	}
	if g.pending != nil {
		p := g.pending
		g.pending = nil
		g.startFade(p)
	}
	return nil
}

// toggleOptions opens the options menu from play (and closes it again).
func (g *Game) toggleOptions() {
	switch g.mode {
	case modePlay:
		g.mode, g.optHover, g.optDrag = modeOptions, -1, -1
		g.slotCache = map[int]*ebiten.Image{}
		g.slotInfo = map[int]string{}
	case modeOptions, modeSave, modeLoad:
		g.mode = modePlay
	}
}

// resetRun drops everything that belongs to the run being left behind rather
// than to the quest state: a transition already in flight, a minigame owning
// the screen and the frame waiting on it, the idle chatter, and the live
// character fields no entry script will re-place. Both starting a new game and
// loading a slot go through it, or a load lands in the old run's transition.
func (g *Game) resetRun() {
	g.fadeCurve, g.fadeTo, g.fadeOut, g.fadeStep = nil, nil, false, 0
	g.pending = nil
	g.mg, g.mgVar, g.mgParam, g.mgResume = nil, "", 0, nil
	g.idleAct, g.idleT = nil, 0
	g.robyZ = charZCoord
	g.fridHidden, g.fridCell, g.fridZ = true, [2]int{}, 7
	g.invScroll = 0
	g.loadCharacter() // SetRest edits do not outlive the run that made them
}

// restart begins a new game: fresh quest state, back to the first scene.
func (g *Game) restart() {
	g.gs = types.NewGameState()
	g.seedStartup()
	g.gs.AddItem("hand")
	g.gs.AddItem("hat")
	g.gs.Active, g.gs.ActiveChar = "hand", "Roby"
	g.resetRun()
	g.loadScene("INT0", nil, "", "")
	g.mode = modePlay
}

// charZCoord is the hero's starting sub-slot within his grid row, the value
// STARTUP.INF gives him; the walk cycles move him off it with Set Roby,Z.
const charZCoord = 7

// Draw renders one frame: the scrolled scene background, then objects and the
// character in back-to-front Z order (gy*ZPerGrid + ZCoord), then the inventory
// bar over the bottom 80px, then HUD/debug/cursor. Everything in world space is
// shifted left by camX; the bar and cursor are in viewport space.
func (g *Game) Draw(screen *ebiten.Image) {
	switch g.mode {
	case modeLogo, modeTitle:
		g.drawScreens(screen)
		return
	case modeOptions, modeSave, modeLoad:
		g.drawOptions(screen)
		return
	}
	if g.mg != nil {
		g.drawMinigame(screen)
		return
	}
	g.drawPlay(screen, true)
	g.drawFade(screen)
	g.drawCursor(screen)
}

// drawPlay paints the world (and, with hud, the bar and debug overlay) into any
// target — the screen, or an offscreen image when grabbing a save thumbnail.
func (g *Game) drawPlay(screen *ebiten.Image, hud bool) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(g.camX), 0)
	screen.DrawImage(g.bg, op)

	xoff := -g.camX
	// kind orders equal-z draws: the engine draws characters first, then
	// objects, so an object at the same z paints over the hero (docs/08).
	const kindChar, kindObj = 0, 1
	type drawable struct {
		z, kind int
		fn      func()
	}
	items := make([]drawable, 0, len(g.sceneObjs)+2)
	for _, s := range g.sceneObjs {
		if s.visible {
			s := s
			items = append(
				items,
				drawable{s.z, kindObj, func() { s.draw(screen, g.grid, xoff) }},
			)
		}
	}
	items = append(
		items,
		drawable{
			g.cell[1]*g.zper + g.robyZ,
			kindChar,
			func() { g.drawCharacter(screen) },
		},
	)
	if g.fridVisible() {
		items = append(
			items,
			drawable{
				g.fridCell[1]*g.zper + g.fridZ,
				kindChar,
				func() { g.drawFrid(screen) },
			},
		)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].z != items[j].z {
			return items[i].z < items[j].z
		}
		return items[i].kind < items[j].kind
	})
	for _, it := range items {
		it.fn()
	}

	if !hud {
		return
	}
	g.drawBar(screen)
	if g.debug {
		g.drawDebug(screen)
	}
}

// cursorType returns the cursor for the hovered zone: 1..4 = arrows
// (left/right/up/down), 0 = hand (object action), -1 = default pointer.
// edgeExit returns the scene exit reachable by clicking the viewport edge: the
// screen edge only leads out once the camera has scrolled to the matching end of
// the scene, which is how the original gates its left/right exits.
func (g *Game) edgeExit(mx int) *types.Exit {
	const margin = 40
	maxCam := maxInt(0, g.w-ViewW)
	if mx < margin && g.camX == 0 && g.exitL.OK {
		return &g.exitL
	}
	if mx > ViewW-margin && g.camX >= maxCam && g.exitR.OK {
		return &g.exitR
	}
	return nil
}

func (g *Game) cursorType(mx, my int) int {
	if my >= PlayH {
		return -1 // bar area
	}
	wx, wy := mx+g.camX, my
	if e := g.edgeExit(mx); e != nil {
		if e == &g.exitL {
			return 1
		}
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

func drawTriangle(
	dst *ebiten.Image,
	ax, ay, bx, by, cx, cy float32,
	fill, outline color.Color,
) {
	vector.StrokeLine(dst, ax, ay, bx, by, 3, outline, true)
	vector.StrokeLine(dst, bx, by, cx, cy, 3, outline, true)
	vector.StrokeLine(dst, cx, cy, ax, ay, 3, outline, true)
	vector.StrokeLine(dst, ax, ay, bx, by, 1.5, fill, true)
	vector.StrokeLine(dst, bx, by, cx, cy, 1.5, fill, true)
	vector.StrokeLine(dst, cx, cy, ax, ay, 1.5, fill, true)
}

func (g *Game) drawCharacter(screen *ebiten.Image) {
	// An action or entry movie is a cutscene and plays even when the hero is
	// off stage: the intro bridges carry no controllable hero but do run entry
	// animations (INT1.FS is the whole room-and-TV opening), and a script that
	// hides Roby while Friday acts still needs Friday's movie drawn.
	if g.drawAction(screen) {
		return // an action movie is playing in place of idle/walk
	}
	if g.charHidden {
		return // HideChar: hero not on stage
	}
	a, fidx := g.heroAnim()
	if !a.OK() {
		return
	}
	fi := fidx % len(a.Frames)
	if fi < 0 {
		fi = 0
	}
	drawAnim(screen, a, fi, g.pos[0]-float64(g.camX), g.pos[1])
}

// drawAnim blits animation frame fi so the movie canvas origin sits at
// (ox, oy) - Shift, the engine's placement (see use_cases.Grid). ox,oy is the
// cell anchor in screen space. A soft shadow is laid under the figure's feet.
func drawAnim(
	screen *ebiten.Image,
	a *adapters.Animation,
	fi int,
	ox, oy float64,
) {
	frame := a.Frames[fi]
	if frame == nil {
		return // a transparent frame
	}
	bb := a.BBox[fi]
	fx := ox - float64(a.Shift[0]) + float64(bb[0])
	fy := oy - float64(a.Shift[1]) + float64(bb[1])
	fw, fh := frame.Bounds().Dx(), frame.Bounds().Dy()
	vector.FillCircle(
		screen,
		float32(fx)+float32(fw)/2,
		float32(fy)+float32(fh)-4,
		16,
		rgba(0, 0, 0, 70),
		true,
	)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(fx, fy)
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
			ebitenutil.DebugPrintAt(
				screen,
				fmt.Sprintf("%d,%d", gx, gy),
				x+4,
				y-6,
			)
		}
	}
	for _, c := range g.path {
		x, y := g.grid.ToScreen(c[0], c[1])
		vector.FillCircle(
			screen,
			float32(x+xoff),
			float32(y),
			4,
			rgba(255, 230, 0, 230),
			true,
		)
	}
	for _, hs := range g.hotspots {
		r := hs.rect
		vector.StrokeRect(
			screen,
			float32(r.Min.X+xoff),
			float32(r.Min.Y),
			float32(r.Dx()),
			float32(r.Dy()),
			1,
			rgba(255, 230, 0, 200),
			false,
		)
		ebitenutil.DebugPrintAt(screen, hs.ob.Name, r.Min.X+xoff, r.Min.Y-12)
	}
	rx, ry := g.grid.ToScreen(g.cell[0], g.cell[1])
	vector.StrokeCircle(
		screen,
		float32(rx+xoff),
		float32(ry),
		9,
		2,
		rgba(0, 200, 255, 255),
		true,
	)

	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(
		"DEBUG (F1)  v=%s  scene=%s  cell=%v  cam=%d  moving=%v  "+
			"act=%s pend=%v cyc=%v goto=%v fps=%.0f",
		Version,
		g.sceneName,
		g.cell,
		g.camX,
		g.moving,
		g.actDebug(),
		g.pendingAct != nil,
		g.roby.cycles,
		g.pending != nil,
		ebiten.ActualFPS(),
	), 8, PlayH-32)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(
		"exitL=%s(%d,%d) exitR=%s(%d,%d)  objects=%d hotspots=%d",
		g.exitL.Scene,
		g.exitL.GX,
		g.exitL.GY,
		g.exitR.Scene,
		g.exitR.GX,
		g.exitR.GY,
		len(g.objects),
		len(g.hotspots),
	), 8, PlayH-18)
}

// Layout is the fixed original window: a 640x400 scene viewport plus the 80px
// inventory bar. Wide scenes scroll horizontally within it.
func (g *Game) Layout(_, _ int) (int, int) {
	return ViewW, ViewH
}

// heroAnim is the animation and frame the hero is drawn with right now: his
// long-idle chain, his walk cycle, or the standing pose table.
func (g *Game) heroAnim() (*adapters.Animation, int) {
	switch {
	case g.idleAct != nil:
		return g.idleAct.anim, g.idleAct.player.FrameIndex()
	case g.moving:
		if w := g.roby.anim(); w.OK() {
			return w, g.roby.frame
		}
	}
	return g.idle, g.frameI
}

// heroVisualX is the hero's drawn centre in world pixels. Walking motion lives
// in the frame bitmaps, not in his cell, so the camera has to follow this rather
// than the cell anchor or it would lurch a whole cell at every step.
func (g *Game) heroVisualX() float64 {
	x := g.pos[0]
	a, fidx := g.heroAnim()
	if !a.OK() {
		return x
	}
	fi := fidx % len(a.Frames)
	if fi < 0 {
		fi = 0
	}
	if f := a.Frames[fi]; f != nil {
		return x - float64(a.Shift[0]) + float64(a.BBox[fi][0]) +
			float64(f.Bounds().Dx())/2
	}
	return x
}

// cameraTarget is where the camera wants to be: the hero centred, clamped to
// the scene.
func (g *Game) cameraTarget() int {
	return clampInt(
		int(g.heroVisualX())-ViewW/2, 0, maxInt(0, g.w-ViewW),
	)
}

// followCamera eases the camera toward its target the way the engine does: each
// engine frame it closes the gap by |gap|/ScrollPar pixels, at least 1 and at
// most ScrollDesc, so a step never snaps the view. The offset is carried as a
// float and only rounded for drawing — rounding it every tick would make the
// view jitter back and forth around a moving target.
func (g *Game) followCamera(dt float64) {
	target := float64(g.cameraTarget())
	gap := target - g.camXf
	if gap == 0 {
		return
	}
	par, desc := g.sc.ScrollPar[0], g.sc.ScrollDesc[0]
	if par <= 0 {
		par = 15
	}
	if desc <= 0 {
		desc = 4
	}
	step := math.Abs(gap) / float64(par)
	step = math.Min(math.Max(step, 1), float64(desc))
	moved := step * dt * enginePace // authored per engine frame
	if moved > math.Abs(gap) {
		moved = math.Abs(gap)
	}
	if gap < 0 {
		moved = -moved
	}
	g.camXf += moved
	g.camX = int(math.Round(g.camXf))
}

// clampCamera puts the camera on its target at once (scene entry, teleports).
func (g *Game) clampCamera() {
	g.camX = g.cameraTarget()
	g.camXf = float64(g.camX)
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

// exitKey names the exit object for a side.
func exitKey(left bool) string {
	if left {
		return "goleft"
	}
	return "gorght"
}

// actDebug describes the running action for the debug overlay.
func (g *Game) actDebug() string {
	if g.act == nil {
		return "-"
	}
	return fmt.Sprintf("%d/%d", g.act.player.FrameIndex(), len(g.act.fs.Frames))
}

// applyObjectBlocking closes the cells the objects on stage stand in. An .OB
// carries its ClosedVert relative to its own cell, which is how the crab, the
// bridge logs and the finished hut keep the hero from walking through them.
func (g *Game) applyObjectBlocking() {
	for _, s := range g.sceneObjs {
		if s.removed || s.ob == nil {
			continue
		}
		for _, c := range s.ob.ClosedVert {
			g.grid.SetVert(s.ref.GX+c[0], s.ref.GY+c[1], false)
		}
	}
}
