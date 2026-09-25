package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// hero is a stand-in for the game: canned answers, and a record of what the
// client asked for.
type hero struct {
	look  types.Percept
	out   types.Outcome
	sight []byte // what is in view; a bare PNG signature without it
	err   error
	calls []string
}

var _ interfaces.IControl = (*hero)(nil)

func (h *hero) note(
	s ...string,
) {
	h.calls = append(h.calls, strings.Join(s, " "))
}

func (h *hero) Look(
	context.Context,
) (types.Percept, error) {
	return h.look, h.err
}

func (h *hero) Sight(context.Context) ([]byte, error) {
	if h.sight != nil {
		return h.sight, nil
	}
	return []byte("\x89PNG"), nil
}

func (h *hero) Use(
	_ context.Context,
	target, item string,
) (types.Outcome, error) {
	h.note("use", target, item)
	return h.out, h.err
}

func (h *hero) Go(_ context.Context, to string) (types.Outcome, error) {
	h.note("go", to)
	return h.out, h.err
}

func (h *hero) OpenMap(context.Context) (types.Outcome, error) {
	h.note("map")
	return h.out, h.err
}

func (h *hero) AskFriday(
	_ context.Context,
	target, item string,
) (types.Outcome, error) {
	h.note("friday", target, item)
	return h.out, h.err
}

func (h *hero) Wait(context.Context, float64) (types.Outcome, error) {
	h.note("wait")
	return h.out, h.err
}

func (h *hero) PuzzleClick(
	_ context.Context,
	x, y int,
	right bool,
) (types.Outcome, error) {
	b := "left"
	if right {
		b = "right"
	}
	h.note("click", strconv.Itoa(x), strconv.Itoa(y), b)
	return h.out, h.err
}

func (h *hero) PuzzleMove(
	_ context.Context,
	fromX, fromY, toX, toY, turns int,
) (types.Outcome, error) {
	h.note("move", strconv.Itoa(fromX), strconv.Itoa(fromY),
		strconv.Itoa(toX), strconv.Itoa(toY), strconv.Itoa(turns))
	return h.out, h.err
}

func (h *hero) PuzzleGiveUp(context.Context) (types.Outcome, error) {
	h.note("give up")
	return h.out, h.err
}

func (h *hero) Save(_ context.Context, slot int) error {
	h.note("save", strconv.Itoa(slot))
	return h.err
}

func (h *hero) Load(_ context.Context, slot int) (types.Outcome, error) {
	h.note("load", strconv.Itoa(slot))
	return h.out, h.err
}

// connect serves the hero to an in-memory client.
func connect(t *testing.T, h *hero) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := sdk.NewInMemoryTransports()
	if _, err := newServer(h, "test").Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "t"}, nil).
		Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(
	t *testing.T, cs *sdk.ClientSession, name string, args map[string]any,
) *sdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(),
		&sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func text(res *sdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func hasImage(res *sdk.CallToolResult) bool {
	return picture(res) != nil
}

// picture is the image a reply carries, if any.
func picture(res *sdk.CallToolResult) []byte {
	for _, c := range res.Content {
		if ic, ok := c.(*sdk.ImageContent); ok {
			return ic.Data
		}
	}
	return nil
}

var beach = types.Percept{
	Where: types.WhereIsland,
	Around: []types.Thing{
		{Name: "Пальма", Side: types.SideNear},
		{Name: "Краб", Side: types.SideRight},
	},
	Exits:      []types.Thing{{Name: "налево", Side: types.SideLeft}},
	Hands:      "Рука",
	EmptyHands: true,
	Carry:      []string{"Панама"},
	Map:        true,
}

// The server offers the hero's eleven tools and the part to play: who he is,
// what the data means, and no words to say.
func TestServerOffersTheHerosTools(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)
	want := []string{
		"ask_friday", "go", "load", "look", "map",
		"puzzle_click", "puzzle_give_up", "puzzle_move", "save", "use",
		"wait",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("tools = %v, want %v", got, want)
	}
	init := cs.InitializeResult()
	for _, want := range []string{
		"Ты — Роби", "Готовых фраз нет", "changes", "misses",
		"некоторые выходы появляются", projectURL, "хорошего выживания",
		"Сохраняйся регулярно", "слоты 10 и 11",
		"анекдотом", "ничего не выдумывай",
		"heard", "видит и слышит", "puzzle_move", "серединой",
	} {
		if !strings.Contains(init.Instructions, want) {
			t.Errorf("the instructions lack %q", want)
		}
	}
	if init.ServerInfo.WebsiteURL != projectURL {
		t.Errorf("website = %q", init.ServerInfo.WebsiteURL)
	}
}

