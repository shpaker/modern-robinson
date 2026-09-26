package mcp

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf8"
)

// Claude Code cuts a server's instructions and each tool's description at
// 2048 characters: they fit, and every tool stays in view from the start.
func TestInstructionsFitTheClient(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	instr := cs.InitializeResult().Instructions
	if n := utf8.RuneCountInString(instr); n > 2048 {
		t.Errorf("the instructions run to %d characters", n)
	}
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if n := utf8.RuneCountInString(tool.Description); n > 2048 {
			t.Errorf("%s: the description runs to %d characters",
				tool.Name, n)
		}
		if tool.Meta["anthropic/alwaysLoad"] != true {
			t.Errorf("%s is not in view from the start", tool.Name)
		}
	}
}

// A role file in a folder beside the game stands over the built-in one, the
// first folder over the next; the files it lacks come built in, and a file
// of its own is a role too. Only the roles there are are handed out.
func TestRolesComeFromTheFoldersBesideTheGame(t *testing.T) {
	game := fstest.MapFS{
		"instructions.md": {Data: []byte("свои инструкции\n")},
		"robinson.md":     {Data: []byte("# Свой Роби\n")},
	}
	exe := fstest.MapFS{
		"robinson.md": {Data: []byte("# Роби у бинарника\n")},
		"friday.md":   {Data: []byte("# Пятница у бинарника\n")},
		"pirate.md":   {Data: []byte("# Пират\n")},
	}
	cs := connectWith(t, &hero{look: beach}, newRoles(game, exe))
	if got := cs.InitializeResult().Instructions; got != "свои инструкции" {
		t.Errorf("instructions = %q", got)
	}
	robi := roleOf(t, cs, "robinson")
	for who, head := range map[string]string{
		"robinson": "# Свой Роби\n\n# Как идёт партия",
		"friday":   "# Пятница у бинарника\n\n",
		"narrator": "# Рассказчик\n",
		"pirate":   "# Пират\n\n",
	} {
		if got := roleOf(t, cs, who); !strings.HasPrefix(got, head) {
			t.Errorf("%s = %.60q…, want it to open with %q", who, got, head)
		}
	}
	for _, name := range shared {
		part, err := newRoles().file(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(robi, part) {
			t.Errorf("robinson lacks the built-in %s", name)
		}
	}
	names := "coordinator, friday, narrator, pirate, robinson"
	for _, who := range []string{
		"instructions", "world", "../robinson", "Роби", "",
	} {
		res := call(t, cs, "role", map[string]any{"who": who})
		if !res.IsError || !strings.Contains(text(res), names) {
			t.Errorf("role %q: %v %q", who, res.IsError, text(res))
		}
	}
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "role" &&
			!strings.Contains(tool.Description, "Есть: "+names+".") {
			t.Errorf("the role tool does not name them: %q",
				tool.Description)
		}
	}
}

// The roles carry the party's rules: the coordinator has no voice and puts
// the words out at once; the narrator is out of the heroes' hearing;
// Robinson decides, having heard Friday out; the scales are each hero's own;
// a puzzle is Robinson's to play; a hero low on the wish to be saved may
// refuse. Robinson is a quick, self-assured townsman who owns up when
// Friday's "why?" finds the hole; Friday is a wary sceptic who speaks
// Robinson's tongue badly, on purpose, but keeps the names and his scales
// straight.
func TestThePartyPlaysByTheRules(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	if got := strings.Join(newRoles().names(), ","); got !=
		"coordinator,friday,narrator,robinson" {
		t.Errorf("roles = %s", got)
	}
	lacks(t, "the coordinator's role", roleOf(t, cs, "coordinator"),
		"нет голоса", "сразу, как пришли, дословно", "одновременно",
		"без строки «Шкалы:»", "не прерываешь", "«делай»",
		"«Ход:» сам не исполняй", "Agent с name", "SendMessage",
	)
	lacks(t, "the shared rules", roleOf(t, cs, "friday"),
		"Решающий голос — у Роби", "герои его не слышат", "Решение:",
		"Ход:", "Шкалы:", "Игроку:", "к другу", "остров", "спасение",
		"Другой герой их не видит", "может отказаться от задачи игрока",
		"сам не пиши (SendMessage)",
	)
	lacks(t, "the narrator's role", roleOf(t, cs, "narrator"),
		"тебя не слышат", "«—»", "Не подсказывай",
	)
	lacks(t, "Robinson's role", roleOf(t, cs, "robinson"),
		"решающий голос — за тобой", "выслушай его", "решаешь её сам",
		"«делай»", "остров 3, спасение 9", "горожанин до мозга костей",
		"признаёшь и передумываешь", "переспроси",
	)
	lacks(t, "Friday's role", roleOf(t, cs, "friday"),
		"Решает Роби", "ask_friday", "осторожный скептик", "зачем?",
		"с ошибками", "«Шкалы:» пиши без ошибок",
		"Роби 3, остров 6, спасение 5",
	)
}

// The instructions and the roles say nothing of saving: the party neither
// saves nor loads of its own accord. Only the save files stay among those
// not to open.
func TestRolesSayNothingOfSaving(t *testing.T) {
	cs := connect(t, &hero{look: beach})
	texts := map[string]string{
		"the instructions": cs.InitializeResult().Instructions,
	}
	for _, who := range newRoles().names() {
		texts[who+"'s role"] = roleOf(t, cs, who)
	}
	for what, s := range texts {
		low := strings.ToLower(s)
		for _, w := range []string{
			"сохраняйся", "сохранись", "сохраниться", "слот", "save", "load",
		} {
			if strings.Contains(low, w) {
				t.Errorf("%s speaks of saving: %q", what, w)
			}
		}
	}
}
