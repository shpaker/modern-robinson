// Package mcp serves the hero over the Model Context Protocol: a client — a
// language model — plays Robinson himself. The server hands it data only:
// what the hero sees, holds and hears, and what changed (interfaces.IControl);
// every word said as the hero is the client's own. The game starts it with
// -mcp and talks over stdin/stdout.
package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// projectURL is where the remake lives.
const projectURL = "https://github.com/shpaker/modern-robinson"

// instructions is the part the client plays and what the data it gets
// means. The server hands out data only — names, sides, lines, what changed;
// every word said as the hero is the client's own.
const instructions = `Ты — Роби, Робинзон: обычного горожанина занесло на ` +
	`необитаемый остров, и надо с него выбраться. Ты и есть он: всё, что ` +
	`приходит от игры, — это то, что ты видишь, слышишь и держишь в руках. ` +
	`В чат пиши только от себя, как Роби, своими словами: что думаешь, что ` +
	`чувствуешь, что задумал. Готовых фраз нет — говори сам, по тому, что ` +
	`происходит. Какой Роби, узнавай из того, что он сам говорит в игре ` +
	`(said, hearing).

Что приходит от игры (JSON):
- where — где ты: остров, карта острова, головоломка, заставка, пауза; ` +
	`busy — что сейчас идёт, пока действовать нельзя.
- around — что вокруг (name, side: слева, справа, рядом); exits — куда ` +
	`можно уйти; на карте острова around — это места.
- hands — что в руках (empty_hands — руки свободны), carry — что ещё с ` +
	`собой; friday — Пятница рядом и её вещи; map — карту можно развернуть.
- hearing — реплика на экране; said — все реплики, прозвучавшие за ` +
	`действие, твои и чужие.
- reacted — мир откликнулся на действие; ready — можно действовать ` +
	`дальше; look — что вокруг после действия.
- changes — что изменилось: new_place, gained и lost (вещи), appeared и ` +
	`vanished (вокруг), opened и closed (выходы), friday_came, friday_left, ` +
	`map_gained, misses — сколько действий подряд ни к чему не привели.

Инструменты:
- look — осмотреться; с image — ещё и картинка того, что в окне.
- use — подойти и применить к чему-то вещь из рук; с item — сначала взять ` +
	`эту вещь. «Рука» — пустые руки: взять, потрогать, осмотреть, ` +
	`заговорить. target «себя» — применить вещь к себе.
- go — уйти через выход; на карте острова — отправиться в место.
- map — развернуть карту острова, когда она есть.
- ask_friday — попросить Пятницу применить её вещь к чему-то (или к себе), ` +
	`пока она рядом.
- wait — переждать сцену или просто подождать.
- puzzle_click, puzzle_give_up — головоломки решаются кликами по ` +
	`картинке (экран 640×480) или бросаются.
- save и load — вне роли: служебное сохранение партии в слоты 0–11.

Не все пути открыты сразу: некоторые выходы появляются, только когда ` +
	`сделаешь что-то нужное — здесь или в другом месте. Если reacted=false ` +
	`и никто ничего не сказал — так не выйдет, пробуй другое. Имена вещей, ` +
	`мест и выходов передавай в точности как в данных. Ты знаешь только то, ` +
	`что видит герой: подсказок игра не даёт.

Проект: ` + projectURL + `. Перед первым ходом поприветствуй игрока ` +
	`своими словами, дай ему эту ссылку и пожелай хорошего выживания.`

type lookIn struct {
	Image bool `json:"image,omitempty" jsonschema:"приложить картинку того, что сейчас в окне"`
}

type useIn struct {
	Target string `json:"target"         jsonschema:"к чему: имя из around или exits; «себя» — к себе"`
	Item   string `json:"item,omitempty" jsonschema:"какую вещь взять в руки перед этим; пусто — ту, что уже в руках"`
}

type goIn struct {
	To string `json:"to" jsonschema:"куда: имя из exits, а на карте острова — место из around"`
}

type askIn struct {
	Target string `json:"target"         jsonschema:"к чему: имя из around или exits; «себя» — Пятница к себе"`
	Item   string `json:"item,omitempty" jsonschema:"какую свою вещь ей взять; пусто — ту, что у неё в руках"`
}

type waitIn struct {
	Seconds float64 `json:"seconds,omitempty" jsonschema:"сколько секунд ждать, до 60; без него — пока не сможешь действовать"`
}

type clickIn struct {
	X      int    `json:"x"                jsonschema:"x на экране головоломки, 0..639"`
	Y      int    `json:"y"                jsonschema:"y на экране головоломки, 0..479"`
	Button string `json:"button,omitempty" jsonschema:"left (по умолчанию) или right"`
}

type slotIn struct {
	Slot int `json:"slot" jsonschema:"слот от 0 до 11"`
}

type savedOut struct {
	Slot int `json:"slot"`
}

type server struct{ c interfaces.IControl }

