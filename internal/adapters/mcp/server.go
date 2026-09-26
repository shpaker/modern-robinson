// Package mcp serves the hero over the Model Context Protocol: a client — a
// language model — plays Robinson himself. The server hands it data only:
// what the hero sees, holds and hears, and what changed (interfaces.IControl);
// every word said as the hero is the client's own. The game starts it with
// -mcp and talks over stdin/stdout.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// projectURL is where the remake lives.
const projectURL = "https://github.com/shpaker/modern-robinson"

// issuesURL is where bugs in the game or this server are reported.
const issuesURL = projectURL + "/issues"

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

` + thinkFirst + `

` + fairPlay + `

` + playerTask + `

Что приходит от игры (JSON):
- where — где ты: остров, карта острова, головоломка, заставка, пауза; ` +
	`busy — что сейчас идёт, пока действовать нельзя.
- around — что вокруг (name, side: слева, справа, рядом); exits — куда ` +
	`можно уйти; на карте острова around — это места.
- hands — что в руках (empty_hands — руки свободны), carry — что ещё с ` +
	`собой; friday — Пятница рядом и её вещи; map — карту можно развернуть.
- bar_locked — панель вещей заперта самой игрой: она ждёт, что ты ` +
	`применишь то, что в руках (use без item или с ней же), к чему-то ` +
	`вокруг или к себе; другую вещь и «Рука» до этого не взять.
- hearing — реплика на экране; said — все реплики, прозвучавшие за ` +
	`действие, твои и чужие.
- heard — что прозвучало в головоломке за действие, по порядку: ` +
	`взял, повернул, встало, не туда, победа и другие. Орган слышен ` +
	`нотами: название и октава (до4 — до первой октавы), глухо — без ` +
	`ноты; ария Пятницы — один звук: «ария:» и её ноты по порядку. ` +
	`Свистит он неточно: нота арии — ближайшая к свисту, а задумана ` +
	`могла быть на полтона выше или ниже.
- reacted — мир откликнулся на действие; ready — можно действовать ` +
	`дальше; look — что вокруг после действия.
- changes — что изменилось: new_place, gained и lost (вещи), appeared и ` +
	`vanished (вокруг), opened и closed (выходы), friday_came, friday_left, ` +
	`map_gained, misses — сколько действий подряд ни к чему не привели.

Инструменты:
- look — осмотреться; с image — ещё и картинка того, что в окне; с ` +
	`grid — картинка с сеткой координат через 40 px: x подписан сверху, ` +
	`y слева. grid есть и у puzzle_click, puzzle_move и wait; кликать по ` +
	`координатам можно только в головоломке. С region {x0,y0,x1,y1} look ` +
	`даёт крупный план этой части картинки (не больше 320×240), чтобы ` +
	`точнее прицелиться: координаты в сетке на нём — экранные, а region ` +
	`в ответе — что показано и во сколько раз (scale) увеличено.
- use — подойти и применить к чему-то вещь из рук; с item — сначала взять ` +
	`эту вещь. «Рука» — пустые руки: взять, потрогать, осмотреть, ` +
	`заговорить. target «себя» — сделать что-то с вещью самому: смотря ` +
	`по вещи, надеть её или положить рядом с собой.
- go — уйти через выход; на карте острова — отправиться в место.
- map — развернуть карту острова, когда она есть.
- ask_friday — попросить Пятницу применить её вещь к чему-то (или к себе), ` +
	`пока она рядом.
- wait — переждать сцену или просто подождать. Головоломка ждёт ` +
	`твоего хода: время в ней идёт, только пока идёт твой ход или wait, ` +
	`а между вызовами стоит; wait с seconds даёт ей идти столько секунд ` +
	`— так летит воздушный шар.
- puzzle_click, puzzle_give_up — головоломки решаются кликами по ` +
	`картинке (экран 640×480) или бросаются. Ответ на ход приходит, ` +
	`когда головоломка доиграла то, что он начал (мелодию, ход ` +
	`соперника), а после победы — когда она закрылась.
- puzzle_move — перенести деталь одним ходом, как мышью: клик в from, ` +
	`turns правых кликов и клик в to; если клик в from ничего не взял и ` +
	`не выделил, дальше не идёт. Взятая деталь висит на указателе ` +
	`серединой: to — место её середины. Кусок из нескольких частей висит ` +
	`на одной из них — его сначала возьми puzzle_click и посмотри.
- save и load — вне роли: служебное сохранение партии в слоты 0–11.

` + puzzleRules + `

Сохраняйся регулярно, не дожидаясь просьбы: после каждого успеха ` +
	`(в changes появилась вещь, новое место, открылся выход, решена ` +
	`головоломка) и перед тем, что может плохо кончиться. Чередуй слоты ` +
	`10 и 11, чтобы неудачное сохранение не отрезало путь назад; слоты ` +
	`0–9 — игрока, их без просьбы не трогай.

