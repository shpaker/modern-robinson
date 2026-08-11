package types

// Bar is the parsed BAR.BAR layout of the inventory panel: the strip rectangle,
// the inventory slot area, and the fixed sub-boxes (portrait, text, arrows,
// tools). Coordinates are viewport pixels. LTWH fields are left,top,width,height;
// Box fields are x0,y0,x1,y1.
type Bar struct {
	Rect       [4]int // BarLTWH: x,y,w,h
	Inventory  [4]int // InventoryLTWH: x,y,w,h
	ItemW      int
	ItemH      int
	ItemsShown int
	CharBox    [4]int
	TextBox    [4]int
	LeftArrow  [4]int
	RightArrow [4]int
	ScisorsBox [4]int
	SaveBox    [4]int
	Data       string   // BarData: bar background container (bar.dat)
	Items      []string // declared item names, in order
}
