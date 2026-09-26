# Архитектура

Go + Ebitengine, Clean Architecture, модуль `github.com/shpaker/modern-robinson`.
Зависимости направлены внутрь; границы проверяет depguard (`.golangci.yml`).

```
internal/
  types/        Domain: сущности, без зависимостей
  interfaces/   контракты (только types)
  use_cases/    логика: изосетка, pathfinding (stateless)
  repositories/ доступ к файлам игры; codec/ — кодеки NGI; webfs/, webdata/ — веб
  adapters/     Ebiten-образы из Domain (анимации, звук); mouse/ — мышь мини-игр;
                mcp/ — герой для MCP-клиента (через interfaces.IControl)
  minigame/     шесть мини-игр, по пакету на игру; catalog/ — таблица по id
  app/          сборка графа, game loop, версия
cmd/main.go     игра
cmd/wasm/       браузерная сборка
cmd/minigames/  мини-игры без приключения (отладка, в релиз не идёт)
skills/         скилл robinson: игра через MCP (идёт в релиз)
```

## Правила

- Ресурсы — только через репозитории; путей игры вне `app`/`repositories` нет.
- DI через `New*`; у реализаций — `var _ interfaces.X = (*Y)(nil)`.
- `types`/`interfaces`/`use_cases` не импортируют движок.
- gofumpt + golines (80). Тесты ядра; хелперы — в `testutil`; `t.Skip` без ресурсов.
- Коммиты и заголовки PR — Conventional Commits (`feat:`, `fix:`, `refactor:`,
  …): по коммитам CI считает версию и пишет CHANGELOG.
- PR вливать merge-коммитом с пустым телом: `gh pr merge N --merge --body ''`
  (в вебе — стереть описание). Иначе GitHub кладёт в тело заголовок PR,
  и release-please пишет запись в CHANGELOG дважды.
- В коммитах и PR — только суть изменений: без `Co-Authored-By`, подписей
  и упоминаний инструментов, которыми сделана работа.
- Описание PR — крайне коротко: один список основных сделанных работ, другого
  форматирования нет.

## Форматы

`.COL` — Windows BGR (своп R↔B). Сетка: `screen = LeftTopGrid + gx·GridSize +
gy·GridShift`. Детали — [`../docs`](../docs).

## Проверка без окна

`just snapshot` / `just demo` гоняют игру headless (Ebiten `exp/vmhost`) и пишут
PNG-кадры.
