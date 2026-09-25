// Package mcp serves the hero over the Model Context Protocol: a client — a
// language model — plays Robinson himself. It looks around, uses his things
// on what it sees, leaves, asks Friday, waits, and learns only what the hero
// sees and hears (interfaces.IControl). The game starts it with -mcp and
// talks over stdin/stdout.
package mcp

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// instructions is the part the client plays, and how the world answers.
const instructions = `Ты — Робинзон: тебя выбросило на необитаемый остров, ` +
	`и надо с него выбраться. Играй от первого лица, как сам герой.

- look — осмотреться: где ты, что вокруг и с какой стороны, выходы, что ` +
	`в руках и с собой, Пятница, карта. С image — ещё и картинка.
- use — подойти и применить к чему-то вещь из рук; с item — сначала взять ` +
	`эту вещь. «Рука» — пустые руки: взять, потрогать, осмотреть, ` +
	`заговорить. target «себя» — применить вещь к себе.
- go — уйти через выход; на карте острова — отправиться в место.
- map — развернуть карту острова, когда она у тебя есть.
- ask_friday — попросить Пятницу применить её вещь к чему-то (или к себе), ` +
	`пока она рядом.
- wait — переждать сцену или просто подождать.
- Головоломки решаются руками по картинке: puzzle_click по экрану 640×480, ` +
	`puzzle_give_up — бросить.
- save и load — вне роли: служебное сохранение партии в слоты 0–11.

Каждое действие отвечает, что прозвучало и что вокруг теперь. Если ничего ` +
	`не произошло — так не выйдет, пробуй другое. Имена вещей, мест и ` +
	`выходов пиши в точности как в look. Ты знаешь только то, что видит ` +
	`герой: подсказок игра не даёт.

Перед первым ходом передай игроку в чат приветствие из первого ответа: ` +
	`ссылку на проект и пожелание хорошего выживания. Дальше комментируй ` +
	`в чате от лица Робинзона: коротко, от первого лица, в его характере — ` +
	`что видишь, что задумал, что из этого вышло.`

// projectURL is where the remake lives.
const projectURL = "https://github.com/shpaker/modern-robinson"

// greeting opens the first reply of a session, for the client to pass on to
// the player.
const greeting = "Добро пожаловать на остров! «Новый Робинзон» — ремейк: " +
	projectURL + ". Хорошего выживания!"

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

type server struct {
	c       interfaces.IControl
	greeted atomic.Bool // the first reply has carried the greeting
}

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
		Description: "Развернуть карту острова, если она у тебя есть.",
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
		return nil, types.Percept{}, refusal(err)
	}
	res := s.reply(narrate(p))
	if in.Image || p.Where == types.WherePuzzle {
		s.attachSight(ctx, res)
	}
	return res, p, nil
}

func (s *server) use(
	ctx context.Context, _ *sdk.CallToolRequest, in useIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx, true)(s.c.Use(ctx, in.Target, in.Item))
}

func (s *server) goTo(
	ctx context.Context, _ *sdk.CallToolRequest, in goIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx, true)(s.c.Go(ctx, in.To))
}

func (s *server) openMap(
	ctx context.Context, _ *sdk.CallToolRequest, _ struct{},
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx, true)(s.c.OpenMap(ctx))
}

func (s *server) askFriday(
	ctx context.Context, _ *sdk.CallToolRequest, in askIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx, true)(s.c.AskFriday(ctx, in.Target, in.Item))
}

func (s *server) wait(
	ctx context.Context, _ *sdk.CallToolRequest, in waitIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx, false)(s.c.Wait(ctx, in.Seconds))
}

func (s *server) puzzleClick(
	ctx context.Context, _ *sdk.CallToolRequest, in clickIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	right := strings.EqualFold(in.Button, "right")
	return s.answer(ctx, true)(s.c.PuzzleClick(ctx, in.X, in.Y, right))
}

func (s *server) puzzleGiveUp(
	ctx context.Context, _ *sdk.CallToolRequest, _ struct{},
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx, true)(s.c.PuzzleGiveUp(ctx))
}

func (s *server) save(
	ctx context.Context, _ *sdk.CallToolRequest, in slotIn,
) (*sdk.CallToolResult, savedOut, error) {
	if err := s.c.Save(ctx, in.Slot); err != nil {
		return nil, savedOut{}, refusal(err)
	}
	return s.reply("Партия сохранена в слот " + strconv.Itoa(in.Slot) + "."),
		savedOut(in), nil
}

func (s *server) load(
	ctx context.Context, _ *sdk.CallToolRequest, in slotIn,
) (*sdk.CallToolResult, types.Outcome, error) {
	return s.answer(ctx, true)(s.c.Load(ctx, in.Slot))
}

// answer turns an outcome into the tool's reply: what happened in words, the
// outcome itself as structured content, and the puzzle's picture while one is
// on screen. acted is false for a wait, which cannot come to nothing.
func (s *server) answer(ctx context.Context, acted bool) func(
	types.Outcome, error,
) (*sdk.CallToolResult, types.Outcome, error) {
	return func(o types.Outcome, err error) (
		*sdk.CallToolResult, types.Outcome, error,
	) {
		if err != nil {
			return nil, types.Outcome{}, refusal(err)
		}
		res := s.reply(narrateOutcome(o, acted))
		if o.Look.Where == types.WherePuzzle {
			s.attachSight(ctx, res)
		}
		return res, o, nil
	}
}

// attachSight adds the picture of what is in view to a reply.
func (s *server) attachSight(ctx context.Context, res *sdk.CallToolResult) {
	png, err := s.c.Sight(ctx)
	if err != nil || len(png) == 0 {
		return
	}
	res.Content = append(res.Content,
		&sdk.ImageContent{Data: png, MIMEType: "image/png"})
}

// reply is a reply in words; the session's first one opens with the
// greeting.
func (s *server) reply(text string) *sdk.CallToolResult {
	if s.greeted.CompareAndSwap(false, true) {
		text = greeting + "\n\n" + text
	}
	return &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: text}},
	}
}

// refusal is an error as the client reads it: a sentence. Go keeps error
// texts lower case and unpunctuated.
func refusal(err error) error { return errors.New(sentence(err.Error())) }

// sentence capitalises a text and ends it with a full stop.
func sentence(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	s = string(unicode.ToUpper(r)) + s[n:]
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}
