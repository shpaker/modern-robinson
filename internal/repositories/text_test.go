package repositories

import (
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/testutil"
)

func TestDecodeCP1251(t *testing.T) {
	// "Пальма" in CP1251
	got := decodeCP1251([]byte{0xCF, 0xE0, 0xEB, 0xFC, 0xEC, 0xE0})
	if got != "Пальма" {
		t.Fatalf("got %q", got)
	}
	if s := decodeCP1251([]byte{0xA8, 0xB8}); s != "Ёё" {
		t.Fatalf("yo: %q", s)
	}
}

func TestTexts(t *testing.T) {
	root := testutil.GameRoot(t)
	r := NewResources(root)
	lines := r.Texts()
	if len(lines) < 1000 {
		t.Fatalf("texts: %d lines", len(lines))
	}
	if !strings.Contains(lines[2], "Пальма") {
		t.Fatalf("line 2 = %q, want Пальма", lines[2])
	}
	if !strings.Contains(lines[5], "Краб") {
		t.Fatalf("line 5 = %q, want Краб", lines[5])
	}
}