// decode reads a reply's data back.
func decode[T any](t *testing.T, res *sdk.CallToolResult) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(text(res)), &v); err != nil {
		t.Fatalf("reply is no data: %v\n%s", err, text(res))
	}
	return v
}

// look answers with the percept itself, and the picture on request.
func TestLookAnswersWithData(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	res := call(t, cs, "look", nil)
	p := decode[types.Percept](t, res)
	if p.Where != types.WhereIsland || len(p.Around) != 2 ||
		p.Around[1].Name != "Краб" || !p.EmptyHands || p.Carry[0] != "Панама" {
		t.Errorf("percept = %+v", p)
	}
	if res.StructuredContent == nil {
		t.Error("no structured percept")
	}
	if hasImage(res) {
		t.Error("the picture is on request only")
	}
	if !hasImage(call(t, cs, "look", map[string]any{"image": true})) {
		t.Error("image:true brings the picture")
	}
}

// Actions pass their words through and answer with the outcome as it came:
// the game's lines and the changes, nothing phrased by the server.
func TestActionsReachTheHero(t *testing.T) {
	h := &hero{look: beach, out: types.Outcome{
		Said: []string{"Попался, который кусался"}, Reacted: true,
		Ready: true, Look: beach,
		Changes: types.Changes{
			Gained: []string{"Краб в шляпе"}, Lost: []string{"Панама"},
		},
	}}
	cs := connect(t, h)
	o := decode[types.Outcome](t,
		call(t, cs, "use", map[string]any{"target": "Краб", "why": "поймать"}))
	if len(o.Said) != 1 || o.Said[0] != "Попался, который кусался" ||
		o.Changes.Gained[0] != "Краб в шляпе" {
		t.Errorf("outcome = %+v", o)
	}
	call(t, cs, "use", map[string]any{
		"target": "себя", "item": "Панама", "why": "от солнца",
	})
	call(t, cs, "go", map[string]any{"to": "налево", "why": "осмотреться"})
	call(t, cs, "map", map[string]any{"why": "куда дальше"})
	call(t, cs, "ask_friday", map[string]any{
		"target": "Пальма", "why": "она выше",
	})
	call(t, cs, "wait", map[string]any{"seconds": 2})
	call(
		t,
		cs,
		"puzzle_click",
		map[string]any{"x": 10, "y": 20, "button": "right", "why": "повернуть"},
	)
	call(t, cs, "puzzle_move", map[string]any{
		"from_x": 400, "from_y": 100, "to_x": 150, "to_y": 60, "turns": 2,
		"why": "бревно на место",
	})
	call(t, cs, "puzzle_move", map[string]any{
		"from_x": 1, "from_y": 2, "to_x": 3, "to_y": 4, "why": "без поворота",
	})
	call(t, cs, "puzzle_give_up", map[string]any{"why": "не выходит"})
	if s := decode[savedOut](t,
		call(t, cs, "save", map[string]any{"slot": 3})); s.Slot != 3 {
		t.Errorf("saved = %+v", s)
	}
	call(t, cs, "load", map[string]any{"slot": 3})
	want := []string{
		"use Краб ", "use себя Панама", "go налево", "map",
		"friday Пальма ", "wait", "click 10 20 right",
		"move 400 100 150 60 2", "move 1 2 3 4 0", "give up", "save 3",
		"load 3",
	}
	if strings.Join(h.calls, "|") != strings.Join(want, "|") {
		t.Errorf("calls = %q\nwant  %q", h.calls, want)
	}
}

// A refusal reaches the client as a tool error, as the game gave it.
func TestRefusalIsAToolError(t *testing.T) {
	cs := connect(t, &hero{err: errors.New("занято: сцена — wait")})
	res := call(t, cs, "use", map[string]any{"target": "Краб", "why": "так"})
	if !res.IsError || text(res) != "занято: сцена — wait" {
		t.Errorf("error = %v %q", res.IsError, text(res))
	}
}