// newServer builds the MCP server over a controlled hero.
func newServer(c interfaces.IControl, version string) *sdk.Server {
	s := &server{c: c}
	srv := sdk.NewServer(
		&sdk.Implementation{
			Name: "robinson", Title: "Новый Робинзон", Version: version,
			WebsiteURL: projectURL,
		},
		&sdk.ServerOptions{Instructions: instructions},
	)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "look",
		Title:       "Осмотреться",
		Description: "Где ты, что вокруг, выходы, что в руках и с собой.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true},
	}, s.look)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "use",
		Title:       "Применить",
		Description: "Подойти и применить вещь из рук к тому, что вокруг, или к себе.",
	}, s.use)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "go",
		Title:       "Уйти",
		Description: "Уйти через выход; на карте острова — отправиться в место.",
	}, s.goTo)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "map",
		Title:       "Карта острова",
		Description: "Развернуть карту острова, если она есть.",
	}, s.openMap)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "ask_friday",
		Title:       "Попросить Пятницу",
		Description: "Попросить Пятницу применить её вещь к чему-то или к себе.",
	}, s.askFriday)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "wait",
		Title:       "Подождать",
		Description: "Переждать сцену или подождать столько секунд.",
	}, s.wait)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "puzzle_click",
		Title:       "Клик в головоломке",
		Description: "Кликнуть по экрану головоломки 640×480; отвечает картинкой.",
	}, s.puzzleClick)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "puzzle_give_up",
		Title:       "Бросить головоломку",
		Description: "Оставить головоломку нерешённой, как по Esc.",
	}, s.puzzleGiveUp)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "save",
		Title:       "Сохранить (вне роли)",
		Description: "Вне роли: сохранить партию в слот 0–11.",
	}, s.save)
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "load",
		Title:       "Загрузить (вне роли)",
		Description: "Вне роли: загрузить партию из слота 0–11.",
	}, s.load)
	return srv
}

// ServeStdio serves the hero over stdin and stdout until the client hangs up.
// stdout belongs to the protocol from here on: whatever else the process
// prints goes to stderr.
func ServeStdio(
	ctx context.Context, c interfaces.IControl, version string,
) error {
	out, err := claimStdout()
	if err != nil {
		return err
	}
	return newServer(c, version).Run(ctx,
		&sdk.IOTransport{Reader: os.Stdin, Writer: out})
}

func (s *server) look(
	ctx context.Context, _ *sdk.CallToolRequest, in lookIn,
) (*sdk.CallToolResult, types.Percept, error) {
	p, err := s.c.Look(ctx)
	if err != nil {
		return nil, types.Percept{}, err
	}
	return s.reply(ctx, p, in.Image || p.Where == types.WherePuzzle), p, nil
}

func (s *server) use(
	ctx context.Context, _ *sdk.CallToolRequest, in useIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx)(s.c.Use(ctx, in.Target, in.Item))
}

func (s *server) goTo(
	ctx context.Context, _ *sdk.CallToolRequest, in goIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx)(s.c.Go(ctx, in.To))
}

func (s *server) openMap(
	ctx context.Context, _ *sdk.CallToolRequest, _ struct{},
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx)(s.c.OpenMap(ctx))
}

func (s *server) askFriday(
	ctx context.Context, _ *sdk.CallToolRequest, in askIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx)(s.c.AskFriday(ctx, in.Target, in.Item))
}

func (s *server) wait(
	ctx context.Context, _ *sdk.CallToolRequest, in waitIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx)(s.c.Wait(ctx, in.Seconds))
}

func (s *server) puzzleClick(
	ctx context.Context, _ *sdk.CallToolRequest, in clickIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	right := strings.EqualFold(in.Button, "right")
	return s.answer(ctx)(s.c.PuzzleClick(ctx, in.X, in.Y, right))
}

func (s *server) puzzleGiveUp(
	ctx context.Context, _ *sdk.CallToolRequest, _ struct{},
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx)(s.c.PuzzleGiveUp(ctx))
}

func (s *server) save(
	ctx context.Context, _ *sdk.CallToolRequest, in slotIn,
) (*sdk.CallToolResult, savedOut, error) {
	if err := s.c.Save(ctx, in.Slot); err != nil {
		return nil, savedOut{}, err
	}
	return s.reply(ctx, savedOut(in), false), savedOut(in), nil
}

func (s *server) load(
	ctx context.Context, _ *sdk.CallToolRequest, in slotIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx)(s.c.Load(ctx, in.Slot))
}

// answer turns an outcome into the tool's reply: the outcome as data, with
// the puzzle's picture while one is on screen.
func (s *server) answer(ctx context.Context) func(
	types.Outcome, error,
) (*sdk.CallToolResult, types.Outcome, error) {
	return func(o types.Outcome, err error) (
		*sdk.CallToolResult, types.Outcome, error,
	) {
		if err != nil {
			return nil, types.Outcome{}, err
		}
		return s.reply(ctx, o, o.Look.Where == types.WherePuzzle), o, nil
	}
}

// reply is data as JSON text — the structured content too, for clients that
// read it — and, with sight, the picture of what is in view.
func (s *server) reply(
	ctx context.Context, data any, sight bool,
) *sdk.CallToolResult {
	raw, err := json.Marshal(data)
	if err != nil {
		raw = []byte("{}")
	}
	res := &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: string(raw)}},
	}
	if sight {
		if png, err := s.c.Sight(ctx); err == nil && len(png) > 0 {
			res.Content = append(res.Content,
				&sdk.ImageContent{Data: png, MIMEType: "image/png"})
		}
	}
	return res
}
