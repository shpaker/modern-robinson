package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// The debug layers. F1 draws the scene as the engine sees it: the walk lattice
// a click picks its cell from, where the hero stands in each cell, the fences,
// the routes, and what a click under the cursor would do. F2 is the quest
// state with the last script commands. The original engine has a single
// switch of its own, STARTUP.INF's GridDebug, and all it draws is the bare
// lattice (drawGridDebug).

// debugView is what the debug layers keep between frames.
type debugView struct {
	state bool // F2: the quest-state panel
	grid  bool // STARTUP.INF GridDebug 1: the engine's own lattice

	log []debugLine // the last script commands, oldest first

	// the quest state the change highlight compares against
	seen  *types.GameState
	vars  map[string]int
	chars map[string]string
	flash map[string]float64 // "v:"/"c:" + name -> seconds left lit

	scripts      map[string]string // hover script lookups, for one scene
	scriptsScene string
}

// debugLine is one command log entry; n counts identical repeats in a row, so
// an ambient loop does not push everything else out.
type debugLine struct {
	text string
	n    int
}

const (
	debugLogLines = 10  // the command log's length
	debugFlash    = 3.0 // seconds a changed variable stays lit
	debugLineH    = 14  // text line step of the debug font
	debugCharW    = 6   // glyph width of the debug font
)

// drawGridDebug is the engine's own GridDebug (0x4145c7): the bare walk
// lattice, GridLength+1 lines each way from LeftTopGrid one GridSize apart,
// drawn with vrtLine in colour 12 of the scene palette over the sprites.
func (g *Game) drawGridDebug(screen *ebiten.Image) {
	c := g.pal[12]
	g.drawLattice(screen, color.RGBA{c[0], c[1], c[2], 255})
}

// drawLattice draws the cell borders of the walk grid. They are the borders
// of the click: ToCell inverts Corner, so a click anywhere inside a cell's
// rectangle picks that cell.
func (g *Game) drawLattice(dst *ebiten.Image, col color.Color) {
	nx, ny := g.grid.Dims()
	x0, y0 := g.grid.Corner(0, 0)
	x1, y1 := g.grid.Corner(nx, ny)
	ox := float32(-g.camX) + 0.5 // pixel centres, for a crisp 1px line
	for i := 0; i <= ny; i++ {
		_, y := g.grid.Corner(0, i)
		fy := float32(y) + 0.5
		vector.StrokeLine(
			dst, float32(x0)+ox, fy, float32(x1)+ox, fy, 1, col, false,
		)
	}
	for i := 0; i <= nx; i++ {
		x, _ := g.grid.Corner(i, 0)
		fx := float32(x) + ox
		vector.StrokeLine(
			dst, fx, float32(y0)+0.5, fx, float32(y1)+0.5, 1, col, false,
		)
	}
}

// drawDebug is the F1 layer. The world part is clipped to the play area so it
// never spills over the bar.
func (g *Game) drawDebug(screen *ebiten.Image) {
	world := screen.SubImage(image.Rect(0, 0, ViewW, PlayH)).(*ebiten.Image)
	pv := g.previewClick(ebiten.CursorPosition())
	g.drawCells(world)
	g.drawFences(world)
	g.drawZones(world, pv.hot)
	g.drawPreview(world, pv)
	g.drawHeroes(world)
	if !g.dbg.state {
		g.drawHUD(world, pv) // the F2 panel covers the top of the screen
	}
}

// cellSize is the grid step in pixels.
func (g *Game) cellSize() (int, int) {
	x0, y0 := g.grid.Corner(0, 0)
	x1, y1 := g.grid.Corner(1, 1)
	return x1 - x0, y1 - y0
}

// footOffset is the bottom centre of frame fi's opaque box relative to the
// anchor it is drawn at (see drawAnim): where the character's feet are.
func footOffset(a *adapters.Animation, fi int) (int, int, bool) {
	if !a.OK() {
		return 0, 0, false
	}
	fi %= len(a.Frames)
	if fi < 0 {
		fi = 0
	}
	f := a.Frames[fi]
	if f == nil {
		return 0, 0, false
	}
	b := f.Bounds()
	return a.BBox[fi][0] - a.Shift[0] + b.Dx()/2,
		a.BBox[fi][1] - a.Shift[1] + b.Dy(), true
}

