// Package mcp serves the game over the Model Context Protocol to a client —
// a language model — and the subagents it runs: the coordinator passes the
// game's answers on, the narrator tells the story, Robinson decides, Friday
// advises. The server hands out data only: what the hero sees, holds and
// hears, and what changed (interfaces.IControl); every word said is the
// client's own. Who plays what is told by text files, the roles (roles.go).
// The game starts it with -mcp and talks over stdin/stdout.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// projectURL is where the remake lives.
const projectURL = "https://github.com/shpaker/modern-robinson"

type lookIn struct {
	Image  bool      `json:"image,omitempty"  jsonschema:"приложить картинку того, что сейчас в окне"`
	Grid   bool      `json:"grid,omitempty"   jsonschema:"приложить картинку с сеткой координат через 40 px (на крупном плане чаще); кликать по ним — только в головоломке"`
	Region *regionIn `json:"region,omitempty" jsonschema:"крупный план части картинки до 320×240 (в головоломке — экрана 640×480), чтобы точнее прицелиться; координаты в сетке на нём — экранные"`
}

// lookOut is the percept, and with a region what its close-up shows.
type lookOut struct {
	types.Percept
	Region *shownOut `json:"region,omitempty"`
}

type useIn struct {
	Target string `json:"target"         jsonschema:"к чему: имя из around или exits; «себя» — самому: надеть или положить рядом"`
	Item   string `json:"item,omitempty" jsonschema:"какую вещь взять в руки перед этим; пусто — ту, что уже в руках"`
	Why    string `json:"why"            jsonschema:"зачем это действие — коротко причина героя, которую перед вызовом выложили в чат: чего он хочет добиться и почему именно так"`
}

type goIn struct {
	To  string `json:"to"  jsonschema:"куда: имя из exits, а на карте острова — место из around"`
	Why string `json:"why" jsonschema:"зачем это действие — коротко причина героя, которую перед вызовом выложили в чат: чего он хочет добиться и почему именно так"`
}

type askIn struct {
	Target string `json:"target"         jsonschema:"к чему: имя из around или exits; «себя» — Пятница сам, без цели"`
	Item   string `json:"item,omitempty" jsonschema:"какую свою вещь ему взять; пусто — ту, что у него в руках"`
	Why    string `json:"why"            jsonschema:"зачем это действие — коротко причина героя, которую перед вызовом выложили в чат: чего он хочет добиться и почему именно так"`
}

// whyIn is an action that takes nothing but its reason.
type whyIn struct {
	Why string `json:"why" jsonschema:"зачем это действие — коротко причина героя, которую перед вызовом выложили в чат: чего он хочет добиться и почему именно так"`
}

type waitIn struct {
	Seconds float64 `json:"seconds,omitempty" jsonschema:"сколько секунд ждать, до 60; без него — пока не сможешь действовать"`
	Grid    bool    `json:"grid,omitempty"    jsonschema:"сетка координат через 40 px на картинке головоломки"`
}

type clickIn struct {
	X      int    `json:"x"                jsonschema:"x на экране головоломки, 0..639"`
	Y      int    `json:"y"                jsonschema:"y на экране головоломки, 0..479"`
	Button string `json:"button,omitempty" jsonschema:"left (по умолчанию) или right"`
	Grid   bool   `json:"grid,omitempty"   jsonschema:"сетка координат через 40 px на картинке ответа"`
	Why    string `json:"why"              jsonschema:"зачем это действие — коротко причина героя, которую перед вызовом выложили в чат: чего он хочет добиться и почему именно так"`
}