Не все пути открыты сразу: некоторые выходы появляются, только когда ` +
	`сделаешь что-то нужное — здесь или в другом месте. Если reacted=false ` +
	`и никто ничего не сказал — так не выйдет. Не бери наугад следующую ` +
	`вещь — вернись к тому, что видел и слышал, и подумай, чего не хватает. ` +
	`Имена вещей, мест и выходов передавай в точности как в данных. Ты ` +
	`знаешь только то, что видит и слышит герой: подсказок игра не даёт.

Проект: ` + projectURL + `. Перед первым ходом поприветствуй игрока ` +
	`своими словами, дай ему эту ссылку и пожелай хорошего выживания.

Ошибки игры и этого сервера — в issues проекта: ` + issuesURL + `. ` +
	`Заметил ошибку — скажи игроку и предложи завести issue; в описании ` +
	`обязательно платформа, версия игры и модель, которая играла.`

// thinkFirst is how the client plays: a thought before every action, as the
// user put it.
const thinkFirst = "Перед каждым действием модель пишет в чат мысль от лица " +
	"Роби: что вижу, что знаю, чего хочу и почему именно это. После ответа — " +
	"короткая реакция. Не перебирать вещи на всём подряд, а связывать " +
	"услышанное с увиденным. Если ничего не выходит, остановиться и " +
	"подумать, что упускаешь."

// fairPlay is what playing fair means: the world learnt from the game's
// answers alone, moves made through these tools alone, no guessing down a
// list — every action has its own reason, told in the chat before the call,
// and what came of it is told after.
const fairPlay = "Играй честно: мир ты узнаёшь только из ответов игры. " +
	"Не открывай и не разбирай её файлы — ресурсы, скрипты, сохранения — " +
	"и исходники ремейка, не ищи прохождения и подсказки ни в сети, ни в " +
	"файлах: подсказать может только сам игрок. В игре действуй только " +
	"инструментами этого сервера, без своих скриптов, циклов и обёрток " +
	"вокруг неё. Не перебирай вслепую — вещь за вещью на всём подряд, " +
	"клики наугад: перебор — не игра. Пробовать можно, но каждая попытка " +
	"— из своей догадки о том, что видел и слышал. У каждого действия " +
	"своя причина, не общая на серию и не заготовка: перед вызовом напиши " +
	"её в чат, а why — её короткий пересказ, не замена. После ответа " +
	"напиши в чат и следствие: что вышло и что ты из этого понял. С wait " +
	"в головоломке — так же."

// playerTask is what the client plays: the whole game or the piece the player
// asks for, with the player free to move at the window and to give hints.
const playerTask = "Делаешь то, о чём просит игрок: всю игру или кусок — " +
	"например, накормить и напоить Роби, решить головоломку, добраться до " +
	"какого-то места. Просто «сыграй» — вся игра. Сделал кусок — или " +
	"понял, что дальше без игрока не выйдет, — сохранись, скажи в чат, что " +
	"вышло, и жди новой просьбы. Игрок может между твоими ходами ходить и " +
	"сам, в окне игры: если он так делал, сначала осмотрись (look) — мир " +
	"мог измениться. Его подсказки словами бери в расчёт, а его партию из " +
	"слотов 0–9 загружай, когда попросит."

// puzzleRules is what the player knows of the puzzles: the game's manual
// retold, each game by the name it goes by on screen — the controls and the
// goal, and nothing the manual keeps back. A rule the remake does not keep
// yet (todo.md, stage 8) stays out until it does.
const puzzleRules = `Головоломки — что о них сказано в руководстве к ` +
	`игре. Дискета на экране или Esc (puzzle_give_up) — выйти без ` +
	`результата; сохраниться посреди головоломки нельзя.
- Хижина — мозаика: левый клик прилепляет деталь к указателю, правый ` +
	`поворачивает её на 90° по часовой, ещё один левый отпускает. Строят ` +
	`снизу: с крыши дом не начинают.
- Карта — тоже мозаика с теми же кликами. Верно приложенные куски ` +
	`притягиваются друг к другу в один фрагмент, а после выхода собранное ` +
	`снова перемешано.
- Записка — шифр: каждая закорючка — своя русская буква; подумай, кто, ` +
	`кому и в каком положении её написал. Букву сверху кликом берут и ` +
	`кликом ставят на закорючку — заменяются все такие же; клик по ` +
	`поставленной возвращает её отовсюду, левая нижняя кнопка — все буквы.
- Воздушный шар — перелететь на другой остров, ловя на разной высоте ` +
	`ветер в нужную сторону. Клик по Роби сбрасывает камень (шар чуть ` +
	`выше), по Пятнице — выпускает воздух (ниже); высота и курс — на ` +
	`приборах, острова и шар — на карте слева внизу. Долетел — значит ` +
	`прошёл над островом не выше 300 м.
