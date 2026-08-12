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

# Собрать архивы для раздачи: бинарник + README + образец настроек
release version="dev":
    #!/usr/bin/env bash
    set -euo pipefail
    out=_build/release; rm -rf "$out"; mkdir -p "$out"
    for target in macos:darwin:arm64:1:robinson_darwin_arm64 \
                  linux:linux:amd64:0:robinson_linux_amd64 \
                  windows:windows:amd64:0:robinson_windows_amd64.exe; do
        IFS=: read -r name goos goarch cgo binary <<<"$target"
        stage="$out/$name"; mkdir -p "$stage"
        GOOS="$goos" GOARCH="$goarch" CGO_ENABLED="$cgo" {{gocmd}} build -trimpath \
            -ldflags "-s -w -X {{module}}/internal/app.Version={{version}}" \
            -o "$stage/$binary" ./cmd
        cp README.md "$stage/"
        cp packaging/config.yml "$stage/"
        ( cd "$out" && zip -qr "modern-robinson_{{version}}_$name.zip" "$name" )
        rm -rf "$stage"
    done
    ls -la "$out"
    echo "The archives go next to the game's DATA/ folder; see README.md."


# Запустить (нужна папка игры рядом или путём аргументом)
run: build
    ./{{binary_name}}

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

# Полная проверка качества
check: fmt-check lint test

# Зависимости
deps:
    {{gocmd}} mod download
    {{gocmd}} mod tidy

clean:
    {{gocmd}} clean
    rm -rf {{binary_name}} _build coverage.out coverage.html

# --- self-test через headless vmhost (без окна) ------------------------------
# Снимок одного кадра игры (окно как в оригинале: 640x480)
snapshot ticks="30" out="/tmp/frame.png":
    {{gocmd}} run ./.vmdriver -pkg ./cmd -ticks {{ticks}} -out {{out}} -w 640 -h 480

# Демонстрационный прогон со вводом, пишет dbg_00..03.png
demo:
    {{gocmd}} run ./.vmdriver-game -pkg ./cmd -ticks 800 -out /tmp/dbg.png -w 640 -h 480

# Кадр конкретной сцены: just scene PALACE
scene name="SCENA0" ticks="40":
    ROBINSON_SCENE={{name}} {{gocmd}} run ./.vmdriver-game -pkg ./cmd \
        -ticks {{ticks}} -out /tmp/{{name}}.png -w 640 -h 480