// standPoint is where a standing hero's feet land in a cell, in viewport
// space: his standing pose drawn at the cell anchor, bottom centre. The anchor
// itself is the top-left of his head — GridShift lifts it about a hundred
// pixels above the cell — so a grid drawn at anchors hangs over the scenery
// while the feet stay inside the cell the click picked.
func (g *Game) standPoint(gx, gy int) (float32, float32) {
	return g.standPointOf(g.idle, gx, gy)
}

// standPointOf is standPoint for a character standing in pose a.
func (g *Game) standPointOf(
	a *adapters.Animation, gx, gy int,
) (float32, float32) {
	if fx, fy, ok := footOffset(a, 0); ok {
		ax, ay := g.grid.ToScreen(gx, gy)
		return float32(ax - g.camX + fx), float32(ay + fy)
	}
	cx, cy := g.grid.Corner(gx, gy)
	sx, sy := g.cellSize()
	return float32(cx - g.camX + sx/2), float32(cy + sy/2)
}

// drawCells shades the cells by what a walk makes of them and marks where the
// hero would stand in each: red for ClosedVert (the scene's and the objects'),
// grey for the cells whose anchor is off the scene surface.
func (g *Game) drawCells(dst *ebiten.Image) {
	nx, ny := g.grid.Dims()
	sx, sy := g.cellSize()
	for gy := 0; gy < ny; gy++ {
		for gx := 0; gx < nx; gx++ {
			x, y := g.grid.Corner(gx, gy)
			x -= g.camX
			dot := nrgba(80, 255, 120, 230)
			switch {
			case g.grid.Blocked(gx, gy):
				dot = nrgba(255, 60, 60, 230)
				fillRect(dst, x, y, sx, sy, nrgba(255, 40, 40, 70))
				// crossed out too: a tint alone is lost on the sandy floors
				cross := nrgba(255, 60, 60, 160)
				x0, y0 := float32(x), float32(y)
				x1, y1 := float32(x+sx), float32(y+sy)
				vector.StrokeLine(dst, x0, y0, x1, y1, 1, cross, true)
				vector.StrokeLine(dst, x0, y1, x1, y0, 1, cross, true)
			case !g.grid.Valid(gx, gy):
				dot = nrgba(160, 160, 160, 200)
				fillRect(dst, x, y, sx, sy, nrgba(0, 0, 0, 100))
			}
			ebitenutil.DebugPrintAt(dst, fmt.Sprintf("%d,%d", gx, gy), x+3, y)
			px, py := g.standPoint(gx, gy)
			vector.FillCircle(dst, px, py, 2.5, dot, true)
		}
	}
	g.drawLattice(dst, nrgba(255, 255, 255, 110))
}

// drawFences draws each ClosedDir fence as a stub from the cell's stand point
// towards the neighbour it may not step to, capped with a bar: two stubs meet
// where the data fences both sides.
func (g *Game) drawFences(dst *ebiten.Image) {
	col := nrgba(255, 70, 70, 240)
	nx, ny := g.grid.Dims()
	for gy := 0; gy < ny; gy++ {
		for gx := 0; gx < nx; gx++ {
			x, y := g.standPoint(gx, gy)
			for d := 1; d <= 9; d++ {
				if d == 5 || !g.grid.Fenced(gx, gy, d) {
					continue
				}
				st := use_cases.StepDelta(d)
				tx, ty := g.standPoint(gx+st[0], gy+st[1])
				ex, ey := x+(tx-x)*0.4, y+(ty-y)*0.4
				vector.StrokeLine(dst, x, y, ex, ey, 2, col, true)
				l := float32(math.Hypot(float64(ex-x), float64(ey-y)))
				if l == 0 {
					continue
				}
				px, py := -(ey-y)/l*5, (ex-x)/l*5
				vector.StrokeLine(dst, ex-px, ey-py, ex+px, ey+py, 2, col, true)
			}
		}
	}
}

// drawZones outlines the click zones with their object keys (the names the
// scripts are composed from); the one a click would hit is drawn bright.
func (g *Game) drawZones(dst *ebiten.Image, hot *hotspot) {
	for i := range g.hotspots {
		hs := &g.hotspots[i]
		r := hs.rect.Add(image.Pt(-g.camX, 0))
		col, w := nrgba(255, 230, 0, 170), float32(1)
		if hs == hot {
			col, w = nrgba(255, 255, 255, 255), 2
		}
		vector.StrokeRect(dst, float32(r.Min.X)+0.5, float32(r.Min.Y)+0.5,
			float32(r.Dx()-1), float32(r.Dy()-1), w, col, false)
		ebitenutil.DebugPrintAt(dst, hs.key, r.Min.X, maxInt(0, r.Min.Y-12))
	}
}

