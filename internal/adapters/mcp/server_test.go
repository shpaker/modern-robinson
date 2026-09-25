package mcp

import (
	"context"
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

func (h *hero) Voice() []string { return []string{"Где я?", "Бедный Роби!"} }

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

// The server offers the hero's ten tools and the part to play.
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
		"Ты — Роби", "от первого лица", "«Где я?», «Бедный Роби!»",
		"некоторые выходы появляются",
	} {
		if !strings.Contains(init.Instructions, want) {
			t.Errorf("the instructions lack %q", want)
		}
	}
	if init.ServerInfo.WebsiteURL != projectURL {
		t.Errorf("website = %q", init.ServerInfo.WebsiteURL)
	}
}

// The session's first reply greets the player with the project's link; the
// rest do not.
func TestFirstReplyGreets(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	first := text(call(t, cs, "look", nil))
	if !strings.HasPrefix(first, greeting) ||
		!strings.Contains(first, projectURL) {
		t.Errorf("first reply:\n%s", first)
	}
	if s := text(call(t, cs, "look", nil)); strings.Contains(s, projectURL) {
		t.Errorf("the greeting came twice:\n%s", s)
	}
}

// look answers in words and in structure; the picture comes on request.
func TestLookTellsWhatIsAround(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	res := call(t, cs, "look", nil)
	s := text(res)
	for _, want := range []string{
		"Я на острове.", "Вокруг меня: Пальма (рядом), Краб (справа).",
		"Отсюда можно уйти: налево (слева).", "Руки свободны.",
		"С собой: Панама.", "(map)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("look lacks %q:\n%s", want, s)
		}
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

// Actions pass their words through, and tell what came of them.
func TestActionsReachTheHero(t *testing.T) {
	h := &hero{look: beach, out: types.Outcome{
		Said: []string{`"Он меня чуть не укусил!!!"`}, Reacted: true,
		Ready: true, Look: beach,
	}}
	cs := connect(t, h)
	s := text(call(t, cs, "use", map[string]any{"target": "Краб"}))
	if !strings.Contains(s, "Прозвучало:\n— \"Он меня чуть не укусил!!!\"") {
		t.Errorf("use reply:\n%s", s)
	}
	call(t, cs, "use", map[string]any{"target": "себя", "item": "Панама"})
	call(t, cs, "go", map[string]any{"to": "налево"})
	call(t, cs, "map", nil)
	call(t, cs, "ask_friday", map[string]any{"target": "Пальма"})
	call(t, cs, "wait", map[string]any{"seconds": 2})
	call(
		t,
		cs,
		"puzzle_click",
		map[string]any{"x": 10, "y": 20, "button": "right"},
	)
	call(t, cs, "puzzle_give_up", nil)
	call(t, cs, "save", map[string]any{"slot": 3})
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

// Nothing happening is news too, and so is a scene still playing — but a
// wait has nothing to fail at.
func TestOutcomeSaysWhenNothingHappened(t *testing.T) {
	cs := connect(t, &hero{out: types.Outcome{Look: beach}})
	s := text(call(t, cs, "use", map[string]any{"target": "Пальма"}))
	if !strings.Contains(s, "Ничего не произошло.") ||
		!strings.Contains(s, "подождать (wait)") {
		t.Errorf("reply:\n%s", s)
	}
	if s := text(call(t, cs, "wait", nil)); strings.Contains(s, "Ничего") {
		t.Errorf("a wait reported failure:\n%s", s)
	}
}

// What changed is told as plain facts, for the client to feel something
// about; a miss counts only from the second in a row.
func TestOutcomeTellsWhatChanged(t *testing.T) {
	cs := connect(t, &hero{out: types.Outcome{
		Reacted: true, Ready: true, Look: beach,
		Changes: types.Changes{
			Gained: []string{"Краб в шляпе"}, Lost: []string{"Панама"},
			Vanished: []string{"Краб"}, Opened: []string{"направо"},
			Misses: 1,
		},
	}})
	s := text(call(t, cs, "use", map[string]any{"target": "Краб"}))
	for _, want := range []string{
		"Изменилось:\n— теперь у меня: Краб в шляпе\n— больше нет: Панама",
		"— пропало из виду: Краб", "— открылся путь: направо",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("reply lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "подряд") {
		t.Errorf("a single miss is no streak:\n%s", s)
	}
	if got := narrateChanges(types.Changes{Misses: 3}); !strings.Contains(
		got, "3-й раз подряд") {
		t.Errorf("misses: %q", got)
	}
}

// A refusal reaches the client as a tool error in a sentence.
func TestRefusalIsAToolError(t *testing.T) {
	cs := connect(t, &hero{err: errors.New("сейчас не выйдет: идёт сцена")})
	res := call(t, cs, "use", map[string]any{"target": "Краб"})
	if !res.IsError || text(res) != "Сейчас не выйдет: идёт сцена." {
		t.Errorf("error = %v %q", res.IsError, text(res))
	}
}

// A puzzle is played by picture: every answer under it carries the screen.
func TestPuzzleRepliesCarryThePicture(t *testing.T) {
	puzzle := types.Percept{Where: types.WherePuzzle}
	cs := connect(t, &hero{look: puzzle, out: types.Outcome{
		Reacted: true, Ready: true, Look: puzzle,
	}})
	if res := call(t, cs, "look", nil); !hasImage(res) ||
		!strings.Contains(text(res), "головоломка") {
		t.Errorf("look under a puzzle: %q image=%v", text(res), hasImage(res))
	}
	res := call(t, cs, "puzzle_click", map[string]any{"x": 1, "y": 2})
	if !hasImage(res) {
		t.Error("a puzzle click answers with the picture")
	}
}