type moveIn struct {
	FromX int    `json:"from_x"          jsonschema:"x детали на экране головоломки, 0..639"`
	FromY int    `json:"from_y"          jsonschema:"y детали на экране головоломки, 0..479"`
	ToX   int    `json:"to_x"            jsonschema:"x, куда её положить (туда придёт её середина), 0..639"`
	ToY   int    `json:"to_y"            jsonschema:"y, куда её положить (туда придёт её середина), 0..479"`
	Turns int    `json:"turns,omitempty" jsonschema:"сколько раз повернуть её у цели правой кнопкой, 0..3"`
	Grid  bool   `json:"grid,omitempty"  jsonschema:"сетка координат через 40 px на картинке ответа"`
	Why   string `json:"why"             jsonschema:"зачем это действие — коротко причина героя, которую перед вызовом выложили в чат: чего он хочет добиться и почему именно так"`
}

type slotIn struct {
	Slot int `json:"slot" jsonschema:"слот от 0 до 11"`
}

type savedOut struct {
	Slot int `json:"slot"`
}

type roleIn struct {
	Who string `json:"who" jsonschema:"чья роль: coordinator — основная модель, narrator — рассказчик, robinson — Роби, friday — Пятница"`
}

type server struct {
	c interfaces.IControl
	r roles
}

// newServer builds the MCP server over a controlled hero, with the roles it
// hands out.
func newServer(
	c interfaces.IControl, version string, r roles,
) (*sdk.Server, error) {
	instr, err := r.instructions()
	if err != nil {
		return nil, err
	}
	s := &server{c: c, r: r}
	srv := sdk.NewServer(
		&sdk.Implementation{
			Name: "robinson", Title: "Новый Робинзон", Version: version,
			WebsiteURL: projectURL,
		},
		&sdk.ServerOptions{Instructions: instr},
	)
	add(srv, &sdk.Tool{
		Name:  "role",
		Title: "Роль (вне игры)",
		Description: "Полный текст роли: её файл из папки roles рядом с " +
			"игрой и общие правила. Первым делом возьми свою: основная " +
			"модель — coordinator, сабагент — своё имя. Есть: " +
			strings.Join(r.names(), ", ") + ".",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true},
	}, s.role)
	add(srv, &sdk.Tool{
		Name:        "look",
		Title:       "Осмотреться",
		Description: "Где ты, что вокруг, выходы, что в руках и с собой.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true},
	}, s.look)
	add(srv, &sdk.Tool{
		Name:        "use",
		Title:       "Применить",
		Description: "Подойти и применить вещь из рук к тому, что вокруг, или к себе.",
	}, s.use)
	add(srv, &sdk.Tool{
		Name:        "go",
		Title:       "Уйти",
		Description: "Уйти через выход; на карте острова — отправиться в место.",
	}, s.goTo)
	add(srv, &sdk.Tool{
		Name:        "map",
		Title:       "Карта острова",
		Description: "Развернуть карту острова, если она есть.",
	}, s.openMap)
	add(srv, &sdk.Tool{
		Name:        "ask_friday",
		Title:       "Попросить Пятницу",
		Description: "Попросить Пятницу применить его вещь к чему-то или к себе.",
	}, s.askFriday)
	add(srv, &sdk.Tool{
		Name:  "wait",
		Title: "Подождать",
		Description: "Переждать сцену или подождать столько секунд. " +
			"Головоломка между вызовами стоит, а wait даёт ей идти " +
			"столько секунд.",
	}, s.wait)
	add(srv, &sdk.Tool{
		Name:  "puzzle_click",
		Title: "Клик в головоломке",
		Description: "Кликнуть по экрану головоломки 640×480; отвечает, " +
			"когда она доиграла начатое, картинкой и тем, что прозвучало " +
			"(heard).",
	}, s.puzzleClick)
	add(srv, &sdk.Tool{
		Name:  "puzzle_move",
		Title: "Перенести в головоломке",
		Description: "Перенести деталь, как мышью: клик в from берёт её, " +
			"у to — turns правых кликов и клик, чтобы положить; если клик " +
			"в from ничего не взял и не выделил, дальше не идёт. Взятая " +
			"деталь висит на указателе серединой, кусок из нескольких " +
			"частей — на одной из них. Отвечает, когда головоломка " +
			"доиграла начатое, картинкой и тем, что прозвучало (heard).",
	}, s.puzzleMove)
	add(srv, &sdk.Tool{
		Name:  "puzzle_give_up",
		Title: "Бросить головоломку",
		Description: "Оставить головоломку нерешённой, как по Esc, " +
			"когда она доиграла начатое.",
	}, s.puzzleGiveUp)
	add(srv, &sdk.Tool{
		Name:        "save",
		Title:       "Сохранить (вне роли)",
		Description: "Вне роли: сохранить партию в слот 0–11.",
	}, s.save)
	add(srv, &sdk.Tool{
		Name:        "load",
		Title:       "Загрузить (вне роли)",
		Description: "Вне роли: загрузить партию из слота 0–11.",
	}, s.load)
	return srv, nil
}

