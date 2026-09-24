// Command webicon takes the game's own icon out of the game folder and writes
// the favicon set for the browser build.
//
// START.ICO on the disc is a single 32x32 16-colour image from 1998 — the
// yellow disc with two bare footprints. It ships verbatim as favicon.ico, and
// as PNGs beside it: browsers prefer those, and iOS wants something bigger than
// 32 px for a home-screen icon.
//
//	go run ./tools/webicon -out dist/web extracted/ROBINSON_ISO/ROBINSON
//
// The icon is a game asset, so like every other asset it is never committed —
// it is extracted at build time. See docs/11-web-build.md.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// iconName is the icon the installer used for the Start Menu shortcut, and the
// only .ico on the disc.
const iconName = "START.ICO"

// touchSize is the home-screen icon. 192 is 32x6, so the upscale stays an exact
// pixel multiple — anything else would smear a 27-year-old pixel drawing.
const touchSize = 192

func main() {
	out := flag.String("out", "dist/web", "output directory")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: webicon [flags] <game-dir>\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatal(err)
	}

	src := filepath.Join(flag.Arg(0), iconName)
	raw, err := os.ReadFile(src)
	if err != nil {
		fatal(err)
	}
	img, err := decodeICO(raw)
	if err != nil {
		fatal(fmt.Errorf("%s: %w", src, err))
	}

	// The .ico goes out byte for byte: it is what the game shipped, and every
	// browser still reads it.
	if err := os.WriteFile(filepath.Join(*out, "favicon.ico"), raw, 0o644); err != nil {
		fatal(err)
	}
	if err := writePNG(filepath.Join(*out, "icon-32.png"), img, 1); err != nil {
		fatal(err)
	}
	b := img.Bounds()
	if err := writePNG(filepath.Join(*out, "icon-192.png"), img, touchSize/b.Dx()); err != nil {
		fatal(err)
	}
	fmt.Printf("icon %dx%d -> %s (favicon.ico, icon-32.png, icon-192.png)\n",
		b.Dx(), b.Dy(), *out)
}

// decodeICO reads the first image out of a Windows .ico. Only the classic
// BMP-payload icons are handled — 4 and 8 bits per pixel with a 1-bit
// transparency mask, which is all a 1998 disc can hold.
func decodeICO(d []byte) (*image.NRGBA, error) {
	if len(d) < 22 || binary.LittleEndian.Uint16(d[2:4]) != 1 {
		return nil, fmt.Errorf("not an icon file")
	}
	if binary.LittleEndian.Uint16(d[4:6]) == 0 {
		return nil, fmt.Errorf("icon holds no images")
	}
	w, h := int(d[6]), int(d[7])
	if w == 0 {
		w = 256
	}
	if h == 0 {
		h = 256
	}
	colors := int(d[8])
	size := int(binary.LittleEndian.Uint32(d[14:18]))
	off := int(binary.LittleEndian.Uint32(d[18:22]))
	if off+size > len(d) {
		return nil, fmt.Errorf("image runs past the end of the file")
	}
	img := d[off : off+size]
	if len(img) < 40 {
		return nil, fmt.Errorf("truncated bitmap header")
	}
	if img[0] == 0x89 && string(img[1:4]) == "PNG" {
		return nil, fmt.Errorf("PNG-payload icons are not handled")
	}

	headerLen := int(binary.LittleEndian.Uint32(img[0:4]))
	bpp := int(binary.LittleEndian.Uint16(img[14:16]))
	if bpp != 4 && bpp != 8 {
		return nil, fmt.Errorf("%d bits per pixel is not handled", bpp)
	}
	if colors == 0 {
		colors = 1 << bpp
	}

	palette := make([]color.NRGBA, colors)
	for i := range palette {
		p := headerLen + i*4
		if p+4 > len(img) {
			return nil, fmt.Errorf("truncated palette")
		}
		// Stored BGRA, and the fourth byte is padding rather than alpha.
		palette[i] = color.NRGBA{R: img[p+2], G: img[p+1], B: img[p], A: 255}
	}

	// Rows are padded to four bytes, stored bottom-up, and the mask follows the
	// colour bitmap — which is why the header claims twice the icon's height.
	pixStride := ((w*bpp + 31) / 32) * 4
	maskStride := ((w + 31) / 32) * 4
	pixOff := headerLen + colors*4
	maskOff := pixOff + pixStride*h
	if maskOff+maskStride*h > len(img) {
		return nil, fmt.Errorf("truncated pixel data")
	}

	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		row := img[pixOff+(h-1-y)*pixStride:]
		mask := img[maskOff+(h-1-y)*maskStride:]
		for x := range w {
			var idx int
			if bpp == 4 {
				if x%2 == 0 {
					idx = int(row[x/2] >> 4)
				} else {
					idx = int(row[x/2] & 0x0F)
				}
			} else {
				idx = int(row[x])
			}
			if idx >= len(palette) {
				return nil, fmt.Errorf("palette index %d out of range", idx)
			}
			c := palette[idx]
			// A set mask bit means "let the desktop through".
			if mask[x/8]>>(7-x%8)&1 == 1 {
				c.A = 0
			}
			dst.SetNRGBA(x, y, c)
		}
	}
	return dst, nil
}

// writePNG saves img enlarged by an integer factor, duplicating pixels. Nearest
// neighbour is the point: this is a 32-pixel drawing, and any smoothing turns
// its hard edges to mush.
func writePNG(path string, img *image.NRGBA, scale int) error {
	if scale < 1 {
		scale = 1
	}
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx()*scale, b.Dy()*scale))
	for y := range dst.Bounds().Dy() {
		for x := range dst.Bounds().Dx() {
			dst.SetNRGBA(x, y, img.NRGBAAt(x/scale, y/scale))
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, dst); err != nil {
		return err
	}
	return f.Close()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "webicon:", err)
	os.Exit(1)
}
