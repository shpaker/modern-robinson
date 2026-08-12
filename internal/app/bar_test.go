package app

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

// shippedBar is the panel layout as the game ships it.
func shippedBar(t *testing.T) *types.Bar {
	t.Helper()
	res := repositories.NewResources(testutil.GameRoot(t))
	c := res.SceneContainer("BAR")
	if c == nil {
		t.Fatal("no BAR container")
	}
	d, err := c.ExtractName("BAR.BAR")
	if err != nil {
		t.Fatalf("BAR.BAR: %v", err)
	}
	return repositories.SceneParser{}.ParseBar(string(d))
}

// names lists the overlays in draw order.
func names(o []barOverlay) []string {
	out := make([]string, len(o))
	for i, v := range o {
		out[i] = v.name
	}
	return out
}

// The strip background is cut out under both right-hand buttons, so they are
// drawn from BAR72/BAR74 (map) and BAR75 (disk) every frame or the panel shows
// a hole. The arrows are printed dimmed into the background instead, so only
// the lit one is overlaid, and only on a side that can still scroll.
func TestBarOverlaysPerState(t *testing.T) {
	bar := shippedBar(t)
	cases := []struct {
		name    string
		items   []string
		scroll  int
		mapOpen bool
		want    []string
	}{
		{
			name:  "fits in the window",
			items: []string{"hand", "hat"},
			want:  []string{"BAR5", "BAR74", "BAR75"},
		},
		{
			name:  "more items to the right",
			items: []string{"hand", "hat", "axe", "rope"},
			want:  []string{"BAR5", "BAR81", "BAR74", "BAR75"},
		},
		{
			name:   "scrolled to the end",
			items:  []string{"hand", "hat", "axe", "rope"},
			scroll: 1,
			want:   []string{"BAR5", "BAR78", "BAR74", "BAR75"},
		},
		{
			name:   "scrollable both ways",
			items:  []string{"hand", "hat", "axe", "rope", "net"},
			scroll: 1,
			want:   []string{"BAR5", "BAR78", "BAR81", "BAR74", "BAR75"},
		},
		{
			name:    "map found",
			items:   []string{"hand"},
			mapOpen: true,
			want:    []string{"BAR5", "BAR72", "BAR75"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gs := types.NewGameState()
			gs.Inventory = c.items
			gs.UI["map"] = c.mapOpen
			g := &Game{bar: bar, gs: gs, invScroll: c.scroll}
			got := names(g.barOverlays())
			if len(got) != len(c.want) {
				t.Fatalf("overlays = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("overlays = %v, want %v", got, c.want)
				}
			}
		})
	}
}

// Each overlay lands on the box BAR.BAR gives it: the buttons fill the
// background's cut-outs exactly, and the right arrow sits against the far edge
// of its box, where the background prints it.
func TestBarOverlayPlacement(t *testing.T) {
	bar := shippedBar(t)
	gs := types.NewGameState()
	gs.Inventory = []string{"hand", "hat", "axe", "rope", "net"}
	gs.UI["map"] = true
	g := &Game{bar: bar, gs: gs, invScroll: 1}
	at := map[string][2]int{}
	for _, o := range g.barOverlays() {
		at[o.name] = [2]int{o.x, o.y}
	}
	want := map[string][2]int{
		"BAR5":  {bar.InvMask[0], bar.InvMask[1]},
		"BAR78": {bar.LeftArrow[0], bar.LeftArrow[1]},
		"BAR81": {bar.RightArrow[2], bar.RightArrow[1]}, // no sprites loaded: width 0
		"BAR72": {bar.ScisorsBox[0], bar.ScisorsBox[1]},
		"BAR75": {bar.SaveBox[0], bar.SaveBox[1]},
	}
	for name, xy := range want {
		if at[name] != xy {
			t.Errorf("%s at %v, want %v", name, at[name], xy)
		}
	}
	if bar.InvMask != [2]int{297, 400} || bar.SaveBox[0] != 565 {
		t.Errorf("layout not the shipped one: mask=%v save=%v",
			bar.InvMask, bar.SaveBox)
	}
}

// The panel ships a normal/selected pair per declared item, so no item falls
// back to its world sprite and the engine never has to mark a selection itself.
func TestItemIndexCoversEveryPair(t *testing.T) {
	bar := shippedBar(t)
	if len(bar.Items) != 33 {
		t.Fatalf("items = %d, want 33", len(bar.Items))
	}
	g := &Game{bar: bar}
	for i, item := range bar.Items {
		if got := g.itemIndex(item); got != i {
			t.Fatalf("itemIndex(%q) = %d, want %d", item, got, i)
		}
	}
	if g.itemIndex("no-such-item") != -1 {
		t.Error("unknown item should have no pair")
	}
}
