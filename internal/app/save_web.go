//go:build js && wasm

package app

import (
	"encoding/base64"
	"fmt"
	"syscall/js"
)

// localSaves keeps the slots in localStorage. Twelve slots are a few kilobytes
// of JSON plus a 129x98 PNG each — about 330 KB all told against a 5 MB quota,
// so there is no reason to reach for IndexedDB and its async API.
type localSaves struct{ prefix string }

func (l localSaves) key(name string) string { return l.prefix + name }

func (l localSaves) store() js.Value {
	return js.Global().Get("localStorage")
}

func (l localSaves) Read(name string) ([]byte, error) {
	st := l.store()
	if !st.Truthy() {
		return nil, fmt.Errorf("localStorage unavailable")
	}
	v := st.Call("getItem", l.key(name))
	if !v.Truthy() {
		return nil, fmt.Errorf("slot %q empty", name)
	}
	return base64.StdEncoding.DecodeString(v.String())
}

func (l localSaves) Write(name string, data []byte) error {
	st := l.store()
	if !st.Truthy() {
		return fmt.Errorf("localStorage unavailable")
	}
	// A full quota throws on the JS side; turn that into a plain error so a
	// failed save just reports itself instead of taking the frame down.
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("localStorage write %q: %v", name, r)
			}
		}()
		st.Call("setItem", l.key(name), base64.StdEncoding.EncodeToString(data))
	}()
	return err
}

// defaultSaveStore is the platform's place for saves: localStorage in the
// browser, where there is no game folder to write into.
func defaultSaveStore(string) saveStore {
	return localSaves{prefix: "robinson/"}
}