// A puzzle is played by picture: every answer under it carries the screen.
func TestPuzzleRepliesCarryThePicture(t *testing.T) {
	puzzle := types.Percept{Where: types.WherePuzzle}
	cs := connect(t, &hero{look: puzzle, out: types.Outcome{
		Reacted: true, Ready: true, Look: puzzle,
	}})
	res := call(t, cs, "look", nil)
	if !hasImage(res) || decode[types.Percept](t, res).Where !=
		types.WherePuzzle {
		t.Errorf("look under a puzzle: %q image=%v", text(res), hasImage(res))
	}
	if !hasImage(call(t, cs, "puzzle_click",
		map[string]any{"x": 1, "y": 2, "why": "пробую"})) {
		t.Error("a puzzle click answers with the picture")
	}
	if !hasImage(call(t, cs, "puzzle_move", map[string]any{
		"from_x": 1, "from_y": 2, "to_x": 3, "to_y": 4, "why": "переношу",
	})) {
		t.Error("a puzzle move answers with the picture")
	}
}

// What a puzzle sounded comes as data, in order, the way the game heard it;
// an answer with nothing heard carries no such field at all.
func TestPuzzleSoundsComeAsData(t *testing.T) {
	puzzle := types.Percept{Where: types.WherePuzzle}
	h := &hero{look: puzzle, out: types.Outcome{
		Heard: []string{"взял", "встало"}, Reacted: true, Look: puzzle,
	}}
	cs := connect(t, h)
	res := call(t, cs, "puzzle_click",
		map[string]any{"x": 5, "y": 6, "why": "ставлю бревно"})
	if o := decode[types.Outcome](t, res); strings.Join(o.Heard, "|") !=
		"взял|встало" {
		t.Errorf("heard = %q", o.Heard)
	}
	h.out = types.Outcome{Reacted: true, Look: puzzle}
	res = call(t, cs, "puzzle_click",
		map[string]any{"x": 5, "y": 6, "why": "ещё раз"})
	if strings.Contains(text(res), `"heard"`) {
		t.Errorf("nothing heard, yet: %s", text(res))
	}
}

// Every action asks for its reason, and a blank one is turned away before
// the game hears of it; looking and waiting need none.
func TestActionsNeedAReason(t *testing.T) {
	h := &hero{look: beach, out: types.Outcome{Look: beach}}
	cs := connect(t, h)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	acts := map[string]bool{
		"use": true, "go": true, "map": true, "ask_friday": true,
		"puzzle_click": true, "puzzle_move": true, "puzzle_give_up": true,
	}
	for _, tool := range res.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Required []string `json:"required"`
		}
		_ = json.Unmarshal(raw, &schema)
		has := strings.Contains(","+strings.Join(schema.Required, ",")+",",
			",why,")
		if has != acts[tool.Name] {
			t.Errorf("%s: why required = %v", tool.Name, has)
		}
	}
	for _, args := range []map[string]any{
		{"target": "Краб"}, {"target": "Краб", "why": "  "},
	} {
		r := call(t, cs, "use", args)
		if !r.IsError || !strings.Contains(text(r), "why") {
			t.Errorf("use %v: %v %q", args, r.IsError, text(r))
		}
	}
	if len(h.calls) > 0 {
		t.Errorf("an action without a reason reached the game: %q", h.calls)
	}
	if !strings.Contains(connect(t, h).InitializeResult().Instructions,
		thinkFirst) {
		t.Error("the instructions lack the thought before every action")
	}
}

// A move names both of its ends and may leave the turns out; what it heard
// comes back as it came, in order.
func TestPuzzleMoveNamesBothEnds(t *testing.T) {
	puzzle := types.Percept{Where: types.WherePuzzle}
	h := &hero{look: puzzle, out: types.Outcome{
		Heard:   []string{"взял", "повернул", "не туда"},
		Reacted: true, Look: puzzle,
	}}
	cs := connect(t, h)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "puzzle_move" {
			continue
		}
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Required []string `json:"required"`
		}
		_ = json.Unmarshal(raw, &schema)
		sort.Strings(schema.Required)
		if got := strings.Join(schema.Required, ","); got !=
			"from_x,from_y,to_x,to_y,why" {
			t.Errorf("puzzle_move requires %s", got)
		}
	}
	o := decode[types.Outcome](t, call(t, cs, "puzzle_move", map[string]any{
		"from_x": 400, "from_y": 100, "to_x": 150, "to_y": 60, "turns": 1,
		"why": "бревно на место",
	}))
	if strings.Join(o.Heard, "|") != "взял|повернул|не туда" {
		t.Errorf("heard = %q", o.Heard)
	}
}

