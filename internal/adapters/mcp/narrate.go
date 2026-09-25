package mcp

import (
	"strconv"
	"strings"

	"github.com/shpaker/modern-robinson/internal/types"
)

// narrate puts what the hero sees into a few plain sentences, the way he
// would take it in: the reply the client reads, in the first person, so that
// it speaks as him in turn.
func narrate(p types.Percept) string {
	var b strings.Builder
	line := func(s string) {
		if s != "" {
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}
	switch p.Where {
	case types.WherePause:
		line("Игра на паузе: открыто меню. Жду, пока его закроют.")
		return strings.TrimSpace(b.String())
	case types.WherePuzzle:
		line("Передо мной головоломка — она на картинке. Решаю её кликами: " +
			"puzzle_click (экран 640×480); бросить — puzzle_give_up.")
		return strings.TrimSpace(b.String())
	case types.WhereScene:
		line("Идёт заставка — надо подождать (wait).")
		return strings.TrimSpace(b.String())
	case types.WhereMap:
		line("Передо мной карта острова.")
	default:
		line("Я на острове.")
	}
	if p.Busy != "" {
		line("Сейчас " + p.Busy + ".")
	}
	if p.Where == types.WhereMap {
		line(listOf("Места", p.Around))
	} else {
		line(listOf("Вокруг меня", p.Around))
	}
	line(listOf("Отсюда можно уйти", p.Exits))
	switch {
	case p.EmptyHands:
		line("Руки свободны.")
	case p.Hands != "":
		line("В руках: " + p.Hands + ".")
	}
	switch {
	case len(p.Carry) > 0:
		line("С собой: " + strings.Join(p.Carry, ", ") + ".")
	case p.Hands != "":
		line("Больше с собой ничего.")
	}
	if f := p.Friday; f != nil {
		s := "Пятница рядом со мной"
		if f.Side != "" && f.Side != types.SideNear {
			s = "Пятница " + f.Side + " от меня"
		}
		if f.Hands != "" {
			s += ", у неё в руках: " + f.Hands
		}
		if len(f.Carry) > 0 {
			s += ", с собой: " + strings.Join(f.Carry, ", ")
		}
		line(s + ".")
	}
	if p.Map {
		line("Могу развернуть карту острова (map).")
	}
	if p.Hearing != "" {
		line("Звучит: " + p.Hearing)
	}
	return strings.TrimSpace(b.String())
}

// listOf is a heading and the things under it with their sides: "Вокруг
// меня: Пальма (слева), Краб (рядом)."
func listOf(head string, things []types.Thing) string {
	if len(things) == 0 {
		return ""
	}
	parts := make([]string, 0, len(things))
	for _, t := range things {
		if t.Side != "" {
			parts = append(parts, t.Name+" ("+t.Side+")")
		} else {
			parts = append(parts, t.Name)
		}
	}
	return head + ": " + strings.Join(parts, ", ") + "."
}

// narrateChanges tells what the hero notices has changed: the facts, for
// the client to feel something about.
func narrateChanges(ch types.Changes) string {
	var out []string
	add := func(head string, names []string) {
		if len(names) > 0 {
			out = append(out, head+": "+strings.Join(names, ", "))
		}
	}
	if ch.NewPlace {
		out = append(out, "я в новом месте")
	}
	add("теперь у меня", ch.Gained)
	add("больше нет", ch.Lost)
	add("появилось", ch.Appeared)
	add("пропало из виду", ch.Vanished)
	add("открылся путь", ch.Opened)
	add("закрылся путь", ch.Closed)
	if ch.FridayCame {
		out = append(out, "Пятница теперь со мной")
	}
	if ch.FridayLeft {
		out = append(out, "Пятницы рядом больше нет")
	}
	if ch.MapGained {
		out = append(out, "теперь у меня есть карта острова")
	}
	if ch.Misses >= 2 {
		out = append(out, "ничего не выходит уже "+
			strconv.Itoa(ch.Misses)+"-й раз подряд")
	}
	if len(out) == 0 {
		return ""
	}
	return "Изменилось:\n— " + strings.Join(out, "\n— ")
}

// narrateOutcome tells what came of an action, what changed, then what the
// hero sees now. A wait is no action: it has nothing to fail at.
func narrateOutcome(o types.Outcome, acted bool) string {
	var b strings.Builder
	if len(o.Said) > 0 {
		b.WriteString("Прозвучало:\n")
		for _, s := range o.Said {
			b.WriteString("— " + s + "\n")
		}
	} else if acted && !o.Reacted {
		b.WriteString("Ничего не произошло.\n")
	}
	if !o.Ready {
		b.WriteString("Всё ещё идёт — надо подождать (wait).\n")
	}
	if s := narrateChanges(o.Changes); s != "" {
		b.WriteString(s + "\n")
	}
	look := o.Look
	if n := len(o.Said); n > 0 && look.Hearing == o.Said[n-1] {
		look.Hearing = "" // just told
	}
	b.WriteString(narrate(look))
	return strings.TrimSpace(b.String())
}
