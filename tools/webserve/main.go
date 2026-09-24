// Command webserve serves the browser build locally the way the data host does,
// so that what is tested is what ships.
//
// Like the data host, it serves the shell (dist/web) and the resource set
// (dist/webdata, here under /data/ so the service worker leaves it uncached)
// from one origin, prefers a .gz sidecar when the client accepts gzip, and
// answers with permissive CORS headers.
//
//	go run ./tools/webserve
//	open 'http://localhost:8080/?data=http://localhost:8080/data/v1/'
package main

import (
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	shell := flag.String(
		"shell",
		"dist/web",
		"directory with index.html and robinson.wasm",
	)
	data := flag.String(
		"data",
		"dist/webdata",
		"directory with the packed resource set",
	)
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/data/", http.StripPrefix("/data/", &files{root: *data}))
	mux.Handle("/", &files{root: *shell})

	log.Printf("shell   %s", *shell)
	log.Printf("data    %s", *data)
	log.Printf(
		"open    http://localhost%s/?data=http://localhost%s/data/v1/",
		*addr,
		*addr,
	)
	log.Fatal(
		http.ListenAndServe(*addr, mux),
	) //nolint:gosec // a local dev server
}

// files is a static file handler with the two behaviours the data host has
// and http.FileServer does not: precompressed sidecars and CORS.
type files struct{ root string }

func (f *files) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" || strings.HasSuffix(name, "/") {
		name += "index.html"
	}
	// Reject anything that tries to climb out of the served directory.
	clean := filepath.Clean(filepath.FromSlash(name))
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	path := filepath.Join(f.root, clean)

	if ct := mime.TypeByExtension(filepath.Ext(clean)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	if acceptsGzip(r) {
		if st, err := os.Stat(path + ".gz"); err == nil {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Length", fmt.Sprint(st.Size()))
			http.ServeFile(w, r, path+".gz")
			return
		}
	}
	http.ServeFile(w, r, path)
}

func acceptsGzip(r *http.Request) bool {
	for _, enc := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(strings.SplitN(enc, ";", 2)[0]) == "gzip" {
			return true
		}
	}
	return false
}