// drawHeroes marks each character's cell at his stand point, the feet of the
// frame actually on screen (a cross, so a walk shows the sprite against the
// cell), and what is left of his route.
func (g *Game) drawHeroes(dst *ebiten.Image) {
	cyan := nrgba(0, 210, 255, 255)
	g.drawRoute(dst, g.idle, g.cell, g.path, nrgba(255, 230, 0, 230))
	x, y := g.standPoint(g.cell[0], g.cell[1])
	vector.StrokeCircle(dst, x, y, 8, 2, cyan, true)
	if a, fi := g.heroAnim(); g.act == nil && !g.charHidden {
		if fx, fy, ok := footOffset(a, fi); ok {
			drawCross(dst, float32(g.pos[0])-float32(g.camX)+float32(fx),
				float32(g.pos[1])+float32(fy), cyan)
		}
	}
	if g.gs.Var("FridIs") != 1 {
		return // Friday is not in the story yet
	}
	pink := nrgba(255, 90, 230, 255)
	g.drawRoute(dst, g.fridIdle, g.fridCell, g.fridPath, pink)
	x, y = g.standPointOf(g.fridIdle, g.fridCell[0], g.fridCell[1])
	vector.StrokeCircle(dst, x, y, 6, 2, pink, true)
	label := "frid"
	if g.fridHidden {
		label = "frid (hidden)"
	}
	ebitenutil.DebugPrintAt(dst, label, int(x)+8, int(y)-6)
}

// remaining is what is left of a route: the planned cells past the one the
// walker stands on now. A route keeps every cell it was planned with, and
// before the first step lands the walker is on none of them.
func remaining(cell [2]int, path [][2]int) [][2]int {
	for i, c := range path {
		if c == cell {
			return path[i+1:]
		}
	}
	return path
}

// drawRoute draws a walker's route from his cell through the cells still
// ahead, at the stand points of his pose a, ringing the goal.
func (g *Game) drawRoute(
	dst *ebiten.Image, a *adapters.Animation, cell [2]int, path [][2]int,
	col color.Color,
) {
	rest := remaining(cell, path)
	if len(rest) == 0 {
		return
	}
	x, y := g.standPointOf(a, cell[0], cell[1])
	for _, c := range rest {
		nx, ny := g.standPointOf(a, c[0], c[1])
		vector.StrokeLine(dst, x, y, nx, ny, 2, col, true)
		x, y = nx, ny
	}
	vector.StrokeCircle(dst, x, y, 5, 2, col, true)
}

// clickPreview is what a click at the cursor would do, worked out the way
// click() decides it.
type clickPreview struct {
	what  string   // for the HUD
	hot   *hotspot // the zone that takes the click
	cell  [2]int   // the cell under the cursor
	goal  [2]int   // where the hero would walk
	walk  bool     // the click walks
	route [][2]int // the route to goal
}

// previewClick follows click()'s order: a movie takes any click as a skip, the
// bar lives on its own, SetMouse OFF deafens the scene, then a zone, the
// acting character's own cell, and last a walk to the nearest free cell.
func (g *Game) previewClick(mx, my int) clickPreview {
	switch {
	case g.act != nil:
		return clickPreview{what: "skip"}
	case my >= PlayH:
		return clickPreview{what: "bar"}
	case !g.gs.UI["mouse"]:
		return clickPreview{what: "mouse OFF"}
	}
	wx, wy := mx+g.camX, my
	if hs := g.hotspotAt(wx, wy); hs != nil {
		what := fmt.Sprintf("%s z=%d -> %s", hs.key, hs.z,
			g.previewScript(actionName(g.gs.ActiveChar, g.gs.Active, hs.key)))
		return clickPreview{what: what, hot: hs}
	}
	cx, cy := g.grid.ToCell(wx, wy)
	p := clickPreview{cell: [2]int{cx, cy}}
	if own, standing := g.actingCell(); own == p.cell && standing {
		if scriptTok(g.gs.Active) == "HAN" || g.gs.Active == "" {
			p.what = "self: hand, eaten"
		} else {
			p.what = "self -> " + g.previewScript(
				selfActionName(g.gs.ActiveChar, g.gs.Active))
		}
		return p
	}
	tx, ty, ok := g.grid.NearestFree(cx, cy)
	if !ok {
		p.what = "no free cell"
		return p
	}
	p.goal, p.walk = [2]int{tx, ty}, true
	p.route = g.grid.Path(g.cell, p.goal)
	p.what = fmt.Sprintf("walk %d,%d", tx, ty)
	if p.goal != p.cell {
		p.what += fmt.Sprintf(" (nearest to %d,%d)", cx, cy)
	}
	if len(p.route) == 0 {
		p.what += " no route"
	}
	return p
}