// The instructions retell the manual's puzzle rules: every game by the name
// the player sees it under, and not a number the manual does not give — no
// spots on the screen, no tolerances; the only words not in Russian are the
// key and the tools the player has.
func TestInstructionsRetellThePuzzleRules(t *testing.T) {
	init := connect(t, &hero{look: beach}).InitializeResult()
	if !strings.Contains(init.Instructions, puzzleRules) {
		t.Error("the instructions lack the puzzle rules")
	}
	for _, want := range []string{
		"Хижина", "Карта", "Записка", "Воздушный шар", "Мелодия на органе",
		"Шашки с пиратом", "Esc", "сохраниться посреди головоломки нельзя",
	} {
		if !strings.Contains(puzzleRules, want) {
			t.Errorf("the puzzle rules lack %q", want)
		}
	}
	bare := strings.NewReplacer("90°", "", "300 м", "").Replace(puzzleRules)
	for _, w := range strings.Fields(bare) {
		if strings.IndexFunc(w, unicode.IsDigit) >= 0 {
			t.Errorf("the puzzle rules give a number: %q", w)
		}
	}
	known := map[string]bool{
		"Esc": true, "puzzle_click": true, "puzzle_move": true,
		"puzzle_give_up": true,
	}
	for _, w := range regexp.MustCompile(`[A-Za-z_]+`).
		FindAllString(puzzleRules, -1) {
		if !known[w] {
			t.Errorf("the puzzle rules name %q", w)
		}
	}
}

// A puzzle stands between calls, and the client is told so where it acts:
// the instructions and the wait tool say a wait is the puzzle's time — the
// balloon's flight too — and a move answers once the puzzle has played out.
// The manual retold says nothing of it: the player's balloon flies on.
func TestWaitIsThePuzzlesTime(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "wait" &&
			!strings.Contains(tool.Description, "Головоломка") {
			t.Errorf("the wait tool says nothing of puzzles: %q",
				tool.Description)
		}
	}
	init := cs.InitializeResult()
	for _, want := range []string{
		"между вызовами стоит", "wait с seconds", "летит воздушный шар",
		"доиграла",
	} {
		if !strings.Contains(init.Instructions, want) {
			t.Errorf("the instructions lack %q", want)
		}
	}
	if strings.Contains(puzzleRules, "wait") {
		t.Error("the manual retold speaks of the wait tool")
	}
}

// The grid comes on request, over the very picture the game gave: look
// brings it anywhere, the puzzle's moves and wait lay it over the puzzle
// they answer with; without it the picture goes byte for byte as it came.
func TestGridOnRequest(t *testing.T) {
	puzzle := types.Percept{Where: types.WherePuzzle}
	screen := flat(t, 640, 480, grounds["grey"])
	h := &hero{look: puzzle, sight: screen, out: types.Outcome{
		Reacted: true, Ready: true, Look: puzzle,
	}}
	cs := connect(t, h)
	click := map[string]any{"x": 1, "y": 2, "why": "пробую"}
	move := map[string]any{
		"from_x": 1, "from_y": 2, "to_x": 3, "to_y": 4, "why": "переношу",
	}
	for name, args := range map[string]map[string]any{
		"look": nil, "puzzle_click": click, "puzzle_move": move, "wait": nil,
	} {
		if got := picture(call(t, cs, name, args)); !bytes.Equal(got, screen) {
			t.Errorf("%s without grid changed the picture", name)
		}
		gridded := map[string]any{"grid": true}
		for k, v := range args {
			gridded[k] = v
		}
		got := picture(call(t, cs, name, gridded))
		if bytes.Equal(got, screen) {
			t.Errorf("%s with grid: no grid", name)
			continue
		}
		if img := unpack(t, got); img.RGBAAt(40, 250) == grounds["grey"] ||
			img.RGBAAt(60, 250) != grounds["grey"] {
			t.Errorf("%s with grid: not the grid over the screen", name)
		}
	}
	h.look = beach
	scene := flat(t, 640, 400, grounds["grey"])
	h.sight = scene
	if hasImage(call(t, cs, "look", nil)) {
		t.Error("a scene's picture is on request only")
	}
	got := picture(call(t, cs, "look", map[string]any{"grid": true}))
	if got == nil || unpack(t, got).Bounds() != image.Rect(0, 0, 640, 400) {
		t.Error("grid:true brings the scene's picture with the grid")
	}
	if !bytes.Equal(h.sight, scene) {
		t.Error("the grid drew on the game's own picture")
	}
}

