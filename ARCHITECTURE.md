# Архитектура

Go + Ebitengine, Clean Architecture, модуль `github.com/shpaker/modern-robinson`.
Зависимости направлены внутрь; границы проверяет depguard (`.golangci.yml`).

```
internal/
  types/        Domain: сущности, без зависимостей
  interfaces/   контракты (только types)
  use_cases/    логика: изосетка, pathfinding (stateless)
  repositories/ доступ к файлам игры; codec/ — кодеки NGI
  adapters/     Ebiten-образы из Domain (анимации, звук)
  minigame/     шесть мини-игр, по пакету на игру; catalog/ — таблица по id
  app/          сборка графа, game loop, версия
cmd/main.go     игра
```

## Правила

- Ресурсы — только через репозитории; путей игры вне `app`/`repositories` нет.
- DI через `New*`; у реализаций — `var _ interfaces.X = (*Y)(nil)`.
- `types`/`interfaces`/`use_cases` не импортируют движок.
- gofumpt + golines (80). Тесты ядра; хелперы — в `testutil`; `t.Skip` без ресурсов.
- Коммиты — Conventional Commits (`feat:`, `fix:`, `refactor:`, …): по ним CI
  считает версию и пишет CHANGELOG.
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