// previewScript names the script a composed name resolves to in this scene,
// or says there is none (the engine eats the click). Lookups are remembered
// per scene, keyed with the CharVar redirection they went through.
func (g *Game) previewScript(base string) string {
	if g.sceneC == nil {
		return "none"
	}
	d := &g.dbg
	if d.scripts == nil || d.scriptsScene != g.sceneName {
		d.scripts, d.scriptsScene = map[string]string{}, g.sceneName
	}
	key := base + "|" + g.gs.CharVar(strings.ToLower(base))
	name, ok := d.scripts[key]
	if !ok {
		if n, _, found := g.lookupScript(base); found {
			name = n
		} else {
			name = base + "? none"
		}
		d.scripts[key] = name
	}
	return name
}

// drawPreview shows where a walk click would send the hero: the goal cell
// filled, the cell under the cursor outlined when the goal is its nearest free
// neighbour, and the route dotted.
func (g *Game) drawPreview(dst *ebiten.Image, pv clickPreview) {
	if !pv.walk {
		return
	}
	sx, sy := g.cellSize()
	col := nrgba(0, 210, 255, 255)
	x, y := g.grid.Corner(pv.goal[0], pv.goal[1])
	fillRect(dst, x-g.camX, y, sx, sy, nrgba(0, 210, 255, 60))
	if pv.cell != pv.goal {
		cx, cy := g.grid.Corner(pv.cell[0], pv.cell[1])
		vector.StrokeRect(dst, float32(cx-g.camX)+0.5, float32(cy)+0.5,
			float32(sx-1), float32(sy-1), 1, nrgba(0, 210, 255, 160), false)
	}
	for i := 1; i < len(pv.route); i++ {
		ax, ay := g.standPoint(pv.route[i-1][0], pv.route[i-1][1])
		bx, by := g.standPoint(pv.route[i][0], pv.route[i][1])
		drawDots(dst, ax, ay, bx, by, col)
	}
}

// drawHUD writes the frame's numbers at the top of the play area, where the
// scenes keep their scenery — every walk grid lies in the lower half.
func (g *Game) drawHUD(dst *ebiten.Image, pv clickPreview) {
	mx, my := ebiten.CursorPosition()
	wx := mx + g.camX
	cx, cy := g.grid.ToCell(wx, my)
	lines := []string{
		fmt.Sprintf("F1 %s  %s  fps %.0f  cam %d/%d  objs %d zones %d",
			Version, g.sceneName, ebiten.ActualFPS(), g.camX,
			maxInt(0, g.w-ViewW), len(g.sceneObjs), len(g.hotspots)),
		fmt.Sprintf("roby %d,%d z=%d walk=%v path=%d  %s",
			g.cell[0], g.cell[1], g.cell[1]*g.zper+g.robyZ, g.moving,
			len(remaining(g.cell, g.path)), g.fridDebug()),
		fmt.Sprintf("act %s  pending=%v", g.actDebug(), g.pending != nil),
		fmt.Sprintf("item %s  char %s  ui %s", g.gs.Active, g.gs.ActiveChar,
			uiDebug(g.gs.UI)),
		fmt.Sprintf("exits L=%s R=%s", g.exitDebug(true), g.exitDebug(false)),
		fmt.Sprintf("> %d,%d cell %d,%d: %s", wx, my, cx, cy, pv.what),
	}
	if g.moving {
		lines = append(lines, "cyc "+strings.Join(g.roby.cycles, " "))
	}
	drawTextBox(dst, 4, 4, lines, nil)
}

// fridDebug is Friday's part of the HUD.
func (g *Game) fridDebug() string {
	if g.gs.Var("FridIs") != 1 {
		return "frid -"
	}
	s := fmt.Sprintf("frid %d,%d z=%d path=%d", g.fridCell[0], g.fridCell[1],
		g.fridCell[1]*g.zper+g.fridZ, len(remaining(g.fridCell, g.fridPath)))
	if g.fridHidden {
		s += " hidden"
	}
	return s
}

