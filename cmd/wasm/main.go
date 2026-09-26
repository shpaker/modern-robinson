//go:build js && wasm

// Command wasm is the browser build of the remake. It downloads the published
// resource set, builds the same Game the desktop binary runs, and hands it to
// Ebitengine's WebGL/WebAudio backend.
//
// The whole resource set is fetched before the game starts. IResources is
// synchronous and the sound bank is read entry by entry from inside
// Game.Update, and a browser cannot block on a fetch inside a frame callback —
// so there is nothing to be lazy with. See package webdata.
package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"syscall/js"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/app"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/repositories/webdata"
	"github.com/shpaker/modern-robinson/internal/repositories/webfs"
)

// defaultDataBase is where the published resource set lives: next to the page,
// on the same host (see webdata.ResolveBase). ?data= points the build at
// another copy, which is how it is run against a local packer output.
const defaultDataBase = "v1/"

// envFromQuery maps query parameters onto the ROBINSON_* variables the game
// already reads for scene entry and diagnostics. os.Setenv works on js/wasm
// (the runtime keeps its own environment), so internal/app needs no web-only
// branch of its own.
var envFromQuery = map[string]string{
	"scene":    "ROBINSON_SCENE",
	"minigame": "ROBINSON_MINIGAME",
	"vars":     "ROBINSON_VARS",
	"ui":       "ROBINSON_UI",
	"items":    "ROBINSON_ITEMS",
	"trace":    "ROBINSON_TRACE",
	"frid":     "ROBINSON_FRID",
}

// configFromQuery are the config.yml keys that make sense in a browser. scale
// is not among them: Ebitengine sizes the canvas from the page on js, so the
// window magnification is the page's business, not the game's.
var configFromQuery = append(
	[]string{"sound", "music", "speed", "debug", "crt"}, app.TubeKeys...,
)

func main() {
	// The resource set is ~279 MB live in a 32-bit heap, so the peak matters
	// more than collector throughput. Most of that is pointer-free blobs, which
	// are cheap to mark, and the game allocates little per frame — a tight
	// target costs almost nothing here and keeps the tab well clear of trouble.
	debug.SetGCPercent(20)

	q := query()
	base := q("data")
	if base == "" {
		base = defaultDataBase
	}

	ui := newBootUI()
	base, err := webdata.ResolveBase(
		js.Global().Get("location").Get("href").String(), base,
	)
	if err != nil {
		ui.fail(err)
		return
	}

	ui.status("Читаю список ресурсов…")
	m, err := webdata.LoadManifest(base, app.Version)
	if err != nil {
		ui.fail(err)
		return
	}

	ui.status(fmt.Sprintf("Загружаю ресурсы игры (%d файлов)", len(m.Files)))
	files, err := webdata.Fetch(base, m, ui.progress)
	if err != nil {
		ui.fail(err)
		return
	}

	for key, env := range envFromQuery {
		if v := q(key); v != "" {
			_ = os.Setenv(env, v)
		}
	}

	ui.status("Готовлю игру…")
	res := repositories.NewResourcesFS(webfs.New(files), m.Paths())
	g := app.NewGameWith(
		res,
		app.LoadConfigFrom(strings.NewReader(configLines(q))),
	)

	// Waiting for a click before the first frame is what lets the sound work:
	// browsers keep an AudioContext suspended until the user interacts, and
	// oto's js driver resumes on the first mouseup on the document.
	ui.waitForPlay()
	ui.hide()

	ebiten.SetWindowTitle("Новый Робинзон — " + app.Version)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

// configLines turns the query string into the "key: value" lines LoadConfigFrom
// parses, so the browser and config.yml agree on names, ranges and defaults.
func configLines(q func(string) string) string {
	var b strings.Builder
	for _, k := range configFromQuery {
		if v := q(k); v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	return b.String()
}

// query reads the page's query string.
func query() func(string) string {
	params := js.Global().Get("URLSearchParams").New(
		js.Global().Get("location").Get("search"),
	)
	return func(k string) string {
		v := params.Call("get", k)
		if !v.Truthy() {
			return ""
		}
		return strings.TrimSpace(v.String())
	}
}

// bootUI drives the loading overlay defined in web/index.html.
type bootUI struct {
	root, status_, bar, pct, play, errBox js.Value
}

func newBootUI() *bootUI {
	doc := js.Global().Get("document")
	el := func(id string) js.Value { return doc.Call("getElementById", id) }
	return &bootUI{
		root:    el("boot"),
		status_: el("boot-status"),
		bar:     el("boot-bar"),
		pct:     el("boot-pct"),
		play:    el("boot-play"),
		errBox:  el("boot-error"),
	}
}

func (u *bootUI) status(s string) { u.status_.Set("textContent", s) }

func (u *bootUI) progress(done, total int64) {
	if total <= 0 {
		return
	}
	frac := float64(done) / float64(total)
	u.bar.Get("style").Set("width", fmt.Sprintf("%.2f%%", frac*100))
	u.pct.Set("textContent", fmt.Sprintf(
		"%.0f%%  ·  %.0f / %.0f МБ", frac*100, mb(done), mb(total),
	))
}

func mb(n int64) float64 { return float64(n) / (1 << 20) }

// waitForPlay shows the start button and blocks until it is clicked. Blocking
// here is safe and is the whole point: main is not a JS callback, so the Go
// runtime hands control back to the event loop while it waits.
func (u *bootUI) waitForPlay() {
	u.status("Готово")
	u.pct.Set("textContent", "")
	u.play.Get("style").Set("display", "inline-block")

	clicked := make(chan struct{}, 1)
	cb := js.FuncOf(func(js.Value, []js.Value) any {
		select {
		case clicked <- struct{}{}:
		default:
		}
		return nil
	})
	u.play.Call("addEventListener", "click", cb)
	<-clicked
	u.play.Call("removeEventListener", "click", cb)
	cb.Release()
}

func (u *bootUI) hide() { u.root.Get("style").Set("display", "none") }

func (u *bootUI) fail(err error) {
	log.Println("boot:", err)
	u.status("Не удалось загрузить игру")
	u.pct.Set("textContent", "")
	u.bar.Get("style").Set("width", "0%")
	u.errBox.Set("textContent", err.Error())
	u.errBox.Get("style").Set("display", "block")
}
