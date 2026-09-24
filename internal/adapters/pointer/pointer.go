// Package pointer merges the mouse and a finger into the one pointer the game
// reads. The original knows only the mouse; a finger is mapped onto it:
//
//   - a tap is a click, taken on release, so a finger can also drag;
//   - a finger held down is a cursor hovering there (the caption shows);
//   - a finger held still for half a second is a right click;
//   - a finger moving past a few pixels drags, and a caller that uses the
//     drag (the scene scrolling) claims it, so its release is no click.
//
// Update samples the devices once per tick; everything else reads that sample.
package pointer

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	// DragSlop is how far a finger may wander, in screen pixels, and still tap.
	DragSlop = 10
	// LongPress is how long a finger held still takes to right-click, seconds.
	LongPress = 0.5
)

// Sample is one tick of raw device state.
type Sample struct {
	MouseX, MouseY int
	Left, Right    bool // mouse buttons held
	Finger         bool // the followed finger is down
	FingerX        int
	FingerY        int
}

// Tracker turns raw samples into the pointer's per-tick events.
type Tracker struct {
	x, y      int
	touch     bool // the last input was a finger
	mouseSeen bool // the mouse has moved or pressed since start
	started   bool
	prev      Sample

	down, up, pressed, clicked, right bool
	dragX, dragY                      int

	finger   bool // a finger gesture is under way
	startX   int
	startY   int
	held     float64
	dragging bool
	longDone bool
	taken    bool // the gesture was claimed: its release is no click
}

// Step advances the tracker by one tick of dt seconds.
func (t *Tracker) Step(s Sample, dt float64) {
	t.down, t.up, t.clicked, t.right = false, false, false, false
	t.dragX, t.dragY = 0, 0
	switch {
	case s.Finger || t.finger:
		t.stepFinger(s, dt)
	case t.mouseActive(s):
		t.stepMouse(s)
	}
	t.prev, t.started = s, true
}

// mouseActive reports whether the mouse is the pointer this tick: it has been
// used since the last finger, or it is being used now.
func (t *Tracker) mouseActive(s Sample) bool {
	moved := t.started &&
		(s.MouseX != t.prev.MouseX || s.MouseY != t.prev.MouseY)
	pressed := s.Left && !t.prev.Left || s.Right && !t.prev.Right
	if moved || pressed {
		t.touch, t.mouseSeen = false, true
	}
	return !t.touch
}

func (t *Tracker) stepMouse(s Sample) {
	t.x, t.y = s.MouseX, s.MouseY
	t.pressed = s.Left
	t.down = s.Left && !t.prev.Left
	t.up = !s.Left && t.prev.Left
	t.clicked = t.down
	t.right = s.Right && !t.prev.Right
}

func (t *Tracker) stepFinger(s Sample, dt float64) {
	switch {
	case s.Finger && !t.finger: // touch down
		t.touch, t.finger = true, true
		t.startX, t.startY = s.FingerX, s.FingerY
		t.held, t.dragging, t.longDone, t.taken = 0, false, false, false
		t.x, t.y = s.FingerX, s.FingerY
		t.down, t.pressed = true, true
	case s.Finger: // held
		dx, dy := s.FingerX-t.x, s.FingerY-t.y
		t.x, t.y = s.FingerX, s.FingerY
		t.held += dt
		if !t.dragging && far(t.x-t.startX, t.y-t.startY) {
			// The first drag tick carries the whole way from the start.
			t.dragging = true
			dx, dy = t.x-t.startX, t.y-t.startY
		}
		if t.dragging {
			t.dragX, t.dragY = dx, dy
		} else if !t.longDone && t.held >= LongPress {
			t.longDone, t.right = true, true
		}
	default: // lifted
		t.finger, t.pressed = false, false
		t.up = true
		t.clicked = !t.taken
	}
}

func far(dx, dy int) bool { return dx*dx+dy*dy > DragSlop*DragSlop }