// actDebug describes the running action for the debug overlay.
func (g *Game) actDebug() string {
	if g.act == nil {
		return "-"
	}
	name := g.act.name
	if name == "" {
		name = g.act.fs.MovieName
	}
	hold := ""
	switch g.act.wait {
	case waitRoby:
		hold = " wait=roby"
	case waitFrid:
		hold = " wait=frid"
	}
	return fmt.Sprintf("%s %d/%d%s", name, g.act.player.FrameIndex(),
		len(g.act.fs.Frames), hold)
}

// uiDebug lists the UI toggles that are on, in a stable order.
func uiDebug(ui map[string]bool) string {
	var on []string
	for k, v := range ui {
		if v {
			on = append(on, k)
		}
	}
	sort.Strings(on)
	return strings.Join(on, " ")
}

// exitDebug is one side's exit as the HUD shows it. The exit is read from the
// departure script, but it only leads out while its arrow object is on stage:
// SCENA2 keeps its goleft hidden until the bananas in SCENA3 are eaten.
func (g *Game) exitDebug(left bool) string {
	e := g.exitR
	if left {
		e = g.exitL
	}
	if !e.OK {
		return "-"
	}
	s := fmt.Sprintf("%s(%d,%d)", e.Scene, e.GX, e.GY)
	if key := exitKey(left); !g.objPresent(key) {
		s += " no " + key
	}
	return s
}

// logCommand keeps a command for the F2 command log. Sounds and the walk
// cycles' Set char,Z are left out: steps and ambient loops fire them all the
// time and would push the quest's own commands off the log.
func (g *Game) logCommand(c types.Command) {
	kw := strings.ToLower(c.Kw)
	if kw == "sound" ||
		kw == "set" && len(c.Args) >= 2 && strings.EqualFold(c.Args[1], "Z") {
		return
	}
	text := fmt.Sprintf("[%s] %s %s", g.sceneName, kw,
		strings.Join(c.Args, ","))
	d := &g.dbg
	if n := len(d.log); n > 0 && d.log[n-1].text == text {
		d.log[n-1].n++
		return
	}
	d.log = append(d.log, debugLine{text: text, n: 1})
	if over := len(d.log) - debugLogLines; over > 0 {
		d.log = d.log[over:]
	}
}

// noteStateChanges lights up the quest variables that changed since the last
// tick, for the F2 panel. A new run or a loaded save swaps the whole state
// out; that resets the baseline rather than lighting up everything.
func (g *Game) noteStateChanges(dt float64) {
	d := &g.dbg
	for k, t := range d.flash {
		if t -= dt; t > 0 {
			d.flash[k] = t
		} else {
			delete(d.flash, k)
		}
	}
	if d.seen != g.gs {
		d.seen = g.gs
		d.vars, d.chars = map[string]int{}, map[string]string{}
		d.flash = map[string]float64{}
		for k, v := range g.gs.Vars {
			d.vars[k] = v
		}
		for k, v := range g.gs.CharVars {
			d.chars[k] = v
		}
		return
	}
	for k, v := range g.gs.Vars {
		if old, ok := d.vars[k]; !ok || old != v {
			d.vars[k] = v
			d.flash["v:"+k] = debugFlash
		}
	}
	for k, v := range g.gs.CharVars {
		if old, ok := d.chars[k]; !ok || old != v {
			d.chars[k] = v
			d.flash["c:"+k] = debugFlash
		}
	}
}

