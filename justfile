# Новый Робинзон — Go/Ebiten remake
gocmd := "go"
binary_name := "robinson"
module := "github.com/shpaker/modern-robinson"
max_line_length := "80"

# Список команд
default:
    @just --list

# Собрать бинарник под текущую ОС
build:
    #!/usr/bin/env bash
    set -euo pipefail
    VERSION="dev-$(date -u +%Y-%m-%dT%H:%M)"
    echo "Building {{binary_name}} ($VERSION)..."
    {{gocmd}} build -ldflags "-X {{module}}/internal/app.Version=${VERSION}" -o {{binary_name}} ./cmd
    echo "Built: {{binary_name}}"

# Кросс-сборки в _build/
build-macos:
    #!/usr/bin/env bash
    set -euo pipefail
    VERSION="dev-$(date -u +%Y-%m-%dT%H:%M)"; out=_build/macos; mkdir -p "$out"
    GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 {{gocmd}} build -trimpath \
        -ldflags "-s -w -X {{module}}/internal/app.Version=${VERSION}" \
        -o "$out/{{binary_name}}_darwin_arm64" ./cmd
    echo "macOS build -> $out"

build-linux:
    #!/usr/bin/env bash
    set -euo pipefail
    VERSION="dev-$(date -u +%Y-%m-%dT%H:%M)"; out=_build/linux; mkdir -p "$out"
    GOOS=linux GOARCH=amd64 {{gocmd}} build -trimpath \
        -ldflags "-s -w -X {{module}}/internal/app.Version=${VERSION}" \
        -o "$out/{{binary_name}}_linux_amd64" ./cmd
    echo "Linux build -> $out"

build-windows:
    #!/usr/bin/env bash
    set -euo pipefail
    VERSION="dev-$(date -u +%Y-%m-%dT%H:%M)"; out=_build/windows; mkdir -p "$out"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 {{gocmd}} build -trimpath \
        -ldflags "-s -w -X {{module}}/internal/app.Version=${VERSION}" \
        -o "$out/{{binary_name}}_windows_amd64.exe" ./cmd
    echo "Windows build -> $out"

build-all: build-macos build-linux build-windows

