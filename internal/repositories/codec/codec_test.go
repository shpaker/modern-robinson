package codec

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

func idxSum(n *types.NGB) (uint64, int) {
	var s uint64
	op := 0
	for i, v := range n.Indices {
		s += uint64(v) * uint64(i%251+1)
		if n.Mask[i] {
			op++
		}
	}
	return s, op
}

// TestContainerLZHUFRawNGB covers directory decryption, LZHUF, the BGR palette
// and subtype-A NGB via the OPTIONS screen (golden values from the Python ref).
func TestContainerLZHUFRawNGB(t *testing.T) {
	root := testutil.GameRoot(t)
	c, err := Open(filepath.Join(root, "DATA", "OPTIONS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	col, err := c.ExtractName("OPTIONS.COL")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := LoadPalette(col)[27], [4]byte{198, 156, 74, 255}; got != want {
		t.Errorf("OPTIONS.COL[27] = %v, want %v (BGR->RGB)", got, want)
	}
	nd, err := c.ExtractName("OPTIONS.NGB")
	if err != nil {
		t.Fatal(err)
	}
	n := DecodeNGB(nd)
	if n.Width != 640 || n.Height != 480 || n.Sig != sigRaw {
		t.Fatalf("OPTIONS.NGB = %dx%d sig=%#x", n.Width, n.Height, n.Sig)
	}
	sum, op := idxSum(n)
	if sum != 2784321292 {
		t.Errorf("idxsum = %d, want 2784321292", sum)
	}
	if op != 307200 {
		t.Errorf("opaque = %d, want 307200", op)
	}
}

// TestLZSSText covers method 0x40 (LZSS) via a character frame script.
func TestLZSSText(t *testing.T) {
	root := testutil.GameRoot(t)
	c, err := Open(filepath.Join(root, "DATA", "CHAR", "ROBY.DAN"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := c.ExtractName("DROVA.FS")
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != 519 {
		t.Errorf("DROVA.FS size = %d, want 519", len(d))
	}
	if !strings.HasPrefix(string(d), "ScriptName") {
		t.Errorf("DROVA.FS start = %q", string(d[:20]))
	}
}

// TestNGBRLESprite covers subtype-B (RLE) NGB decoding.
func TestNGBRLESprite(t *testing.T) {
	root := testutil.GameRoot(t)
	mv, err := Open(filepath.Join(root, "DATA", "MOVIE", "ROBY1.MV"))
	if err != nil {
		t.Fatal(err)
	}
	var frames [][]byte
	for _, e := range mv.Entries() {
		if strings.HasSuffix(strings.ToUpper(e.Name), ".NGB") {
			d, _ := mv.Extract(e)
			frames = append(frames, d)
		}
	}
	if len(frames) < 6 {
		t.Fatalf("frames = %d, want >= 6", len(frames))
	}
	n := DecodeNGB(frames[5])
	if n.Sig != sigRLE {
		t.Errorf("sig = %#x, want subtype B %#x", n.Sig, sigRLE)
	}
	if _, op := idxSum(n); op != 5244 {
		t.Errorf("ROBY1 frame5 opaque = %d, want 5244", op)
	}
}

// TestDeflate0x100 covers method 0x100 (raw DEFLATE) via a HOUSE.DAT sprite.
func TestDeflate0x100(t *testing.T) {
	root := testutil.GameRoot(t)
	c, err := Open(filepath.Join(root, "HOUSE.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := c.ExtractName("H11.NGB")
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != 2252 {
		t.Errorf("H11.NGB size = %d, want 2252", len(d))
	}
	n := DecodeNGB(d)
	if n.Width != 70 || n.Height != 45 {
		t.Errorf("H11.NGB dims = %dx%d, want 70x45", n.Width, n.Height)
	}
}

// TestPositionTables checks the non-canonical LZHUF position tables.
func TestPositionTables(t *testing.T) {
	if dLen[0] != 1 || dLen[31] != 1 || dLen[32] != 2 {
		t.Errorf("d_len prefix = %d,%d,%d, want 1,1,2", dLen[0], dLen[31], dLen[32])
	}
	if dCode[255] != 63 {
		t.Errorf("d_code[255] = %d, want 63", dCode[255])
	}
}
