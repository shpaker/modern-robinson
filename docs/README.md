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

Источник образа: раздача old-games.ru (`old-games.nfo`), архив
`Noviy_Robinson_ISO.rar` → `ROBINSON.iso`.

## Статус реверса

| Слой | Статус | Где описано |
|---|---|---|
| Формат контейнера `NL` (заголовок + директория) | ✅ вскрыт | [01-container-format.md](01-container-format.md) |
| Шифрование директории (потоковый шифр) | ✅ вскрыт | [01-container-format.md](01-container-format.md) |
| Кодек `0x00` — raw (звук WAV) | ✅ реализован | [02-compression.md](02-compression.md) |
| Кодек `0x40` — LZSS (текстовые скрипты) | ✅ реализован | [02-compression.md](02-compression.md) |
| Кодек `0x80` — LZHUF (графика, палитры, ~75%) | ✅ реализован | [02-compression.md](02-compression.md) |
| Кодек `0x100` — графический вариант (227 записей) | ⏳ не разобран | [02-compression.md](02-compression.md) |
| Формат спрайта `.NGB` (внутренний RLE) | 🟡 частично | [03-resource-types.md](03-resource-types.md) |
| Скриптовый язык (`.FS`/`.SCN`/`.OB`) | ✅ читается | [04-script-language.md](04-script-language.md) |
| Формат бинарного скрипта `.SCR` | ⏳ не разобран | [04-script-language.md](04-script-language.md) |
| Формат сейва `.SAV`/`.BGI` | 🟡 частично | [03-resource-types.md](03-resource-types.md) |

**Декодируемость сейчас: 99.2 %** всех записей (27 826 из 28 053).
Проверено массово: 2058 записей из 60 случайных контейнеров распаковались
без единой ошибки, длины совпали с заявленными.

## Ремейк

Движок на Go + Ebitengine — в корне репозитория ([`../README.md`](../README.md),
[`../ARCHITECTURE.md`](../ARCHITECTURE.md)): рендер родных сцен, ходьба по
изосетке, кликабельные объекты, переходы между локациями. Это подтверждает
главный вывод: геймплей задан данными, и ремейк сводится к интерпретатору
ресурсов.

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
- [02-compression.md](02-compression.md) — три кодека `ngiUnpack` + нерешённый `0x100`.
- [03-resource-types.md](03-resource-types.md) — типы записей: `NGB`, `COL`, `WAV`, `FAD`, `CHR`, сейвы.
- [04-script-language.md](04-script-language.md) — язык сцен и кадровых скриптов.
- [05-engine.md](05-engine.md) — устройство движка, DLL, ключевые адреса в `NGI32.DLL`.
- [06-remake-roadmap.md](06-remake-roadmap.md) — что осталось и план ремейка.
- [assets/inventory.md](assets/inventory.md) — полный инвентарь 736 контейнеров.

> Все адреса функций и смещения даны для конкретных файлов с диска и
> проверены дизассемблированием `NGI32.DLL` (ImageBase `0x10000000`).
