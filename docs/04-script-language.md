# Скриптовый язык NGI

Логика игры описана **текстовыми скриптами** (метод `0x40`, LZSS). Это делает
ремейк реалистичным: движок — интерпретатор этих скриптов, а весь геймплей,
диалоги, анимации и переходы заданы данными, а не зашиты в код.

Три вида скриптов:

- `.SCN` — **сцена** (экран/локация): сетка перемещения, список объектов, звуки.
- `.OB` — **объект** на сцене: кликабельная зона, курсор, фоновый скрипт.
- `.FS` — **кадровый скрипт** (frame script): покадровая анимация ролика/действия
  с событиями (звук, текст, смена состояния, ветвление).

Синтаксис общий: строки вида `Команда  аргументы;`, разделитель — `;`,
комментарии/отступы табами, конец — `End;`. Кодировка — CP1251 (русские тексты).

---

## `.SCN` — описание сцены

Пример (`SCENA0.SCN`, распакован):

```
SceneName       scena0;
ScreenName      scena0;
BarName         bar;
ScreenSize      1024,400;
ScrollPar       15,15
ScrollDesc      4,4
LeftTopGrid     15,150;
GridSize        144,36;
GridLength      8,5;
GridShift       46,-99;
ZPerGrid        8;
ClosedVert      4,2; 4,3; 5,2; 5,3; 6,3; ...
ClosedDir       2,3,3; 3,4,7; 2,4,9; 3,3,1;
ObjectList      palma,2,3,*; cc1,2,3,*; bgstone,2,3; clay,2,3; coco,2,3;
                fire,1,3; smoke,2,3; goleft,0,1,*; wave,0,4,*; crb,4,3,*; ...
SoundVariables  step,"step.wav",1; frstep,"step_pp.wav",1; ...
TextVariables   ...
Music           ...
End;
```

| Ключ | Смысл |
|---|---|
| `SceneName` / `ScreenName` | id сцены и фонового изображения |
| `BarName` | имя панели инвентаря (`bar`) |
| `ScreenSize` | размер сцены в пикселях (может быть шире экрана → скролл) |
| `ScrollPar` / `ScrollDesc` | параметры и скорость прокрутки |
| `LeftTopGrid`, `GridSize`, `GridLength`, `GridShift`, `ZPerGrid` | **изометрическая сетка** ходьбы: начало, размер ячейки, число ячеек по X/Y, сдвиг, шаг Z |
| `ClosedVert` | занятые (непроходимые) ячейки сетки |
| `ClosedDir` | запрещённые направления перехода между ячейками |
| `ObjectList` | объекты на сцене: `имя, gx, gy [, *]` (координаты в сетке; `*` — особый флаг) |
| `SoundVariables` | именованные звуки: `имя,"файл.wav",канал` |
| `TextVariables` | именованные строки текста |
| `Music` | фоновая музыка сцены |

Сетка задаёт изометрию: персонаж ходит по ячейкам, `Z` определяет порядок
перекрытия спрайтов.

---

## `.OB` — объект сцены

Пример (`BGSTONE.OB`):

```
ObjectName      bgstone;
FonScript       bgstone;
ZCoord          5;
ClosedVert
ClosedDir       0,0,3; 1,1,7;
ActiveZone      98,16,22,17;
Cursor          0;
Text            9;
End;
```

| Ключ | Смысл |
|---|---|
| `ObjectName` | id объекта (совпадает с именем в `ObjectList` сцены) |
| `FonScript` | фоновый `.FS`-скрипт (постоянная анимация объекта) |
| `ZCoord` | Z-порядок отрисовки |
| `ActiveZone` | прямоугольник клика `x, y, w, h` |
| `Cursor` | тип курсора над зоной (рука/глаз/…) |
| `Text` | id всплывающей подписи (индекс в таблице текста) |
| `ClosedVert`/`ClosedDir` | локальные ограничения проходимости |

---

## `.FS` — кадровый скрипт

Заголовок + список кадров с событиями. Пример (`DROVA.FS`):

```
ScriptName      DO NOTE;
MovieName       Cannotdo.mv;
Shift           51,30;
TotalFrames     14;

Frame 0,0;
Delay 142;

Frame 1,2;
Delay 160;
Sound "rr447.wav",1,12;
Text 447,1;
...
Frame 13,1;
Delay 210;
SetRest Roby,2,roby7a;

End;
```

### Словарь команд `.FS` (по частоте на всём образе)

