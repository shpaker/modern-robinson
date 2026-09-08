# Сторонний код и лицензии

Собственный код репозитория пока не сопровождается лицензией. Ниже — сторонние
компоненты, которые входят в репозиторий или в собранный бинарник.

## Скопированный код

- `.vmdriver/main.go`, `.vmdriver-game/main.go` — производные от кода
  [Ebitengine](https://github.com/hajimehoshi/ebiten), © The Ebitengine Authors,
  Apache License 2.0. Заголовки лицензии сохранены в файлах.

## Зависимости, статически линкуемые в бинарник

| Модуль | Лицензия |
|---|---|
| github.com/hajimehoshi/ebiten/v2 | Apache-2.0 |
| github.com/hajimehoshi/bitmapfont/v4 | Apache-2.0 (глифы — см. LICENSE модуля) |
| github.com/ebitengine/oto/v3 | Apache-2.0 |
| github.com/ebitengine/purego | Apache-2.0 |
| github.com/ebitengine/gomobile | BSD-3-Clause |
| github.com/ebitengine/hideconsole | Apache-2.0 |
| github.com/go-text/typesetting | Unlicense / BSD-3-Clause |
| github.com/jfreymuth/pulse | MIT |
| github.com/pierrec/lz4/v4 | BSD-3-Clause |
| github.com/srwiley/rasterx | BSD-3-Clause |
| golang.org/x/{image,sync,sys,text} | BSD-3-Clause |
| Go runtime и стандартная библиотека | BSD-3-Clause |

Тексты лицензий: Apache-2.0 — <https://www.apache.org/licenses/LICENSE-2.0>,
остальные — в репозиториях соответствующих модулей (`go mod download` кладёт их
в `$GOMODCACHE`).

## Не входит в репозиторий

Графика, звук, тексты и шрифты игры «Новый Робинзон» принадлежат правообладателю
(«Никита») и не распространяются вместе с этим кодом — см. раздел «Ассеты» в README.
