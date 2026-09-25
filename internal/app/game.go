package app

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/minigame"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

type hotspot struct {
	key  string
	ob   *types.SceneObject
	rect image.Rectangle
	z    int // its object's draw order, so the topmost zone is hit first
}

// Game is the Ebiten game: composition root wiring resources, parser, grid,
// animation, and audio into the run loop.
type Game struct {
	res    interfaces.IResources
	parser interfaces.ISceneParser
	audio  interfaces.IAudio

	sceneName string
	bg        *ebiten.Image
	barBG     *ebiten.Image
	sc        *types.Scene
	sceneC    interfaces.IContainer
	grid      interfaces.IGrid
	w, h      int     // scene (world) size; the viewport is ViewW x PlayH
	camX      int     // horizontal scroll offset into the scene (drawn)
	camXf     float64 // the same offset before rounding, so easing stays smooth
	camTarget float64 // where the camera eases to (scene+0x70 in ROBY.EXE)
	zper      int
	objects   map[string]*types.SceneObject
	sceneObjs []*sceneObj
	fsByName  map[string][]byte
	hotspots  []hotspot
	exitL     types.Exit
	exitR     types.Exit

	cursors      map[string]cursorSprite // the game's own, from ROBY.EXE
	cursorName   string                  // this tick's pick (see cursor.go)
	cursorSystem bool                    // the system arrow is showing instead

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
	fridFrame  int    // standing pose, row*3+column (fridLook)
	fridBox    [4]int // FRID.CHR LookBox: her head turns by it

	bar        *types.Bar
	itemIcons  map[string]*ebiten.Image
	barSprites map[string]*ebiten.Image
	texts      []string
	hover      string
	invScroll  int

	act *actionPlay

	// pal is the scene palette: the engine paints every sprite on stage with
	// it and never reads a movie's own .COL (docs/03-resource-types.md).
	pal types.Palette

	idle       *adapters.Animation
	standMovie string // ROBY.CHR standing loop, reloaded with each palette
	lookBox    [4]int // ROBY.CHR LookBox: the head turns by it (idle.go)
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
	debug   bool      // F1: the world as the engine sees it (debug.go)
	dbg     debugView // F2 panel, command log, STARTUP.INF GridDebug

	// the scene's ambient pool and the countdown to its next shot (ambient.go)
	ambientPool []types.SoundVar
	ambientT    float64
	randn       func(n int) int // rand.Intn, swapped out in tests

	mode        int  // logo -> title -> menu -> loading -> play
	started     bool // a run is underway: "continue" and "save" are meaningful
	modeT       float64
	logoImg     *ebiten.Image
	titleImg    *ebiten.Image
	loadingImg  *ebiten.Image
	loadingFrom string // scene the loading screen is sitting out
	quit        bool

	// options menu, save/load screens
	optSprites map[string]*ebiten.Image
	optHover   int
	optDrag    int
	btnDown    int // slot-screen button held: 0 left, 1 right, -1 none
	slotHover  int
	slotSel    int
	slotDblT   float64 // what is left of the double-click window, seconds
	slotCache  map[int]*ebiten.Image
	slotEmpty  [2]*ebiten.Image // TEMP, the empty slot: as painted, shaded
	slotDim    float64          // unchosen thumbnails' brightness; 0 = not yet
	saves      saveStore
	thumb      *ebiten.Image
	scratch    *ebiten.Image
	volSound   float64
	volMusic   float64
	speed      float64 // 0..1 game speed slider (0.5 = original pace)

	mg       minigame.Game   // the minigame currently taking over the screen
	mgVar    string          // quest variable its result goes into
	mgResume []types.Command // frame tail waiting on the minigame's result

	// scene transition fade driven by the scene's .FAD table
	fadeCurve []float64
	fadeStep  int
	fadeOut   bool
	fadeT     float64
	fadeTo    *types.Exit

	ctl     *control    // a driver playing the hero from outside (control.go)
	stopped atomic.Bool // Stop: end the loop on the next tick
}

