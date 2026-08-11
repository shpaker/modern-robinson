# Скриптовый язык `.FS` — полная спецификация

Составлено по всем 5140 `.FS`-скриптам игры (361 305 командных строк, 38 команд).
`.FS` лежат только в `.DAN` (метод `0x40` = LZSS, текст CP1251); в `.MV` их нет.

## Структура

Команда = строка, завершается `;`. Формат `Keyword arg1, arg2, ...;`. Первый
токен — команда, дальше аргументы через запятую (целые, строки в `"..."`,
идентификаторы, токены `ON`/`OFF`/`X`/`Y`/`Z`/`open`/`close`).

Ключевые слова и токены **регистронезависимы**; есть синонимы:
`DelObject`≡`DeleteObject`, `ShowCursor`≡`SHowCursor`, `EndIf`≡`Endif`.

Заголовок (ровно 4 строки, всегда в начале):

```
ScriptName   DO NOTE;        свободный ярлык, движком игнорируется
MovieName    Cannotdo.mv;    ролик .MV со спрайтами кадров
Shift        51,30;          привязка спрайта ролика на экране (X,Y)
TotalFrames  14;             число кадров (== числу блоков Frame)
```

Блок кадра:

```
Frame <index>,<eventCount>;   index = 0..TotalFrames-1; eventCount = число событий ниже
Delay <ms>;                   длительность кадра; Delay в eventCount НЕ входит
<событие 1>
...
```

**Парсер**: читать `Frame` → `Delay` → события до следующего `Frame`/`End`.
`eventCount` использовать как контроль, не как жёсткую длину (38 из 142 931
кадров битые — авторские правки). `Delay < 0` (68 скриптов) → `duration =
abs(Delay)`, знак помечает амбиентный/зациклённый кадр (волны, музыкальные
катсцены, статичные «маски» дверей).

## Команды (по частоте)

Покадровые: `Frame`(142931), `Delay`(142930), `Sound`(26208), `Text`(4976).
Заголовок: `ScriptName`/`MovieName`/`TotalFrames`/`End`/`Shift`(по ~5140).
Движение: `Shift`(3-арг,136), `Set`(1506), `Aproach`(2962).
Логика: `If`/`EndIf`(2750), `SetVar`(1837), `Set`(1506), `SetCharVar`(1162),
`AddVar`(19). Мир: `CreateObject`(1336), `DelObject`(1102), `DeleteItem`(431),
`AddItem`(246), `SetRest`(114), `SetVert`(84). Сцены: `GoScene`(1563),
`StartGame`(13), `EndGame`(1). UI/тумблеры: `SetMouse`(223), `SetMap`(90),
`LockBar`(70), `SetMusic`(25), `HideChar`(41), `ShowChar`(27), `ShiftScreen`(26),
`SetActive`(18), `ShowCursor`(10), `Interrupt`(8), `SetBar`(6), `ClearScreen`(3),
`ShowNav`(1).

### Синтаксис и семантика

| Команда | Форма | Смысл |
|---|---|---|
| `Sound` | `ID,ch` / `"file",ch` / `"file",ch,frames` / `ID,ch,frames` | звук; имя через `SoundVariables` сцены; 3-й арг — кадры лип-синка озвучки (`< TotalFrames`) |
| `Text` | `id,1` | субтитр по id (глобальная таблица; парная озвучка `rr<id>.wav`); 2-й арг всегда `1` |
| `Shift` | `char,X\|Y,±1` | инкремент клетки персонажа (walk-циклы) |
| `Set` | `char,X\|Y\|Z,int` | задать координату персонажа (Z — глубина/порядок) |
| `Aproach` | `char,obj,dx,dy` / `char,gx,gy` | подойти к объекту+смещение / к абсолютной клетке перед действием |
| `SetVar` | `name,int` | глобальный квест-флаг |
| `AddVar` | `name,int` | прибавить (счётчик) |
| `SetCharVar` | `name,"script"` | строковая переменная = имя следующего скрипта-варианта (автомат реплик) |
| `If … EndIf` | `var,value` | блок при `var == value`; else нет (эмулируется серией If); вложенность = И |
| `CreateObject`/`DelObject` | `scene,obj,gx,gy` / `scene,obj,char,dx,dy` | создать/убрать объект: абсолютная клетка / относительно персонажа |
| `AddItem` | `item` / `char,item` | в инвентарь |
| `DeleteItem` | `item` | из инвентаря |
| `SetRest` | `char,state,anim` | поза простоя после действия |
| `GoScene` | `scene,char,entry,gx,gy` / `scene,c1,e1,c2,e2,gx,gy` | переход: входной скрипт прибытия + клетка появления |
| `StartGame` | `gameId,resultVar,paramVar` | мини-игра; результат в `resultVar` → ветвление `If` |
| `SetVert` | `gx,gy,open\|close\|closed` | переключить проходимость клетки |
| `ShiftScreen` | `dx,dy` | прокрутка экрана |
| тумблеры | `ON`/`OFF` | `SetMouse`/`SetMap`/`LockBar`/`SetBar`/`ShowCursor`/`Interrupt` |
| `HideChar`/`ShowChar` | `Roby`/`Frid` | скрыть/показать персонажа |
| `SetActive` | `id` | активный инструмент/персонаж (`hand`, `stick`, `Roby`…) |
| `SetMusic` | `id`/`none` | сменить/остановить музыку |

**Координаты**: `Aproach`(3-арг) и `Create/DelObject`(4-арг) — абсолютные клетки;
`Aproach`(4-арг) и `Create/DelObject`(5-арг) — смещение от объекта/персонажа.
Сетка — из `.SCN`.

## FonScript vs действие

Различие задаёт вызывающий, не сам `.FS`:
- `.OB`: `FonScript <name>` — движок крутит `.FS` **циклически** (анимация
  объекта); `NULL` = статичный. FonScript-скрипты короткие (медиана 1 кадр;
  многокадровые — амбиент: `wave`, `fire`, `smoke`).
- Действия (по клику) — **однократные**, длинные (медиана 29 кадров); имя по
  конвенции `<RO|FR><HAN|AXE|CON…><объект>`.
- `.CHR` (`ROBY.CHR`): `MoveType NumPadGoing` (8 направлений), idle-скрипты
  (`head`/`ok0`/`roby1`), walk-циклы `rg_XY` (цифры = направления нумпада,
  список в `DO.LST`).

## Пример: подобрать топор (`ROHANAXE.FS`)

```
Frame 0,1;  Delay 200; Aproach Roby,axe,0,0;      подойти к клетке топора
Frame 6,1;  Delay 400; DelObject CAB_B1, axe,2,2; убрать объект
Frame 12,2; Delay 372; Sound "rs9011.wav",1,9;    озвучка (9 кадров лип-синка)
                       Text 1000,1;               субтитр
Frame 25,3; Delay 400; AddItem axe;               в инвентарь
                       SetVar Findaxe,1;           квест-флаг
                       Sound "rr093.wav",1;
End;
```

## Открытые вопросы

- Точный смысл `Delay < 0` (флаг цикла / синхронизация со звуком).
- `Sound` 3-й арг: длина лип-синка в кадрах vs позиция в ролике (в катсценах до 241).
- `StartGame` 3-й аргумент (предусловие или параметр игры).