// The grid is asked for where there is a picture to lay it on, and the
// client is told what its numbers are worth outside a puzzle.
func TestGridIsOffered(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"look": true, "puzzle_click": true, "puzzle_move": true, "wait": true,
	}
	for _, tool := range res.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]struct {
				Type string `json:"type"`
			} `json:"properties"`
		}
		_ = json.Unmarshal(raw, &schema)
		p, has := schema.Properties["grid"]
		if has != want[tool.Name] || has && p.Type != "boolean" {
			t.Errorf("%s: grid = %v %q", tool.Name, has, p.Type)
		}
	}
	init := cs.InitializeResult()
	for _, want := range []string{
		"с grid", "40 px", "x подписан сверху", "только в головоломке",
	} {
		if !strings.Contains(init.Instructions, want) {
			t.Errorf("the instructions lack %q", want)
		}
	}
}

// images is how many pictures a reply carries.
func images(res *sdk.CallToolResult) int {
	n := 0
	for _, c := range res.Content {
		if _, ok := c.(*sdk.ImageContent); ok {
			n++
		}
	}
	return n
}

// look with a region brings the close-up of that part of the picture, in a
// scene as in a puzzle, in place of the whole picture; the region as the
// picture's edges left it and the scale come with it. Without a region look
// answers as it did, and a region that makes no close-up is refused.
func TestLookRegionIsACloseUp(t *testing.T) {
	scene := flat(t, 640, 400, grounds["grey"])
	h := &hero{look: beach, sight: scene}
	cs := connect(t, h)
	want, err := json.Marshal(beach)
	if err != nil {
		t.Fatal(err)
	}
	if got := text(call(t, cs, "look", nil)); got != string(want) {
		t.Errorf("look without a region:\n%s\nwant\n%s", got, want)
	}
	past := map[string]any{"x0": 600, "y0": 300, "x1": 700, "y1": 420}
	res := call(t, cs, "look", map[string]any{"region": past})
	out := decode[lookOut](t, res)
	if out.Region == nil || *out.Region != (shownOut{600, 300, 640, 400, 4}) {
		t.Errorf("shown = %+v", out.Region)
	}
	if out.Where != types.WhereIsland || out.Around[1].Name != "Краб" {
		t.Errorf("percept = %+v", out.Percept)
	}
	if images(res) != 1 || unpack(t, picture(res)).Bounds() !=
		image.Rect(0, 0, 160, 400) {
		t.Errorf("close-up: %d pictures", images(res))
	}
	gridded := picture(call(t, cs, "look",
		map[string]any{"region": past, "grid": true}))
	if bytes.Equal(gridded, picture(res)) {
		t.Error("region with grid: no grid")
	}
	if !bytes.Equal(h.sight, scene) {
		t.Error("the close-up drew on the game's own picture")
	}
	h.look = types.Percept{Where: types.WherePuzzle}
	h.sight = pattern(t, 640, 480)
	res = call(t, cs, "look", map[string]any{
		"region": map[string]any{"x0": 100, "y0": 100, "x1": 260, "y1": 220},
	})
	shown := decode[lookOut](t, res).Region
	if images(res) != 1 || unpack(t, picture(res)).Bounds() !=
		image.Rect(0, 0, 640, 480) || shown == nil || shown.Scale != 4 {
		t.Errorf("a puzzle's close-up: %s", text(res))
	}
	res = call(t, cs, "look", map[string]any{
		"region": map[string]any{"x0": 10, "y0": 10, "x1": 5, "y1": 20},
	})
	if !res.IsError || !strings.HasPrefix(text(res), "region") {
		t.Errorf("an inside-out region: %v %q", res.IsError, text(res))
	}
}

// Only look offers a region, all four of its edges asked for, and the
// client is told what it is for and what the grid's numbers on it are.
func TestRegionIsOffered(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]struct {
				Required []string `json:"required"`
			} `json:"properties"`
		}
		_ = json.Unmarshal(raw, &schema)
		p, has := schema.Properties["region"]
		if has != (tool.Name == "look") {
			t.Errorf("%s: region = %v", tool.Name, has)
		}
		sort.Strings(p.Required)
		if has && strings.Join(p.Required, ",") != "x0,x1,y0,y1" {
			t.Errorf("region requires %v", p.Required)
		}
	}
	init := cs.InitializeResult()
	for _, want := range []string{
		"region {x0,y0,x1,y1}", "крупный план", "точнее прицелиться",
		"экранные", "scale",
	} {
		if !strings.Contains(init.Instructions, want) {
			t.Errorf("the instructions lack %q", want)
		}
	}
}