// add offers a tool the client keeps in view from the start: Claude Code
// otherwise defers MCP tools behind a search, a subagent's ones too.
func add[In, Out any](
	srv *sdk.Server, t *sdk.Tool, h sdk.ToolHandlerFor[In, Out],
) {
	t.Meta = sdk.Meta{"anthropic/alwaysLoad": true}
	sdk.AddTool(srv, t, h)
}

// ServeStdio serves the hero over stdin and stdout until the client hangs up,
// with the roles of the given folders laid over the built-in ones. stdout
// belongs to the protocol from here on: whatever else the process prints
// goes to stderr.
func ServeStdio(
	ctx context.Context, c interfaces.IControl, version string, dirs ...fs.FS,
) error {
	srv, err := newServer(c, version, newRoles(dirs...))
	if err != nil {
		return err
	}
	out, err := claimStdout()
	if err != nil {
		return err
	}
	return srv.Run(ctx, &sdk.IOTransport{Reader: os.Stdin, Writer: out})
}

// role hands out who's role: its own text followed by the shared ones.
func (s *server) role(
	_ context.Context, _ *sdk.CallToolRequest, in roleIn,
) (*sdk.CallToolResult, any, error) {
	text, err := s.r.role(strings.TrimSpace(in.Who))
	if err != nil {
		return nil, nil, err
	}
	return &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: text}},
	}, nil, nil
}

func (s *server) look(
	ctx context.Context, _ *sdk.CallToolRequest, in lookIn,
) (*sdk.CallToolResult, lookOut, error) {
	p, err := s.c.Look(ctx)
	if err != nil {
		return nil, lookOut{}, err
	}
	out := lookOut{Percept: p}
	if in.Region != nil {
		pic, err := s.c.Sight(ctx)
		if err != nil {
			return nil, lookOut{}, err
		}
		pic, shown, err := closeUp(pic, *in.Region, in.Grid)
		if err != nil {
			return nil, lookOut{}, err
		}
		out.Region = &shown
		return reply(out, pic), out, nil
	}
	var pic []byte
	if in.Image || in.Grid || p.Where == types.WherePuzzle {
		pic = s.sight(ctx, in.Grid)
	}
	return reply(out, pic), out, nil
}

func (s *server) use(
	ctx context.Context, _ *sdk.CallToolRequest, in useIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	if err := reasoned(in.Why); err != nil {
		return nil, types.Outcome{}, err
	}
	return s.answer(ctx, false)(s.c.Use(ctx, in.Target, in.Item))
}

func (s *server) goTo(
	ctx context.Context, _ *sdk.CallToolRequest, in goIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	if err := reasoned(in.Why); err != nil {
		return nil, types.Outcome{}, err
	}
	return s.answer(ctx, false)(s.c.Go(ctx, in.To))
}

func (s *server) openMap(
	ctx context.Context, _ *sdk.CallToolRequest, in whyIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	if err := reasoned(in.Why); err != nil {
		return nil, types.Outcome{}, err
	}
	return s.answer(ctx, false)(s.c.OpenMap(ctx))
}

