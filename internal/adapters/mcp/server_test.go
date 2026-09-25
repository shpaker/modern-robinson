package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// hero is a stand-in for the game: canned answers, and a record of what the
// client asked for.
type hero struct {
	look  types.Percept
	out   types.Outcome
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
	for _, c := range res.Content {
		if _, ok := c.(*sdk.ImageContent); ok {
			return true
		}
	}
	return false
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

// The server offers the hero's ten tools and the part to play: who he is,
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
		"puzzle_click", "puzzle_give_up", "save", "use", "wait",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("tools = %v, want %v", got, want)
	}
	init := cs.InitializeResult()
	for _, want := range []string{
		"Ты — Роби", "Готовых фраз нет", "changes", "misses",
		"некоторые выходы появляются", projectURL, "хорошего выживания",
		"Сохраняйся регулярно", "слоты 10 и 11",
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
	call(t, cs, "puzzle_give_up", map[string]any{"why": "не выходит"})
	if s := decode[savedOut](t,
		call(t, cs, "save", map[string]any{"slot": 3})); s.Slot != 3 {
		t.Errorf("saved = %+v", s)
	}
	call(t, cs, "load", map[string]any{"slot": 3})
	want := []string{
		"use Краб ", "use себя Панама", "go налево", "map",
		"friday Пальма ", "wait", "click 10 20 right", "give up", "save 3",
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
		"puzzle_click": true, "puzzle_give_up": true,
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