// Pos is where the pointer is: the cursor, or the finger (the last one seen).
func (t *Tracker) Pos() (int, int) { return t.x, t.y }

// Touch reports whether the last input came from a finger.
func (t *Tracker) Touch() bool { return t.touch }

// Mouse reports whether the pointer is a mouse that has actually been used, so
// a cursor still parked where the window opened is not taken for a real one.
func (t *Tracker) Mouse() bool { return !t.touch && t.mouseSeen }

// Hover reports whether the pointer rests on the screen: the mouse always, a
// finger only while it is down.
func (t *Tracker) Hover() bool { return !t.touch || t.finger }

// Down reports a press that began this tick, finger or button.
func (t *Tracker) Down() bool { return t.down }

// Pressed reports the button or finger held down.
func (t *Tracker) Pressed() bool { return t.pressed }

// Up reports a release this tick.
func (t *Tracker) Up() bool { return t.up }

// Clicked reports a click: the left button going down, or a finger lifted
// from a gesture nobody claimed.
func (t *Tracker) Clicked() bool { return t.clicked }

// TakeRightClick reports a right click — the right button, or a finger held
// still long enough — and claims the finger's gesture, so lifting it after
// the turn does not click as well.
func (t *Tracker) TakeRightClick() bool {
	if t.right && t.finger {
		t.taken = true
	}
	return t.right
}

// Dragging reports a finger drag under way and where the gesture began.
func (t *Tracker) Dragging() (startX, startY int, ok bool) {
	return t.startX, t.startY, t.finger && t.dragging
}

// TakeDrag returns this tick's drag step and claims the gesture: the finger
// is scrolling, so lifting it is no tap.
func (t *Tracker) TakeDrag() (dx, dy int) {
	if t.finger && t.dragging {
		t.taken = true
	}
	return t.dragX, t.dragY
}

// The game's one pointer.
var (
	cur       Tracker
	followed  ebiten.TouchID
	following bool
	ids       []ebiten.TouchID
)

// Update samples the mouse and the touch screen; call it once per tick before
// anything reads the pointer. Only one finger is followed: the first to come
// down, until it lifts; fingers already down then are ignored.
func Update() {
	s := Sample{}
	s.MouseX, s.MouseY = ebiten.CursorPosition()
	s.Left = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	s.Right = ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	ids = ebiten.AppendTouchIDs(ids[:0])
	if following && !has(ids, followed) {
		following = false
	}
	if !following {
		if fresh := inpututil.AppendJustPressedTouchIDs(nil); len(fresh) > 0 {
			followed, following = fresh[0], true
		}
	}
	if following {
		s.Finger = true
		s.FingerX, s.FingerY = ebiten.TouchPosition(followed)
	}
	cur.Step(s, 1/float64(ebiten.TPS()))
}

func has(ids []ebiten.TouchID, id ebiten.TouchID) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// Pos is where the pointer is.
func Pos() (int, int) { return cur.Pos() }

// Touch reports whether the last input came from a finger.
func Touch() bool { return cur.Touch() }

// Mouse reports whether the pointer is a mouse that has been used.
func Mouse() bool { return cur.Mouse() }

// Hover reports whether the pointer rests on the screen.
func Hover() bool { return cur.Hover() }

// Down reports a press that began this tick.
func Down() bool { return cur.Down() }

// Pressed reports the button or finger held down.
func Pressed() bool { return cur.Pressed() }

// Up reports a release this tick.
func Up() bool { return cur.Up() }

// Clicked reports a click this tick.
func Clicked() bool { return cur.Clicked() }

// TakeRightClick reports a right click and claims a finger's gesture.
func TakeRightClick() bool { return cur.TakeRightClick() }

// Dragging reports a finger drag under way and where it began.
func Dragging() (startX, startY int, ok bool) { return cur.Dragging() }

// TakeDrag returns this tick's drag step and claims the gesture.
func TakeDrag() (dx, dy int) { return cur.TakeDrag() }