func (s *server) askFriday(
	ctx context.Context, _ *sdk.CallToolRequest, in askIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	if err := reasoned(in.Why); err != nil {
		return nil, types.Outcome{}, err
	}
	return s.answer(ctx, false)(s.c.AskFriday(ctx, in.Target, in.Item))
}

func (s *server) wait(
	ctx context.Context, _ *sdk.CallToolRequest, in waitIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx, in.Grid)(s.c.Wait(ctx, in.Seconds))
}

func (s *server) puzzleClick(
	ctx context.Context, _ *sdk.CallToolRequest, in clickIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	if err := reasoned(in.Why); err != nil {
		return nil, types.Outcome{}, err
	}
	right := strings.EqualFold(in.Button, "right")
	return s.answer(ctx, in.Grid)(s.c.PuzzleClick(ctx, in.X, in.Y, right))
}

func (s *server) puzzleMove(
	ctx context.Context, _ *sdk.CallToolRequest, in moveIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	if err := reasoned(in.Why); err != nil {
		return nil, types.Outcome{}, err
	}
	return s.answer(ctx, in.Grid)(s.c.PuzzleMove(ctx,
		in.FromX, in.FromY, in.ToX, in.ToY, in.Turns))
}

func (s *server) puzzleGiveUp(
	ctx context.Context, _ *sdk.CallToolRequest, in whyIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	if err := reasoned(in.Why); err != nil {
		return nil, types.Outcome{}, err
	}
	return s.answer(ctx, false)(s.c.PuzzleGiveUp(ctx))
}

func (s *server) save(
	ctx context.Context, _ *sdk.CallToolRequest, in slotIn,
) (*sdk.CallToolResult, savedOut, error) {
	if err := s.c.Save(ctx, in.Slot); err != nil {
		return nil, savedOut{}, err
	}
	return reply(savedOut(in), nil), savedOut(in), nil
}

func (s *server) load(
	ctx context.Context, _ *sdk.CallToolRequest, in slotIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx, false)(s.c.Load(ctx, in.Slot))
}

// errNoWhy refuses an action taken without a reason.
var errNoWhy = errors.New(
	"why пуст: сначала причина героя — в чат, а коротко — в why",
)

// reasoned refuses an action whose reason is blank: every action is meant to
// come after the hero's thought put in the chat (roles/common.md). The
// reason stays with the client; the game never sees it.
func reasoned(why string) error {
	if strings.TrimSpace(why) == "" {
		return errNoWhy
	}
	return nil
}

// answer turns an outcome into the tool's reply: the outcome as data, with
// the puzzle's picture while one is on screen — with the grid on request.
func (s *server) answer(ctx context.Context, grid bool) func(
	types.Outcome, error,
) (*sdk.CallToolResult, types.Outcome, error) {
	return func(o types.Outcome, err error) (
		*sdk.CallToolResult, types.Outcome, error,
	) {
		if err != nil {
			return nil, types.Outcome{}, err
		}
		var pic []byte
		if o.Look.Where == types.WherePuzzle {
			pic = s.sight(ctx, grid)
		}
		return reply(o, pic), o, nil
	}
}

// sight is the picture of what is in view, with grid the coordinate grid
// over it (withGrid); nil when there is none to give.
func (s *server) sight(ctx context.Context, grid bool) []byte {
	pic, err := s.c.Sight(ctx)
	if err != nil || len(pic) == 0 {
		return nil
	}
	if grid {
		return withGrid(pic)
	}
	return pic
}

// reply is data as JSON text — the structured content too, for clients that
// read it — and the picture, when there is one.
func reply(data any, pic []byte) *sdk.CallToolResult {
	raw, err := json.Marshal(data)
	if err != nil {
		raw = []byte("{}")
	}
	res := &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: string(raw)}},
	}
	if pic != nil {
		res.Content = append(res.Content,
			&sdk.ImageContent{Data: pic, MIMEType: "image/png"})
	}
	return res
}
