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

build-wasm:
    #!/usr/bin/env bash
    set -euo pipefail
    VERSION="dev-$(date -u +%Y-%m-%dT%H:%M)"; out=dist/web; mkdir -p "$out"
    GOOS=js GOARCH=wasm {{gocmd}} build -trimpath \
        -ldflags "-s -w -X {{module}}/internal/app.Version=${VERSION}" \
        -o "$out/{{binary_name}}.wasm" ./cmd
    cp "$({{gocmd}} env GOROOT)/lib/wasm/wasm_exec.js" "$out/"
    echo "WASM build -> $out"

build-all: build-macos build-linux build-windows

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
    rm -rf {{binary_name}} _build dist coverage.out coverage.html

# --- self-test через headless vmhost (без окна) ------------------------------
# Снимок одного кадра игры
snapshot ticks="30" out="/tmp/frame.png":
    {{gocmd}} run ./.vmdriver -pkg ./cmd -ticks {{ticks}} -out {{out}} -w 1024 -h 400

# Демонстрационный прогон: F1-debug + ходьба + переход, пишет dbg_00..03.png
demo:
    {{gocmd}} run ./.vmdriver-game -pkg ./cmd -ticks 140 -out /tmp/dbg.png -w 1024 -h 400
