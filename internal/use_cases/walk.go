package use_cases

// Walk cycles follow the characters' DO.LST inventory: a cycle is named by two
// numpad digits, "<step><next>", where the first digit is the direction this
// cycle walks and the second is where the walk goes next; 5 means standing. So a
// route is played as 5d1 (accelerate, no cell change), then d(i)d(i+1) for every
// step, and finally d(n)5 (brake). Roby has all 80 pairs (NumPadGoing, eight
// directions); Friday only has the four arrows (ArrowGoing: 2, 4, 6, 8).

// arrowDirs are the directions Friday can walk.
var arrowDirs = map[int]bool{2: true, 4: true, 6: true, 8: true}

// StepDir returns the numpad direction of one grid step, or 5 when it does not
// move. Grid y grows towards the viewer, so dy > 0 walks south.
func StepDir(from, to [2]int) int {
	dx, dy := to[0]-from[0], to[1]-from[1]
	sx, sy := sign(dx), sign(dy)
	switch [2]int{sx, sy} {
	case [2]int{0, 1}:
		return 2
	case [2]int{-1, 0}:
		return 4
	case [2]int{1, 0}:
		return 6
	case [2]int{0, -1}:
		return 8
	case [2]int{-1, 1}:
		return 1
	case [2]int{1, 1}:
		return 3
	case [2]int{-1, -1}:
		return 7
	case [2]int{1, -1}:
		return 9
	}
	return 5
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// WalkCycles turns a route (the cells to step onto, excluding the current one)
// into the cycle suffixes to play. arrowsOnly restricts the result to the four
// arrow directions (Friday), collapsing diagonals into their vertical part.
// The first cycle only accelerates; every later one advances exactly one cell,
// so a route of n cells yields n+1 cycles.
func WalkCycles(start [2]int, route [][2]int, arrowsOnly bool) []string {
	dirs := make([]int, 0, len(route))
	cur := start
	for _, c := range route {
		d := StepDir(cur, c)
		if d == 5 {
			continue // duplicate cell
		}
		if arrowsOnly && !arrowDirs[d] {
			d = arrowFallback(d)
		}
		dirs = append(dirs, d)
		cur = c
	}
	if len(dirs) == 0 {
		return nil
	}
	out := make([]string, 0, len(dirs)+1)
	out = append(out, cycleName(5, dirs[0]))
	for i := 0; i < len(dirs)-1; i++ {
		out = append(out, cycleName(dirs[i], dirs[i+1]))
	}
	out = append(out, cycleName(dirs[len(dirs)-1], 5))
	return out
}

// arrowFallback maps a diagonal onto the nearest arrow direction.
func arrowFallback(d int) int {
	switch d {
	case 1, 3:
		return 2 // south-ish
	case 7, 9:
		return 8 // north-ish
	}
	return d
}

func cycleName(a, b int) string {
	return string(rune('0'+a)) + string(rune('0'+b))
}
