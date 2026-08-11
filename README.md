# Новый Робинзон

Реверс-инжиниринг и открытый ремейк квеста «Новый Робинзон» (Никита, 1999) на
**Go + Ebitengine**. Игра управляется данными; форматы вскрыты (99.2 % записей),
ремейк — интерпретатор ресурсов. Вспомогательные инструменты реверса — на Python
в [`tools/`](tools).

> Ассеты игры проприетарны, в репозитории их нет. Нужна собственная копия игры.

## Как запустить

Ресурсы читаются из папки игры напрямую, конвертировать ничего не нужно.

**1. Образ из архива:**
```bash
unar Noviy_Robinson_ISO.rar        # -> ROBINSON.iso
```

**2. Извлечь образ в папку:**
```bash
# macOS
unar ROBINSON.iso                  # -> ROBINSON/   (или hdiutil attach ROBINSON.iso)
# Linux
7z x ROBINSON.iso -oROBINSON       # -> ROBINSON/   (или sudo mount -o loop)
```
Итог — папка (`GAME_DIR`), где есть `DATA/WAVE/WAVE.DAN`.

**3. Собрать и запустить** (Go 1.24+, [just](https://github.com/casey/just)):
```bash
just build
./robinson /path/to/GAME_DIR
```
Либо положить `robinson` прямо в `GAME_DIR` и запустить без аргументов.

Кросс-сборки: `just build-macos|build-linux|build-windows|build-wasm`.

Управление: клик — идти, край экрана — сменить локацию, объект — осмотреть,
**F1** — отладка.

## Разработка

```bash
just check   # fmt + lint + test
just demo    # headless-прогон -> PNG-кадры
```

Устройство — [ARCHITECTURE.md](ARCHITECTURE.md), форматы игры — [docs/](docs/README.md),
инструменты реверса — [tools/](tools). План — [todo.md](todo.md).

## Ассеты

Код и инструменты — собственная работа. Графика, звук и тексты игры принадлежат
правообладателю (Никита) и не распространяются.