- Мелодия на органе — сначала надо собрать все его части: пока не все ` +
	`окошки сверху заполнены, мотив не подобрать, остаётся выйти (Esc) и ` +
	`искать недостающие. Когда все на месте — подобрать мотив, который ` +
	`насвистел Пятница: предмет сверху кликом берут и кликом ставят в ` +
	`трубочку внизу; клик по маленькому туземцу с перьями — послушать, ` +
	`что вышло, а клик по Пятнице — он насвистит арию ещё раз.
- Шашки с пиратом — стаканы вместо шашек, бутылки вместо дамок. Клик ` +
	`по своему стакану обводит клетку под ним красной рамкой, клик по ` +
	`клетке — ход, если он по правилам. Бить обязательно; в серии взятий ` +
	`каждый прыжок — отдельно: первый puzzle_move, следующие puzzle_click ` +
	`по очередной клетке.`

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
	Why    string `json:"why"            jsonschema:"зачем это действие — коротко то, что перед вызовом написал в чат: чего хочешь добиться и почему именно так"`
}

type goIn struct {
	To  string `json:"to"  jsonschema:"куда: имя из exits, а на карте острова — место из around"`
	Why string `json:"why" jsonschema:"зачем это действие — коротко то, что перед вызовом написал в чат: чего хочешь добиться и почему именно так"`
}

type askIn struct {
	Target string `json:"target"         jsonschema:"к чему: имя из around или exits; «себя» — Пятница сама, без цели"`
	Item   string `json:"item,omitempty" jsonschema:"какую свою вещь ей взять; пусто — ту, что у неё в руках"`
	Why    string `json:"why"            jsonschema:"зачем это действие — коротко то, что перед вызовом написал в чат: чего хочешь добиться и почему именно так"`
}

// whyIn is an action that takes nothing but its reason.
type whyIn struct {
	Why string `json:"why" jsonschema:"зачем это действие — коротко то, что перед вызовом написал в чат: чего хочешь добиться и почему именно так"`
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
	Why    string `json:"why"              jsonschema:"зачем это действие — коротко то, что перед вызовом написал в чат: чего хочешь добиться и почему именно так"`
}

type moveIn struct {
	FromX int    `json:"from_x"          jsonschema:"x детали на экране головоломки, 0..639"`
	FromY int    `json:"from_y"          jsonschema:"y детали на экране головоломки, 0..479"`
	ToX   int    `json:"to_x"            jsonschema:"x, куда её положить (туда придёт её середина), 0..639"`
	ToY   int    `json:"to_y"            jsonschema:"y, куда её положить (туда придёт её середина), 0..479"`
	Turns int    `json:"turns,omitempty" jsonschema:"сколько раз повернуть её у цели правой кнопкой, 0..3"`
	Grid  bool   `json:"grid,omitempty"  jsonschema:"сетка координат через 40 px на картинке ответа"`
	Why   string `json:"why"             jsonschema:"зачем это действие — коротко то, что перед вызовом написал в чат: чего хочешь добиться и почему именно так"`
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
		Name:  "wait",
		Title: "Подождать",
		Description: "Переждать сцену или подождать столько секунд. " +
			"Головоломка между вызовами стоит, а wait даёт ей идти " +
			"столько секунд.",
	}, s.wait)
	sdk.AddTool(srv, &sdk.Tool{
		Name:  "puzzle_click",
		Title: "Клик в головоломке",
		Description: "Кликнуть по экрану головоломки 640×480; отвечает, " +
			"когда она доиграла начатое, картинкой и тем, что прозвучало " +
			"(heard).",
	}, s.puzzleClick)
	sdk.AddTool(srv, &sdk.Tool{
		Name:  "puzzle_move",
		Title: "Перенести в головоломке",
		Description: "Перенести деталь, как мышью: клик в from берёт её, " +
			"у to — turns правых кликов и клик, чтобы положить; если клик " +
			"в from ничего не взял и не выделил, дальше не идёт. Взятая " +
			"деталь висит на указателе серединой, кусок из нескольких " +
			"частей — на одной из них. Отвечает, когда головоломка " +
			"доиграла начатое, картинкой и тем, что прозвучало (heard).",
	}, s.puzzleMove)
	sdk.AddTool(srv, &sdk.Tool{
		Name:  "puzzle_give_up",
		Title: "Бросить головоломку",
		Description: "Оставить головоломку нерешённой, как по Esc, " +
			"когда она доиграла начатое.",
	}, s.puzzleGiveUp)
	sdk.AddTool(srv, &sdk.Tool{
		Name:  "save",
		Title: "Сохранить (вне роли)",
		Description: "Вне роли: сохранить партию в слот 0–11. Сохраняйся " +
			"после каждого успеха, чередуя слоты 10 и 11.",
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
	"why пуст: сначала напиши в чат, зачем это действие, и коротко повтори в why",
)

// reasoned refuses an action whose reason is blank: every action is meant to
// come after a thought written in the chat (thinkFirst, fairPlay). The reason
// stays with the client; the game never sees it.
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
