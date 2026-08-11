package use_cases

import "math"

var numpadAngles = [8][2]int{
	{0, 6}, {45, 9}, {90, 8}, {135, 7}, {180, 4}, {-135, 1}, {-90, 2}, {-45, 3},
}

// ScreenToNumpad maps a screen-space movement vector to a numpad direction
// (1-9, no 5), used to pick the matching Rg_XX walk movie.
func ScreenToNumpad(dx, dy float64) int {
	ang := math.Atan2(-dy, dx) * 180 / math.Pi
	best, bestd := 6, 1e9
	for _, na := range numpadAngles {
		d := math.Abs(math.Mod(ang-float64(na[0])+540, 360) - 180)
		if d < bestd {
			bestd, best = d, na[1]
		}
	}
	return best
}
