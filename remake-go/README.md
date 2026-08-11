# Новый Робинзон — Go/Ebiten

Ремейк квеста «Новый Робинзон» (Никита, 1999). Один бинарник кладётся в папку
игры (с `DATA/`, `*.MV`, `*.DAN`) и читает ресурсы оттуда. Сборки под
macOS / Linux / Windows / WASM.

## Команды

```bash
just build     # -> ./robinson
just dev       # запустить из исходников
just test      # тесты (ядро; golden-сверка кодеков)
just check     # fmt + lint + test
just demo      # headless-прогон -> PNG-кадры
```

Запуск: `./robinson [папка-игры]` — папка, где есть `DATA/WAVE/WAVE.DAN`.

## Управление

Клик — идти; клик по краю — сменить локацию; клик по объекту — осмотреть;
**F1** — отладка (сетка, хотспоты, путь, HUD).

## Устройство

Clean Architecture — [ARCHITECTURE.md](ARCHITECTURE.md). Форматы игры —
[`../docs`](../docs). Проверка без окна — Ebiten `exp/vmhost` (в go.mod
закреплён коммит `95dde051e`).