// statePanel lays the F2 panel out as text: the non-zero quest variables (and
// any that just changed, zero or not), each character's items, the dialogue
// selectors moved off their own name, and the command log. lit marks the lines to light.
func (g *Game) statePanel() (vars, side, log []string, lit map[string]bool) {
	d := &g.dbg
	lit = map[string]bool{}
	names := make([]string, 0, len(g.gs.Vars))
	for k, v := range g.gs.Vars {
		if v != 0 || d.flash["v:"+k] > 0 {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	for _, k := range names {
		s := fmt.Sprintf("%s=%d", k, g.gs.Vars[k])
		vars = append(vars, s)
		if d.flash["v:"+k] > 0 {
			lit[s] = true
		}
	}
	for _, char := range []string{"Roby", "Frid"} {
		side = append(side, strings.ToLower(char)+": "+
			strings.Join(g.gs.InventoryOf(char), " "))
	}
	names = names[:0]
	for k, v := range g.gs.CharVars {
		if !strings.EqualFold(k, v) || d.flash["c:"+k] > 0 {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	for _, k := range names {
		s := fmt.Sprintf("%s=%s", k, g.gs.CharVars[k])
		side = append(side, s)
		if d.flash["c:"+k] > 0 {
			lit[s] = true
		}
	}
	for _, l := range d.log {
		s := l.text
		if l.n > 1 {
			s += fmt.Sprintf(" x%d", l.n)
		}
		log = append(log, s)
	}
	return vars, side, log, lit
}

// drawStatePanel is the F2 layer over the play area: variables in three
// columns on the left, selectors and inventory on the right, the command log
// along the bottom.
func (g *Game) drawStatePanel(screen *ebiten.Image) {
	vars, side, log, lit := g.statePanel()
	fillRect(screen, 0, 0, ViewW, PlayH, nrgba(0, 0, 0, 200))
	const colW, top = 128, 6
	logTop := PlayH - 8 - (len(log)+1)*debugLineH
	rows := maxInt(1, (logTop-top-debugLineH)/debugLineH)
	ebitenutil.DebugPrintAt(screen,
		fmt.Sprintf("F2 quest state  vars %d", len(vars)), 6, top)
	if fit := 3 * rows; len(vars) > fit {
		vars = append(vars[:fit-1], fmt.Sprintf("... +%d", len(vars)-fit+1))
	}
	for i, s := range vars {
		col, row := i/rows, i%rows
		drawTextBox(screen, 6+col*colW-4, top+(row+1)*debugLineH-2,
			[]string{clip(s, colW/debugCharW-1)}, lit)
	}
	for i, s := range side {
		y := top + (i+1)*debugLineH
		if y >= logTop-debugLineH {
			break
		}
		drawTextBox(screen, 6+3*colW, y-2,
			[]string{clip(s, (ViewW-3*colW-8)/debugCharW)}, lit)
	}
	ebitenutil.DebugPrintAt(screen, "commands", 6, logTop)
	for i, s := range log {
		ebitenutil.DebugPrintAt(screen, clip(s, ViewW/debugCharW-2), 6,
			logTop+(i+1)*debugLineH)
	}
}

// clip cuts a line to n characters.
func clip(s string, n int) string {
	if n > 0 && len(s) > n {
		return s[:n]
	}
	return s
}

// drawTextBox writes lines on a dark plate sized to them; a line found in lit
// gets a yellow plate instead.
func drawTextBox(
	dst *ebiten.Image, x, y int, lines []string, lit map[string]bool,
) {
	w := 0
	for _, l := range lines {
		w = maxInt(w, len(l)*debugCharW)
	}
	plate := nrgba(0, 0, 0, 150)
	if len(lines) == 1 && lit[lines[0]] {
		plate = nrgba(200, 160, 0, 200)
	}
	fillRect(dst, x, y, w+8, len(lines)*debugLineH+4, plate)
	for i, l := range lines {
		ebitenutil.DebugPrintAt(dst, l, x+4, y+i*debugLineH)
	}
}

// drawDots draws a dotted segment, the look of a route not yet taken.
func drawDots(dst *ebiten.Image, ax, ay, bx, by float32, col color.Color) {
	l := math.Hypot(float64(bx-ax), float64(by-ay))
	n := int(l / 7)
	for i := 0; i <= n; i++ {
		t := float32(0)
		if n > 0 {
			t = float32(i) / float32(n)
		}
		vector.FillCircle(dst, ax+(bx-ax)*t, ay+(by-ay)*t, 1.5, col, true)
	}
}

// drawCross marks a point with a small x.
func drawCross(dst *ebiten.Image, x, y float32, col color.Color) {
	const r = 4
	vector.StrokeLine(dst, x-r, y-r, x+r, y+r, 2, col, true)
	vector.StrokeLine(dst, x-r, y+r, x+r, y-r, 2, col, true)
}

func fillRect(dst *ebiten.Image, x, y, w, h int, col color.Color) {
	vector.FillRect(dst, float32(x), float32(y), float32(w), float32(h), col,
		false)
}

// nrgba is a straight-alpha colour: the translucent fills here would come out
// too bright as color.RGBA, which Ebiten reads as premultiplied.
func nrgba(r, g, b, a uint8) color.Color { return color.NRGBA{r, g, b, a} }
