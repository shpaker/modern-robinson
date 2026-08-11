# Новый Робинзон

Реверс-инжиниринг и открытый ремейк квеста «Новый Робинзон» (Никита, 1999): от
вскрытия форматов движка **NGI** до кросс-платформенного порта на **Go +
Ebitengine**. Игра управляется данными; форматы вскрыты (99.2 % записей), ремейк —
новый интерпретатор ресурсов.

> Ассеты игры проприетарны, в репозитории их нет. Нужна собственная копия игры.

## Структура

```
docs/       разбор форматов NGI (контейнер, кодеки, скрипты, движок)
tools/      Python-инструменты реверс-инжиниринга (ngiunpack.py)
remake/     прототип на Python (референс)
remake-go/  основной ремейк: Go + Ebitengine
todo.txt    план до полной игры
```

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

**3. Собрать и запустить** (нужен Go 1.24+ и [just](https://github.com/casey/just)):
```bash
cd remake-go && just build
./robinson /path/to/GAME_DIR
```
Либо положить `robinson` прямо в `GAME_DIR` и запустить без аргументов.

Кросс-сборки: `just build-macos|build-linux|build-windows|build-wasm`.

Управление: клик — идти, край экрана — сменить локацию, объект — осмотреть,
**F1** — отладка.

## Разработка

```bash
cd remake-go
just check   # fmt + lint + test
just demo    # headless-прогон -> PNG-кадры
```

Устройство — [remake-go/ARCHITECTURE.md](remake-go/ARCHITECTURE.md), форматы —
[docs/](docs/README.md).

## Ассеты

Код и инструменты — собственная работа. Графика, звук и тексты игры принадлежат
правообладателю (Никита) и не распространяются.