# --- браузерная версия -------------------------------------------------------
# Собрать оболочку в dist/web (wasm + wasm_exec.js + web/ + иконка игры)
build-wasm dir="extracted/ROBINSON_ISO/ROBINSON":
    #!/usr/bin/env bash
    set -euo pipefail
    VERSION="dev-$(date -u +%Y-%m-%dT%H:%M)"; out=dist/web; mkdir -p "$out"
    GOOS=js GOARCH=wasm {{gocmd}} build -trimpath \
        -ldflags "-s -w -X {{module}}/internal/app.Version=${VERSION}" \
        -o "$out/robinson.wasm" ./cmd/wasm
    cp "$({{gocmd}} env GOROOT)/lib/wasm/wasm_exec.js" "$out/"
    cp web/style.css web/sw.js "$out/"
    # Страница ссылается на движок, wasm_exec.js и стили с ?v=<хеш сборки>:
    # новая сборка — новые адреса, и браузер не подставит закешированную старую.
    build=$(cat "$out/robinson.wasm" "$out/wasm_exec.js" "$out/style.css" \
        | shasum -a 256 | cut -c1-12)
    sed "s/__BUILD__/$build/g" web/index.html > "$out/index.html"
    # Иконка — игровой ассет, в репозитории её нет: достаём из папки игры.
    # Без неё сборка не падает, страница просто останется без фавикона.
    if [ -f "{{dir}}/START.ICO" ]; then
        {{gocmd}} run ./tools/webicon -out "$out" "{{dir}}"
    else
        echo "warning: нет {{dir}}/START.ICO — оболочка без фавикона" >&2
    fi
    # .gz рядом с файлом: хост данных отдаёт его готовым (Content-Encoding:
    # gzip), и 24-МБ wasm уезжает семью. Жать на каждый запрос расточительно.
    for f in "$out"/*.wasm "$out"/*.js "$out"/*.html "$out"/*.css; do
        gzip -9 -c "$f" > "$f.gz"
    done
    raw=$(du -h "$out/robinson.wasm" | cut -f1)
    gz=$(du -h "$out/robinson.wasm.gz" | cut -f1)
    echo "Web shell -> $out (wasm $raw, по проводу $gz)"

# Упаковать ресурсы игры для веба в dist/webdata/v1
web-data dir="extracted/ROBINSON_ISO/ROBINSON" version="1":
    {{gocmd}} run ./tools/packweb -out dist/webdata/v{{version}} \
        -version {{version}} {{dir}}

# Локальный прогон: оболочка и данные на одном порту, как в проде
web-serve: build-wasm
    {{gocmd}} run ./tools/webserve

# Выложить всё: оболочку и ресурсы. Куда — аргументами или из окружения:
# ROBINSON_HOST (user@host для rsync) и ROBINSON_PATH (каталог сайта).
web-push host=env_var_or_default("ROBINSON_HOST", "") path=env_var_or_default("ROBINSON_PATH", ""): \
    (web-push-shell host path) (web-push-data host path)

# Выложить только оболочку — это делается на каждую сборку
web-push-shell host=env_var_or_default("ROBINSON_HOST", "") path=env_var_or_default("ROBINSON_PATH", ""):
    #!/usr/bin/env bash
    set -euo pipefail
    test -n "{{host}}" -a -n "{{path}}" || { echo "задайте ROBINSON_HOST и ROBINSON_PATH"; exit 1; }
    src="dist/web/"
    test -f "$src/robinson.wasm" || { echo "нет $src — сначала just build-wasm"; exit 1; }
    echo "Оболочка $(du -sh "$src" | cut -f1) -> {{host}}:{{path}}/"
    # Без --delete: в {{path}} лежат ещё и каталоги ресурсов /vN/.
    # Без --info=progress2: в macOS rsync — openrsync (совместимость с 2.6.9),
    # он такой опции не знает и молча печатает usage.
    rsync -a --partial -v "$src" "{{host}}:{{path}}/"

# Выложить ресурсы игры — редкая операция, ~336 МБ
web-push-data host=env_var_or_default("ROBINSON_HOST", "") path=env_var_or_default("ROBINSON_PATH", "") version="1":
    #!/usr/bin/env bash
    set -euo pipefail
    test -n "{{host}}" -a -n "{{path}}" || { echo "задайте ROBINSON_HOST и ROBINSON_PATH"; exit 1; }
    src="dist/webdata/v{{version}}/"
    test -f "$src/manifest.json" || { echo "нет $src — сначала just web-data"; exit 1; }
    echo "Ресурсы $(du -sh "$src" | cut -f1) -> {{host}}:{{path}}/v{{version}}/"
    rsync -a --delete --partial -v "$src" "{{host}}:{{path}}/v{{version}}/"

# Собрать архив для раздачи, как в релизе: бинарники всех систем, запускалку,
# README, образец настроек, AGENTS.md и .mcp.json для папки игры и скилл
release version="dev":
    #!/usr/bin/env bash
    set -euo pipefail
    out=_build/release; top="modern-robinson_{{version}}"; stage="$out/$top"
    rm -rf "$out"; mkdir -p "$stage/skills"
    for target in darwin:arm64:1:robinson_darwin_arm64 \
                  linux:amd64:0:robinson_linux_amd64 \
                  windows:amd64:0:robinson.exe; do
        IFS=: read -r goos goarch cgo binary <<<"$target"
        GOOS="$goos" GOARCH="$goarch" CGO_ENABLED="$cgo" {{gocmd}} build -trimpath \
            -ldflags "-s -w -X {{module}}/internal/app.Version={{version}}" \
            -o "$stage/$binary" ./cmd
    done
    cp README.md THIRD_PARTY.md packaging/config.yml packaging/.mcp.json "$stage/"
    install -m 0755 packaging/robinson "$stage/"
    cp packaging/AGENTS.md "$stage/AGENTS.md"
    cp -R skills/robinson "$stage/skills/"
    ( cd "$out" && zip -qrX "$top.zip" "$top" -x '*.DS_Store' )
    rm -rf "$stage"
    ls -la "$out"
    echo "Один архив на все системы: всё из него, и .mcp.json тоже, положить рядом с DATA/ игры, см. README.md."


# Запустить (нужна папка игры рядом или путём аргументом)
run: build
    ./{{binary_name}}

# Мини-игры без приключения (отладка): меню, или сразу игра: just minigames 4
minigames id="":
    {{gocmd}} run ./cmd/minigames {{ if id != "" { "-game " + id } else { "" } }}

# Запустить без сборки бинарника
dev:
    {{gocmd}} run ./cmd

# Тесты
test:
    {{gocmd}} test ./...

test-coverage:
    #!/usr/bin/env bash
    {{gocmd}} test -coverprofile=coverage.out ./...
    {{gocmd}} tool cover -html=coverage.out -o coverage.html
    echo "coverage.html"

# Порог покрытия ядра (кодеки + use_cases)
test-coverage-check:
    #!/usr/bin/env bash
    set -euo pipefail
    threshold=70
    {{gocmd}} test -coverprofile=cov-core.out ./internal/use_cases/... ./internal/repositories/...
    total=$({{gocmd}} tool cover -func=cov-core.out | awk '/^total:/ {gsub("%","",$3); print $3}')
    rm -f cov-core.out
    echo "core coverage: ${total}% (threshold ${threshold}%)"
    awk -v t="$total" -v thr="$threshold" 'BEGIN { exit (t < thr) ? 1 : 0 }'

# Форматирование (gofumpt + golines)
fmt:
    #!/usr/bin/env bash
    GOBIN="$({{gocmd}} env GOPATH)/bin"
    "$GOBIN/gofumpt" -l -w .
    "$GOBIN/golines" -w --max-len={{max_line_length}} .

fmt-check:
    #!/usr/bin/env bash
    GOBIN="$({{gocmd}} env GOPATH)/bin"
    "$GOBIN/gofumpt" -l .
    "$GOBIN/golines" -l --max-len={{max_line_length}} .

# Линтинг
lint:
    #!/usr/bin/env bash
    "$({{gocmd}} env GOPATH)/bin/golangci-lint" run

lint-fix:
    #!/usr/bin/env bash
    "$({{gocmd}} env GOPATH)/bin/golangci-lint" run --fix

# Установка инструментов разработки
install-tools:
    {{gocmd}} install mvdan.cc/gofumpt@latest
    {{gocmd}} install github.com/segmentio/golines@latest
    {{gocmd}} install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Отчёт по квесту из данных игры: гейты, предметы, переходы, тупики
quest dir="extracted/ROBINSON_ISO/ROBINSON":
    {{gocmd}} run ./tools/questmap {{dir}}

# Полная проверка качества
check: fmt-check lint test

# Зависимости
deps:
    {{gocmd}} mod download
    {{gocmd}} mod tidy

clean:
    {{gocmd}} clean
    rm -rf {{binary_name}} _build dist coverage.out coverage.html

# --- self-test через headless vmhost (без окна) ------------------------------
# Снимок одного кадра игры (окно как в оригинале: 640x480)
snapshot ticks="30" out="/tmp/frame.png":
    {{gocmd}} run ./.vmdriver -pkg ./cmd -ticks {{ticks}} -out {{out}} -w 640 -h 480

# Демонстрационный прогон со вводом, пишет dbg_00..03.png
demo:
    {{gocmd}} run ./.vmdriver-game -pkg ./cmd -ticks 800 -out /tmp/dbg.png -w 640 -h 480

# Кадр мини-игры без приключения: just minigame-shot 1 -> /tmp/mg1.png
minigame-shot id ticks="30":
    ROBINSON_MINIGAME={{id}} {{gocmd}} run ./.vmdriver -pkg ./cmd/minigames \
        -ticks {{ticks}} -out /tmp/mg{{id}}.png -w 640 -h 480

# puzzle_move на карте (0) или хижине (1) без окна: just puzzle-move 1
puzzle-move id="0":
    #!/usr/bin/env bash
    set -euo pipefail
    log=$(mktemp); trap 'rm -f "$log"' EXIT
    ROBINSON_SCENE=SCENA0 ROBINSON_MINIGAME={{id}} {{gocmd}} run ./.vmdriver \
        -pkg ./tools/puzzlemove -ticks 30000 -out /tmp/move{{id}}.png \
        -w 640 -h 480 2>&1 | tee "$log"
    grep -qx "puzzlemove: ok" "$log" || { echo "puzzle-move: не дошла до конца"; exit 1; }

# Кадр конкретной сцены: just scene PALACE
scene name="SCENA0" ticks="40":
    ROBINSON_SCENE={{name}} {{gocmd}} run ./.vmdriver-game -pkg ./cmd \
        -ticks {{ticks}} -out /tmp/{{name}}.png -w 640 -h 480