// NewGame builds a game over the given resources with the default settings.
func NewGame(res interfaces.IResources) *Game {
	return NewGameWith(res, DefaultConfig())
}

// NewGameWith builds a game over the given resources and the player's settings.
func NewGameWith(res interfaces.IResources, cfg Config) *Game {
	return newGame(res, cfg, adapters.NewAudio(SampleRate))
}

// newGame builds a game that plays through the given audio. The process has
// room for one audio context, so tests that build several games bring their
// own.
func newGame(res interfaces.IResources, cfg Config, audio interfaces.IAudio) *Game {
	g := &Game{
		res:        res,
		parser:     repositories.SceneParser{},
		audio:      audio,
		cycleCache: map[string]*walkCycle{},
		curDir:     6,
		robyZ:      charZCoord,
		// A run opens with Friday hidden, as resetRun leaves it: INT0 hides her
		// anyway, but a direct scene entry skips INT0, and there an Aproach
		// Frid (ROHANGOL's) would walk her unseen with her steps audible.
		fridHidden: true,
		debug:      cfg.Debug,
		gs:         types.NewGameState(),
		randn:      rand.Intn,
	}
	// State commands never reach applyEffect; the trace hears them from here.
	g.interp.Observe = g.trace
	ebiten.SetCursorMode(ebiten.CursorModeHidden) // the game draws its own
	g.loadCursors()
	g.optHover, g.optDrag = -1, -1
	g.slotHover, g.slotSel, g.btnDown = -1, 0, -1
	g.slotCache = map[int]*ebiten.Image{}
	g.volSound, g.volMusic, g.speed = cfg.Sound, cfg.Music, cfg.Speed
	g.seedStartup()
	g.loadBar()
	g.loadOptions()
	g.loadCharacter()
	g.seedItems()
	// STARTUP.INF marks INT0 as the start scene (the home-room cutscene that
	// chains into the island); ROBINSON_SCENE overrides for direct entry.
	start := "INT0"
	if s := os.Getenv("ROBINSON_SCENE"); s != "" {
		start = strings.ToUpper(s)
	}
	g.loadScene(start, nil, "", "")
	g.loadScreens()
	if os.Getenv("ROBINSON_SCENE") != "" {
		// Direct scene entry skips the boot screens and counts as a run.
		g.mode, g.started = modePlay, true
	}
	if mg := os.Getenv("ROBINSON_MINIGAME"); mg != "" {
		// Debug/test aid: jump straight into a minigame by id.
		g.mode, g.started = modePlay, true
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
	if v := os.Getenv("ROBINSON_UI"); v != "" {
		// Debug/test aid: comma-separated UI toggles to switch on (map, bar).
		for _, name := range strings.Split(v, ",") {
			g.gs.UI[strings.ToLower(strings.TrimSpace(name))] = true
		}
	}
	if v := os.Getenv("ROBINSON_ITEMS"); v != "" {
		// Debug/test aid: comma-separated items to add to the inventory.
		for _, name := range strings.Split(v, ",") {
			g.gs.AddItem(strings.TrimSpace(name))
		}
	}
	return g
}

// seedStartup loads STARTUP.INF's initial quest variables into the game state so
// early If-checks branch correctly (most flags are 0, some are not), and its
// GridDebug switch.
func (g *Game) seedStartup() {
	sc := g.res.SceneContainer("STARTUP")
	if sc == nil {
		return
	}
	d, err := sc.ExtractName("STARTUP.INF")
	if err != nil {
		return
	}
	st := g.parser.ParseStartup(string(d))
	for k, v := range st.Vars {
		g.gs.SetVar(k, v)
	}
	for k, v := range st.CharVars {
		g.gs.SetCharVar(k, v)
	}
	g.dbg.grid = st.GridDebug == 1 // the engine tests for exactly 1
}

// startItems is what each character carries when his .CHR cannot be read.
var startItems = map[string][]string{
	"Roby": {"hand", "hat"},
	"Frid": {"handfr"},
}

// seedItems hands each character the Items his .CHR starts him with
// (ROBY.CHR: hand, hat; FRID.CHR: handfr) and puts the hero in control with
// his bare hand.
func (g *Game) seedItems() {
	for _, char := range []string{"Roby", "Frid"} {
		items := startItems[char]
		if c := g.res.SceneContainer(strings.ToUpper(char)); c != nil {
			d, err := c.ExtractName(strings.ToUpper(char) + ".CHR")
			if ch := g.parser.ParseChar(string(d)); err == nil &&
				len(ch.Items) > 0 {
				items = ch.Items
			}
		}
		for _, it := range items {
			g.gs.AddItemTo(char, it)
		}
	}
	g.gs.ActiveChar = "Roby"
	g.gs.Active = "hand"
	if inv := g.gs.Inventory(); len(inv) > 0 {
		g.gs.Active = inv[0]
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
	// Effects belong to the scene that started them: without this a cutscene's
	// thunder (or the rest of a skipped one) keeps playing over the next scene.
	// Music is separate — it survives on channel 0 for "Music Continue".
	g.audio.StopEffects()
	bg, pal, _ := g.res.SceneBackground(name)
	g.sceneName = name
	if bg != nil {
		g.bg = bgImage(bg, pal)
		if pal != g.pal {
			g.pal = pal
			g.repaintHeroes()
		}
	} else {
		g.bg = ebiten.NewImage(ViewW, PlayH) // interiors/cutscenes without a .DAT
	}
	c := g.res.SceneContainer(name)
	g.sceneC = c
	g.act, g.mgResume = nil, nil
	scnData, _ := c.ExtractName(name + ".SCN")
	g.sc = g.parser.ParseScene(string(scnData))
	// The engine arms the ambient timer already expired, so a scene can speak up
	// on its first tick instead of opening with silence (ambient.go).
	g.ambientPool, g.ambientT = g.sc.AmbientSounds(), 0
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
	if tracing() {
		g.traceState("enter scene (entry=%q frid=%q)", entry, entryFrid)
	}
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
	g.sceneObjs = loadSceneObjects(g.res, g.pal, g.parseFS, g.sc, g.objects,
		g.fsByName,
		func(obj string) bool { return g.gs.IsGone(name, obj) },
		func(obj string) (types.Spawn, bool) { return g.gs.SpawnAt(name, obj) })
	for _, sp := range g.gs.Spawns(name) {
		if !g.objPresent(sp.Obj) { // created, but not named by the ObjectList
			g.spawnObject(sp.Obj, sp.GX, sp.GY)
		}
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
	if spawn != nil {
		g.showCell(start[0], start[1])
	} else {
		g.centreOnHero()
	}
	g.path = nil
	g.frameI = 0

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
// world space (the camera offset is applied when the zones are tested). An
// object may own several rectangles, and each becomes its own hotspot.
//
// The engine looks the other way round from the way it draws: its search runs
// z from 40 down to 0 and, within one z, over ObjectList from the start, taking
// the first zone that contains the point. So the topmost object wins an overlap
// — MAPSCR's hut (z=21) over the parrot clearing (z=18) it sits inside — while
// two objects at the same z go to whichever was declared first, even though the
// later one is the one drawn on top. Sorting by z alone reproduces both: the
// sort is stable, so ObjectList order survives inside each z.
//
// The bounds are inclusive on all four sides, so a zone covers (w+1)x(h+1)
// pixels and the 94 objects declaring ActiveZone 0,0,0,0 — the animation
// carriers — end up owning a single pixel each rather than nothing.
func (g *Game) buildHotspots() {
	g.hotspots = nil
	for _, s := range g.sceneObjs {
		if s.removed {
			continue // taken objects are no longer clickable
		}
		cx, cy := g.grid.Corner(s.ref.GX, s.ref.GY)
		for _, az := range s.ob.ActiveZones {
			x, y := cx+az[0], cy+az[1]
			g.hotspots = append(g.hotspots, hotspot{
				key:  s.ref.Name,
				ob:   s.ob,
				rect: image.Rect(x, y, x+az[2]+1, y+az[3]+1),
				z:    s.z,
			})
		}
	}
	sort.SliceStable(g.hotspots, func(i, j int) bool {
		return g.hotspots[i].z > g.hotspots[j].z
	})
}

// hotspotAt returns the zone a world point hits, or nil — the first match in
// the engine's own search order (see buildHotspots).
func (g *Game) hotspotAt(wx, wy int) *hotspot {
	for i := range g.hotspots {
		if pointIn(g.hotspots[i].rect, wx, wy) {
			return &g.hotspots[i]
		}
	}
	return nil
}

// soundFile resolves a Sound event's name to its wav: a scene SoundVariable
// when one answers to it, the literal file otherwise.
func (g *Game) soundFile(name string) string {
	if g.sc != nil {
		if sv, ok := g.sc.SoundVars[name]; ok {
			return sv[0]
		}
	}
	if !strings.Contains(name, ".") {
		return name + ".wav"
	}
	return name
}

// playSound plays a Sound event the way the engine's handler does (0x41B4D4).
// A SoundVariables name sounds on the variable's own voices and ignores the
// channel: "Sound wave0,8" and "Sound step,1" share nothing with channels 8
// and 1. Only a quoted file takes the channel, cutting what that channel was
// playing. A third argument never trims the sound — it retimes the animation
// under it (see parseFS).
func (g *Game) playSound(args []string) {
	if len(args) == 0 {
		return
	}
	if g.sc != nil {
		if sv, ok := g.sc.SoundVars[args[0]]; ok {
			g.audio.PlayVoice(strings.ToLower(sv[0]), g.res.Sound(sv[0]),
				atoiArg(sv[1]), 1, 0)
			return
		}
	}
	ch := 1
	if len(args) > 1 {
		if v, err := strconv.Atoi(args[1]); err == nil {
			ch = v
		}
	}
	file := g.soundFile(args[0])
	g.audio.Play(file, g.res.Sound(file), ch)
}

// soundDurationMS is a voice line's length by the engine's own arithmetic:
// wav bytes minus its "40-byte header" (the engine's constant; real RIFF
// headers are 44) over 22050 Hz 16-bit mono (0x416F60).
func (g *Game) soundDurationMS(name string) (int, bool) {
	b := g.res.Sound(g.soundFile(name))
	if len(b) == 0 {
		return 0, false
	}
	return (len(b)*1000 - 40000) / 44100, true
}

// parseFS parses a frame script and fits its delays to the voice lines it
// carries, the way the engine retimes every script it loads.
func (g *Game) parseFS(raw []byte) *types.FrameScript {
	fs := g.parser.ParseFrameScript(string(raw))
	use_cases.RetimeByVoice(fs, g.soundDurationMS)
	return fs
}

func (g *Game) spawnCell(spawn *[2]int) [2]int {
	if spawn != nil {
		// A GoScene names the arrival cell, and cutscenes are drawn relative to
		// it: relocating it to a walkable neighbour moves the whole scene.
		return *spawn
	}
	cx, cy := g.grid.ToCell(int(float64(g.w)*0.35), int(float64(g.h)*0.62))
	x, y, _ := g.grid.NearestFree(cx, cy)
	return [2]int{x, y}
}

func (g *Game) click(mx, my int) {
	if g.act != nil {
		// A click during a movie can only mean "get on with it", and it works
		// even under SetMouse OFF — the engine takes the skip before it looks
		// at that flag (0x402e1c). skipCutscene itself checks Interrupt ON.
		g.idleAct = nil
		g.skipCutscene()
		return
	}
	if my >= PlayH {
		// The bar keeps living under SetMouse OFF: only LockBar gates it.
		g.clickBar(mx, my)
		return
	}
	if !g.gs.UI["mouse"] {
		return // SetMouse OFF: the scene is deaf to the mouse (0x402e86)
	}
	g.idleAct = nil         // a click on the scene cuts the idle chatter short
	wx, wy := mx+g.camX, my // viewport -> world
	if hs := g.hotspotAt(wx, wy); hs != nil {
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
		// No action script: the engine eats the click whole — no walk, no
		// reaction (its release build stubs out even the "Can't find script"
		// log). The object's name already lives on the hover caption.
		return
	}
	// Every walk the click starts aims the camera at the click, even one that
	// ends up going nowhere: WalkTo sets the target before it looks at the cell.
	g.aimCamera(wx)
	cx, cy := g.grid.ToCell(wx, wy)
	if g.startSelfAction(cx, cy) {
		return // the held item was aimed at the character himself
	}
	if tx, ty, ok := g.grid.NearestFree(cx, cy); ok {
		if g.roby.walking() {
			g.rerouteRoby([2]int{tx, ty}) // under way: never cut the step
			return
		}
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
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
		g.dbg.state = !g.dbg.state
	}
	// The quick save/load keys work only in play: anywhere else a load would
	// swap the run out from under the boot screens and menus.
	if g.mode == modePlay {
		if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
			g.save()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
			g.load()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && g.mg == nil {
		g.toggleOptions() // inside a minigame Esc is the game's own quit
	}
	g.ctl.tick() // a driver's call, if one is running (control.go)
	if g.quit || g.stopped.Load() {
		return ebiten.Termination
	}
	g.updateCursor() // every tick, whoever owns the frame, as OnIdle does
	dt := 1.0 / float64(ebiten.TPS())
	if g.updateScreens(dt) {
		return nil // boot screens own the frame
	}
	if g.updateOptions(dt) {
		return nil // options / save / load own the frame
	}
	if g.updateLoading(dt) {
		return nil // the loading screen owns the frame through the handover
	}
	if g.updateMinigame(dt) {
		return nil // a minigame owns the frame
	}
	// The speed slider scales the whole simulation, like the original's
	// DelayFactor: 0.5 on the slider is the authored pace.
	dt *= 0.5 + g.speed
	// A cutscene starts under the new scene's fade-in, which owns the frame for
	// ~0.3 s. Take the skip anyway, or the first press of an impatient player
	// silently disappears. The gate excludes the fade-out half, where the scene
	// being left is already spoken for.
	if g.fadeCurve != nil && !g.fadeOut && g.fadeTo == nil &&
		g.act != nil && skipPressed() {
		g.skipCutscene()
	}
	if g.updateFade(dt) {
		return nil // a scene transition is fading
	}
	if clickedThisTick() {
		g.click(ebiten.CursorPosition())
	} else if g.act != nil && skipKeyPressed() {
		// A key means only "skip" — it carries no cursor position, so it goes
		// straight past click()'s hotspot logic. skipCutscene itself checks
		// whether the running script allows it (Interrupt ON).
		g.idleAct = nil
		g.skipCutscene()
	}
	mx, my := ebiten.CursorPosition()
	g.updateHover(mx, my)
	g.edgeScroll(mx, my, dt)

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
	g.updateAmbient(dt)
	g.updateAction(dt)
	g.fridLook(mx, my)
	g.updateIdle(dt)

	if !g.moving && (g.act == nil || g.act.frid) {
		// Standing still and out of his own scripts (the engine's +0x590 and
		// +0x238): the head follows the cursor (HEAD.MV is a pose table, not a
		// loop — see idle.go). Friday's action does not hold his head.
		g.lookAtCursor(mx, my)
	}
	if g.msgT > 0 {
		if g.msgT -= dt; g.msgT <= 0 {
			g.msg = ""
		}
	}
	g.noteStateChanges(dt)
	g.flushPending()
	return nil
}

// flushPending starts the transition a script queued this tick.
func (g *Game) flushPending() {
	if g.pending == nil {
		return
	}
	p := g.pending
	g.pending = nil
	g.startFade(p)
}

// openMenu raises the main menu with fresh hover state and slot caches. As in
// the original, whose menu is built anew each time it opens (0x405fc0), the
// slot screens start on the first slot, and their shade comes from the scene
// the menu was opened over.
func (g *Game) openMenu() {
	g.mode, g.optHover, g.optDrag = modeOptions, -1, -1
	g.slotCache = map[int]*ebiten.Image{}
	g.slotSel, g.slotDim = 0, 0
}

// toggleOptions opens the main menu from play and closes it again. Before the
// first run starts there is no play to return to, so Esc only backs the slot
// screens out into the menu.
func (g *Game) toggleOptions() {
	switch g.mode {
	case modePlay:
		g.openMenu()
	case modeOptions, modeSave, modeLoad:
		if g.started {
			g.mode = modePlay
		} else if g.mode != modeOptions {
			g.mode = modeOptions
		}
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
	g.mg, g.mgVar, g.mgResume = nil, "", nil
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
	g.seedItems()
	g.resetRun()
	g.loadScene("INT0", nil, "", "")
	g.enterLoading()
}

// charZCoord is the hero's starting sub-slot within his grid row, the value
// STARTUP.INF gives him; the walk cycles move him off it with Set Roby,Z.
const charZCoord = 7

// Draw renders one frame: the scrolled scene background, then objects and the
// character in back-to-front Z order (gy*ZPerGrid + ZCoord), then the inventory
// bar over the bottom 80px, then HUD/debug/cursor. Everything in world space is
// shifted left by camX; the bar and cursor are in viewport space.
func (g *Game) Draw(screen *ebiten.Image) {
	switch {
	case g.mode == modeLogo || g.mode == modeTitle || g.mode == modeLoading:
		g.drawScreens(screen)
	case g.mode == modeOptions || g.mode == modeSave || g.mode == modeLoad:
		g.drawOptions(screen)
	case g.mg != nil:
		g.drawMinigame(screen)
	default:
		g.drawPlay(screen, true)
		g.drawFade(screen)
	}
	g.drawCursor(screen) // whichever mode picked it (cursor.go)
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
	if g.dbg.grid {
		g.drawGridDebug(screen) // the engine's own, over the sprites
	}

	if !hud {
		return
	}
	g.drawBar(screen)
	if g.debug {
		g.drawDebug(screen)
	}
	if g.dbg.state {
		g.drawStatePanel(screen)
	}
}

// objPresent reports whether an object is currently on stage (an exit arrow,
// an object a spawn record would otherwise build twice).
func (g *Game) objPresent(name string) bool {
	for _, s := range g.sceneObjs {
		if strings.EqualFold(s.ref.Name, name) && !s.removed {
			return true
		}
	}
	return false
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
// cell anchor in screen space. The frames carry their own shadow; nothing is
// laid under the feet.
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
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(fx, fy)
	screen.DrawImage(frame, op)
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
// in the frame bitmaps, not in his cell, so the cell anchor can be a whole step
// off from where he is seen.
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

// scrollMax is the widest scroll the scene allows: its width past the viewport.
func (g *Game) scrollMax() float64 { return float64(maxInt(0, g.w-ViewW)) }

// clampScroll keeps a scroll inside the scene.
func (g *Game) clampScroll(x float64) float64 {
	return math.Min(math.Max(x, 0), g.scrollMax())
}

// aimCamera sets where the camera eases to: world x in the middle of the view.
// The engine aims it at the start of every walk (WalkTo 0x40f14e) — on the
// point clicked, or on the corner of the cell an Aproach names — and never
// follows the walker after that, so the hero may well leave the frame.
func (g *Game) aimCamera(x int) {
	g.camTarget = g.clampScroll(float64(x - ViewW/2))
}

// setCamera puts the view on scroll x at once, its target along with it:
// there is nothing to ease from on a scene entry or a load.
func (g *Game) setCamera(x int) {
	g.camTarget = g.clampScroll(float64(x))
	g.camXf = g.camTarget
	g.camX = int(math.Round(g.camXf))
}

// showCell opens the view on a cell: GoScene's closing pair names the screen
// too, and the engine puts the scene's left edge on that cell's corner
// (0x41b03d), clamped to the scene, wherever the entry script then puts the
// hero.
func (g *Game) showCell(gx, gy int) {
	x, _ := g.grid.Corner(gx, gy)
	g.setCamera(x)
}

// centreOnHero puts the view on the hero, for a scene entered without a
// GoScene naming the screen (a new game, a direct debug entry, an old save).
func (g *Game) centreOnHero() { g.setCamera(int(g.heroVisualX()) - ViewW/2) }

// shiftScreen applies "ShiftScreen dx,dy": the engine aims the camera whole
// grid cells away from where it stands now (0x415690), which the scripts use
// to pan away for a beat and then back (they always come in ±1 pairs).
// Vertical scroll never happens — no scene is taller than the viewport — so
// only the x step is honoured.
func (g *Game) shiftScreen(args []string) {
	if len(args) < 1 || g.sc == nil {
		return
	}
	step := g.sc.GridSize[0]
	if step <= 0 {
		step = 1
	}
	g.camTarget = g.clampScroll(g.camXf + float64(atoiArg(args[0])*step))
}

// Edge scrolling, ROBY.EXE 0x418d04: every engine frame the cursor resting on
// the left or right edge of the scene pushes the view this many pixels.
const (
	edgeScrollZone = 10
	edgeScrollStep = 5
)

// edgeScroll is the original's free look. With the mouse on, a cursor over the
// scene (not the bar) at x <= 10 or x >= 630 moves the view and its eased
// target together, and only while the move stays inside the scene — so the
// hero can be left off screen until the next walk aims the camera back. A
// cursor outside the window is on no edge.
func (g *Game) edgeScroll(mx, my int, dt float64) {
	if !g.gs.UI["mouse"] || mx < 0 || mx >= ViewW || my < 0 || my > g.h {
		return
	}
	step := edgeScrollStep * dt * enginePace
	switch {
	case mx <= edgeScrollZone:
		step = -step
	case mx >= ViewW-edgeScrollZone:
	default:
		return
	}
	g.panCamera(step)
}

// panCamera moves the view and its target by dx, refusing a move that would
// leave the scene, as the engine's edge scroll does.
func (g *Game) panCamera(dx float64) {
	x := g.camXf + dx
	if x < 0 || x > g.scrollMax() {
		return
	}
	g.camXf, g.camTarget = x, g.camTarget+dx
	g.camX = int(math.Round(g.camXf))
}

// followCamera eases the camera toward its target the way the engine does: each
// engine frame it closes the gap by |gap|/ScrollPar pixels, at least 1 and at
// most ScrollDesc, so a step never snaps the view. The offset is carried as a
// float and only rounded for drawing — rounding it every tick would make the
// view jitter back and forth around a moving target.
func (g *Game) followCamera(dt float64) {
	gap := g.camTarget - g.camXf
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

// applyObjectBlocking closes the cells and step fences the objects on stage
// bring along. An .OB carries both relative to its own cell: ClosedVert is how
// the crab, the crocodile and the finished hut stand in the way, ClosedDir is
// how the barrels and stone piles close the diagonals across their corners.
func (g *Game) applyObjectBlocking() {
	for _, s := range g.sceneObjs {
		if s.removed {
			continue
		}
		g.setObjectBlocking(s, false)
	}
}

// setObjectBlocking applies (or lifts) one object's ClosedVert cells and
// ClosedDir fences. Lifting mirrors the engine's Object::UnClose: the entries
// are cleared outright, not restored to the scene's own state, so a removed
// crocodile opens the ford it was lying on. The lift edits the live grid only —
// on re-entry the scene rebuilds from its .SCN and the object, marked gone,
// never applies itself.
func (g *Game) setObjectBlocking(s *sceneObj, open bool) {
	if s.ob == nil {
		return
	}
	for _, c := range s.ob.ClosedVert {
		g.grid.SetVert(s.ref.GX+c[0], s.ref.GY+c[1], open)
	}
	for _, d := range s.ob.ClosedDir {
		g.grid.SetDir(s.ref.GX+d[0], s.ref.GY+d[1], d[2], open)
	}
}