| Команда | ~кол-во | Назначение |
|---|--:|---|
| `Frame gx,gy` | 28 777 | начало кадра, индекс кадра в ролике |
| `Delay ms` | 28 777 | длительность кадра |
| `Sound "file",ch,frame` | 5 382 | проиграть звук |
| `Text id,flag` | 922 | показать строку по id |
| `Shift x,y` | 1 179 | смещение спрайта ролика |
| `ScriptName` / `MovieName` / `TotalFrames` / `End` | по 1049 | шапка/конец |
| `Aproach` | 571 | подойти к точке/объекту перед действием |
| `Set` / `SetVar` / `SetCharVar` | 394/221/268 | установка игровых переменных |
| `CreateObject` / `DelObject` | 178/146 | создать/удалить объект сцены |
| `AddItem` / `DeleteItem` | 60/108 | инвентарь: добавить/убрать предмет |
| `GoScene` | 147 | переход на другую сцену |
| `SetRest char,state,anim` | 58 | задать «покой» персонажа (поза простоя) |
| `If … EndIf` | 34/35 | условное ветвление по переменным |
| `SetVert` | 31 | изменить проходимость сетки |
| `ShiftScreen` | 16 | сдвиг/скролл экрана |
| `SetMouse` / `HideChar` | 13/11 | управление курсором / скрыть персонажа |

Дополнительно в движке (`ROBY.EXE`) присутствуют: `Music`, `Map`, `Mouse`,
`SetBar`, `SetActive`, `GoScene`, `MouseZ`, `DelayFactor`, работа с
`IntVariables`/`CharVariables`/`SoundVariables`/`TextVariables`.

### Модель исполнения

- каждый скрипт привязан к ролику (`MovieName` → `.MV`), кадры скрипта
  синхронизируют кадры ролика с событиями;
- `Delay` задаёт тайминг; `Sound`/`Text` — события внутри кадра;
- переменные (`Set*`, `If`) образуют состояние прохождения (флаги квеста);
- `GoScene`, `AddItem`/`DeleteItem`, `CreateObject`/`DelObject` меняют мир;
- ошибки движка (строки в `ROBY.EXE`) подтверждают модель:
  `Error in script %s.fs in %d frame`, `Can't find scene "%s"`,
  `Can't find int variable "%s"`, `Can't find character "%s"`.

---

## `.SCR` — бинарный скрипт ролика

Внутри `.MV` есть один `.SCR` (метод `0x80`, LZHUF) — скомпилированная
раскадровка. Структура: заголовок `0x20` байт, затем покадровые `u32`-записи.

Заголовок (`ARRIVE.SCR`, 8×`u32`): `count`, `0`, `bbox`(190,200), `canvas`
(640×480), `origin`(393,293). Далее с `0x20` — пары `u32` `(субкадр, индекс)`,
по одной на кадр (аналог текстового `Frame idx,sub`). Для ремейка не критично:
порядок и число кадров восстанавливаются из имён `.NGB` и заголовков.

---

## Глобальные данные: `STARTUP.INF` и `TEXT.DAT`

`DATA/STARTUP.DAN` содержит два ключевых файла.

**`STARTUP.INF`** — «boot-файл» движка:

```
SceneDirectory  \SCEN;   MovieDirectory \MOVIE;   WaveDirectory \WAVE;
CharacterDirectory \CHAR;  BarDirectory \BAR;      Text text.dat;
Scenes          INT0,*;  INT1; … SHIP3;      (39 сцен, * = стартовая)
Characters      Frid, 7, 0, 0, *;  Roby, 7, 0, 4, *;
IntVariables    CrabNeed,0; … TreeIs,1; … MapParts,4; Find6,30; …
CharVariables   rohangol,"rohangol"; … rohanpop,"hirobin"; …
GridDebug 0;  DelayFactor 1;  End;
```

Важно: **не все флаги стартуют нулём** (`TreeIs=1`, `MapParts=4`, `Find6=30`),
поэтому без загрузки этого файла ранние проверки `If` уходят не в ту ветку.
`CharVariables` — указатели на «текущий вариант» реплики, их крутит `SetCharVar`.

**`TEXT.DAT`** — глобальная таблица строк (CP1251, 1202 строки). Идентификатор в
`Text <id>,1;` и в поле `Text` объекта `.OB` — это **номер строки, считая с нуля**.
Строки реплик записаны в кавычках, названия объектов — без. Проверка: у объекта
`palma` `Text 2`, строка 2 — `"Пальма"`; у `bgstone` `Text 9` — `"Большой камень"`.
