# Устройство движка

Игра построена на движке **NGI (Nikita Game Interface)**. Исполняемые файлы —
32-битные PE (Windows, i386), рендер через DirectDraw/DirectX.

## Бинарники

| Файл | Размер | Роль |
|---|--:|---|
| `START.EXE` | 1.0 МБ | лаунчер (автозапуск из `AUTORUN.INF`), меню запуска/настроек |
| `ROBY.EXE` | 525 КБ | **основной движок игры**: интерпретатор сцен и `.FS`-скриптов |
| `DEMO.EXE` | 525 КБ | демо-режим (проигрывание роликов по `DEMO.DAT`) |
| `NGI32.DLL` | 338 КБ | **ядро NGI**: ресурсы (`ngiUnpack`), графика, DirectDraw, распаковка |
| `NAV32.DLL` | 34 КБ | навигация/поиск пути (`navCreateInstance`) |
| `MINIGAME.DLL` | 198 КБ | мини-игры (+ данные `MINIGAME.WDT/SDT`) |
| `DIALOG.DLL` | 160 КБ | диалоговые окна |

Игровой цикл: `ROBY.EXE` загружает `startup.dan`, `config.dat`, `options.dat`,
`logo.dat`, `wave.dan`, затем сцену (`SCENA*.DAN`) и исполняет её `.SCN` + `.OB`
+ `.FS`, подгружая ролики `.MV` и звук из `WAVE.DAN`.

## `NGI32.DLL` — ядро

Экспортирует ~134 функции `ngi*`. Ключевые группы:

**Ресурсы / распаковка**
- `ngiUnpack` (`0x235D0`) — распаковка записи ресурса (см.
  [02-compression.md](02-compression.md)).
- NL-загрузчик (`0x232EF`) — чтение файла, проверка магии, расшифровка директории
  (`0x2333B`), сборка рантайм-структуры `RsLi` (`0x233B9`).

**Графика (DirectDraw / собственный растеризатор)**
- `ngiGetDibInfo`, `ngiGetPalette`, `ngiSetPalette`, `ngiSetPaletteEx`
- `ngiFastBlt`, `ngiFlipToGDI`, `ngiIsBackSurface`, `ngiTuneDirectDraw`,
  `ngiGetDirectDrawPtrs`
- `ngiFreeBitmap`, `ngiFreeTexture`, `ngiFreeFadeTable` — управление ресурсами
  кадров, текстур и таблиц затемнения (`.FAD`)
- работа с «NGB»: `ngiGetNgbWidth`, `ngiGetNgbHeight`, `ngiGetNgbPixel`,
  `ngiSetNgbOrigin`, `ngiShiftNgbOrigin` — **прямое подтверждение**, что `NGB` —
  внутренний растровый формат движка со своими размерами и origin
- 3D/ускорение: `ngiBspTree`, `ngi3DHW`, `ngi3DNow`, `ngiMmx`, `ngiKatmai`
  (SSE), `ngiGetCpuFeatures` — оптимизации под процессоры конца 90-х

**Память / служебное**
- `ngiAlloc` (`0x24A00`), `ngiFree` (`0x24A30`), `ngiFixedMemMaxBlock`
- `ngiProcessError` — вывод `Nikita Game Interface Error`
- реестр: `Software\Nikita\NgiTool`

## Внутренний формат `RsLi`

На диске — `NL`; в памяти движок разворачивает ресурс в структуру `"RsLi"`
(`0x694C7352`): заголовок `0x38` байт + записи по `0x40` байт. `ngiUnpack`
принимает `(RsLi*, index)`. Для распаковки с диска знать `RsLi` не нужно (мы
читаем `NL` напрямую), но это объясняет двухслойность: имя ресурса → индекс в
`RsLi` → распаковка блока выбранным методом.

## Практические выводы для ремейка

1. **Геймплей — это данные.** Логика в текстовых `.SCN`/`.OB`/`.FS`; движок их
   интерпретирует. Значит, можно написать новый интерпретатор, не воспроизводя
   машинный код `ROBY.EXE`.
2. **Рендер простой.** 8-битные палитровые растры (`NGB`) + палитра (`COL`) +
   блиттинг спрайтов с прозрачностью и Z-порядком. Никакого 3D в геймплее
   (3D-функции — ускорение блиттинга).
3. **Звук простой.** Готовые PCM `.WAV`, канальная модель (`Sound file,ch,frame`).
4. **Навигация** — отдельный модуль `NAV32.DLL` (поиск пути по сетке сцены), его
   можно переписать по данным `.SCN` (сетка, `ClosedVert`, `ClosedDir`).
