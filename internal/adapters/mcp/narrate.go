package mcp

import (
	"strings"

	"github.com/shpaker/modern-robinson/internal/types"
)

// narrate puts what the hero sees into a few plain sentences: the reply the
// client reads, in the second person.
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
		line("Игра на паузе: открыто меню. Подожди, пока его закроют.")
		return strings.TrimSpace(b.String())
	case types.WherePuzzle:
		line("Перед тобой головоломка — она на картинке. Кликай по ней: " +
			"puzzle_click (экран 640×480); бросить — puzzle_give_up.")
		return strings.TrimSpace(b.String())
	case types.WhereScene:
		line("Идёт заставка — подожди (wait).")
		return strings.TrimSpace(b.String())
	case types.WhereMap:
		line("Перед тобой карта острова.")
	default:
		line("Ты на острове.")
	}
	if p.Busy != "" {
		line("Сейчас " + p.Busy + ".")
	}
	around := "Вокруг"
	if p.Where == types.WhereMap {
		around = "Места"
	}
	line(listOf(around, p.Around))
	line(listOf("Выходы", p.Exits))
	if p.Hands != "" {
		line("В руках: " + p.Hands + ".")
	}
	if len(p.Carry) > 0 {
		line("С собой: " + strings.Join(p.Carry, ", ") + ".")
	} else if p.Hands != "" {
		line("Больше с собой ничего.")
	}
	if f := p.Friday; f != nil {
		s := "Пятница рядом"
		if f.Side != "" && f.Side != types.SideNear {
			s = "Пятница " + f.Side
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
		line("Карту острова можно развернуть (map).")
	}
	if p.Hearing != "" {
		line("На экране: " + p.Hearing)
	}
	return strings.TrimSpace(b.String())
}

// listOf is a heading and the things under it with their sides: "Вокруг:
// Пальма (слева), Краб (рядом)."
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

// narrateOutcome tells what came of an action, then what the hero sees now.
// A wait is no action: it has nothing to fail at.
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
		b.WriteString("Всё ещё идёт — подожди (wait).\n")
	}
	look := o.Look
	if n := len(o.Said); n > 0 && look.Hearing == o.Said[n-1] {
		look.Hearing = "" // just told
	}
	b.WriteString(narrate(look))
	return strings.TrimSpace(b.String())
}
