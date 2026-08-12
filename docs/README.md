# «Новый Робинзон» — реверс-инжиниринг ресурсов и движка

Рабочая документация проекта по **ремейку исполняемых файлов** игры
*Новый Робинзон* (студия **Никита**, 1999). Здесь собраны все находки по
структуре ресурсов, форматам файлов и техническому устройству игры.

## Что это за игра

- **Название:** Новый Робинзон (New Robinson)
- **Разработчик / издатель:** Никита (`DATA.TAG`: `Company=Nikita`,
  `Application=New Robinson`, `Version=1.00.000`, `Category=Games`, `Misc=Russian`)
- **Год:** 1999 (даты файлов на диске — ноябрь–декабрь 1999)
- **Платформа:** Windows 9x/NT, DirectX, разрешение экранов 640×480 и 1024×400
- **Жанр:** квест / point-and-click с изометрическими сценами и рисованной анимацией
- **Движок:** **NGI — Nikita Game Interface** (реестр `Software\Nikita\NgiTool`,
  строка `Nikita Game Interface Error` в `NGI32.DLL`)

Источник данных: образ диска `ROBINSON.iso`, распакованный в папку.

## Статус реверса

| Слой | Статус | Где описано |
|---|---|---|
| Формат контейнера `NL` (заголовок + директория) | ✅ вскрыт | [01-container-format.md](01-container-format.md) |
| Шифрование директории (потоковый шифр) | ✅ вскрыт | [01-container-format.md](01-container-format.md) |
| Кодек `0x00` — raw (звук WAV) | ✅ реализован | [02-compression.md](02-compression.md) |
| Кодек `0x40` — LZSS (текстовые скрипты) | ✅ реализован | [02-compression.md](02-compression.md) |
| Кодек `0x80` — LZHUF (графика, палитры, ~75%) | ✅ реализован | [02-compression.md](02-compression.md) |
| Кодек `0x100` — raw DEFLATE (227 записей) | ✅ реализован | [02-compression.md](02-compression.md) |
| Формат спрайта `.NGB` (оба подтипа) | ✅ вскрыт | [03-resource-types.md](03-resource-types.md) |
| Скриптовый язык (`.FS`/`.SCN`/`.OB`) | ✅ читается | [04-script-language.md](04-script-language.md) |
| Формат бинарного скрипта `.SCR` | 🟡 разобран структурно | [04-script-language.md](04-script-language.md) |
| Формат сейва `.SAV`/`.BGI` | ✅ вскрыт (стартовое состояние) | [03-resource-types.md](03-resource-types.md) |
| Таблица затемнения `.FAD` | ✅ вскрыта | [03-resource-types.md](03-resource-types.md) |
| Глобальные данные (`STARTUP.INF`, `TEXT.DAT`) | ✅ вскрыты | [04-script-language.md](04-script-language.md) |
| Панель инвентаря (`BAR.BAR` + `BAR.DAT`) | ✅ вскрыта | [03-resource-types.md](03-resource-types.md) |
| Персонажи (`.CHR`, `DO.LST`, циклы ходьбы) | ✅ вскрыты | [09-characters.md](09-characters.md) |
| Мини-игры (ассеты и раскладки) | ✅ вскрыты | [10-minigames.md](10-minigames.md) |
| Мини-игры (правила в `MINIGAME.DLL`) | 🟡 частично | [10-minigames.md](10-minigames.md) |

**Декодируемость: 100 %** всех записей (все четыре кодека реализованы:
`0x00` raw, `0x40` LZSS, `0x80` LZHUF, `0x100` DEFLATE).

## Ремейк

Движок на Go + Ebitengine — в корне репозитория ([`../README.md`](../README.md),
[`../ARCHITECTURE.md`](../ARCHITECTURE.md)). Работает в родном окне 640×480
(viewport 640×400 со скроллом + панель 80 px): рендер сцен и анимаций, ходьба
по авторским циклам, интерпретатор квеста, инвентарь, тексты, сохранения,
опции, музыка, катсцены, оба персонажа. Это подтверждает главный вывод:
геймплей задан данными, и ремейк сводится к интерпретатору ресурсов.

## Инструментарий

Всё в [`../tools/`](../tools), чистый Python без сторонних зависимостей:

- `ngi.py` — библиотека: парсинг контейнера, расшифровка директории, все кодеки.
- `lzhuf.py` — декомпрессор LZHUF (кодек `0x80`).
- `ngiunpack.py` — CLI: `list` / `info` / `extract` / `scan`.

```bash
python tools/ngiunpack.py info    extracted/ROBINSON_ISO/ROBINSON/DATA/SCEN/SCENA0.DAN
python tools/ngiunpack.py list    <file.dan|.dat|.mv>
python tools/ngiunpack.py extract <file> <outdir> ['*.WAV']
python tools/ngiunpack.py scan    <root>          # инвентарь всех NL-файлов
```

## Карта документации

- [01-container-format.md](01-container-format.md) — контейнер `NL`, заголовок, шифр директории.
- [02-compression.md](02-compression.md) — все четыре кодека `ngiUnpack`.
- [03-resource-types.md](03-resource-types.md) — типы записей: `NGB`, `COL`, `WAV`, `FAD`, `CHR`, `BGI`, панель.
- [04-script-language.md](04-script-language.md) — язык сцен и кадровых скриптов, глобальные данные.
- [05-engine.md](05-engine.md) — устройство движка, DLL, ключевые адреса в `NGI32.DLL`.
- [06-remake-roadmap.md](06-remake-roadmap.md) — что осталось и план ремейка.
- [07-fs-scripting.md](07-fs-scripting.md) — полная спецификация языка `.FS` (38 команд).
- [08-scene-objects.md](08-scene-objects.md) — объекты сцены: позиционирование, Z-порядок, курсоры.
- [09-characters.md](09-characters.md) — персонажи: `.CHR`, `DO.LST`, циклы ходьбы, простои, Пятница.
- [10-minigames.md](10-minigames.md) — шесть мини-игр: контейнеры, раскладки, что реализовано.
- [assets/inventory.md](assets/inventory.md) — полный инвентарь 736 контейнеров.

> Все адреса функций и смещения даны для конкретных файлов с диска и
> проверены дизассемблированием `NGI32.DLL` и `MINIGAME.DLL` (ImageBase
> `0x10000000`).

## Что ещё не разобрано

Список открытых вопросов, чтобы не искать заново:

- **Шашки**: превращение в дамку и алгоритм противника (`MINIGAME.DLL`,
  `0x100058F0 → 0x100065B0`). Ремейк играет упрощённо (жадный поиск).
- **Воздушный шар**: точная экранная проекция островов (замкнутая форма).
- **`.SCR`**: смысл полей заголовка помимо размера холста (`0x10/0x14`) и
  origin (`0x18/0x1C`).
- **`Delay < 0`** в кадровых скриптах и третий аргумент `Sound` (кадры
  лип-синка или позиция в ролике).
- Разбор `.SCN`: имя объекта, совпадающее с ключевым словом скрипта (`map`,
  `sound`), различается по тому, что запятая приклеена к нему без пробела.
  Проверено на 365k конструкций, но признак эмпирический — в движке может быть
  явный признак секции.
- Байт `+0xA68` объекта (гейтит освобождение NGB после отрисовки; вероятно
  «оставить декаль резидентной»).

Геометрия сцены полностью вскрыта — см. [08-scene-objects.md](08-scene-objects.md).
