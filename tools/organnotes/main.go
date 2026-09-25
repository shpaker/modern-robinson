// Command organnotes listens to the bamboo organ once, so that the game never
// has to: it finds the pitch of every note the organ sounds (PIPE00..PIPE08.WAV
// of MINIGAME.WDT) and transcribes Friday's aria (MELODY.WAV) into notes, and
// prints them by name. The names are all the remake keeps (puzzleSounds in
// internal/app): a driver hears the organ as notes, never as files, and no
// sound of the game goes into the repository.
//
// Pitch is found with YIN (de Cheveigné & Kawahara, 2002) on 46 ms frames.
// The aria is whistled, so it is cut into notes where the whistle stops or
// dips between two swells; a note is named by the median pitch of its frames.
//
// It then checks itself: the phrase the drummer plays with the tubes as the
// engine wants them (docs/10-minigames.md) must be the aria, transposed. What
// does not match is printed, with how far off the whistle was, and so is
// every other way to seat the tubes that the names match as well.
//
//	go run ./tools/organnotes [game-dir]
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/shpaker/modern-robinson/internal/repositories"
)

// The organ as MINIGAME.DLL plays it (docs/10-minigames.md): the drummer's
// phrase over the mouths, the tubes the engine accepts in them, and the sound
// of each tube. The DLL loads its notes by index from MINIGAME.WDT, where
// they lie in tube order, the dud first: PIPE00, PIPE02, PIPE04, PIPE01, ...
var (
	phrase = []int{0, 1, 0, 1, 3, 0, 2, 4, 4, 4, 5, 6, 1, 1, 7}
	answer = [8]int{2, 0, 1, 3, 4, 5, 6, 7} // mouth -> tube
	tubes  = [8]string{
		"PIPE02.WAV", "PIPE04.WAV", "PIPE01.WAV", "PIPE03.WAV",
		"PIPE05.WAV", "PIPE06.WAV", "PIPE07.WAV", "PIPE08.WAV",
	}
)

// aria is Friday's aria, as the organ game plays it.
const aria = "MELODY.WAV"

// Analysis: a 1024-sample window every 256 samples, pitches from 60 Hz (below
// the lowest tube) to 4 kHz (above the whistle).
const (
	window = 1024
	hop    = 256
	minHz  = 60
	maxHz  = 4000
	// A frame is pitched when its YIN aperiodicity is under this, and loud
	// enough against the loudest frame of the sound.
	aperiodic = 0.2
	quiet     = 0.05
	// Among the dips of the difference function, the shortest period whose
	// dip is this close to the deepest one is the pitch: a period twice as
	// long fits a periodic sound as well, and would read an octave low.
	near = 0.05
	// A whistle's swell that sinks under this share of both its neighbours'
	// peaks ends one note and starts the next.
	dip = 0.25
	// Pitched runs shorter than this are the tail of the note before.
	minNote = 0.05
)

// frame is one analysis window.
type frame struct {
	rms float64
	hz  float64 // 0 when unpitched
}

// note is a note heard: when, how long, how high.
type note struct {
	at, dur float64
	midi    float64 // fractional MIDI number, 69 = A4 = 440 Hz
}

func main() {
	root := "extracted/ROBINSON_ISO/ROBINSON"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	res := repositories.NewResources(root)
	if res.Sound(aria) == nil {
		fmt.Fprintf(os.Stderr, "no MINIGAME.WDT under %s\n", root)
		os.Exit(1)
	}
	load := func(name string) ([]float64, int) {
		pcm, rate, err := decodeWAV(res.Sound(name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			os.Exit(1)
		}
		return pcm, rate
	}

	fmt.Println("organ notes:")
	pitch := map[string]float64{} // file -> MIDI, absent when toneless
	for i := range 9 {
		file := fmt.Sprintf("PIPE%02d.WAV", i)
		pcm, rate := load(file)
		m, share := steady(frames(pcm, rate))
		tube := "dud"
		if t := slices.Index(tubes[:], file); t >= 0 {
			tube = fmt.Sprintf("tube %d", t)
		}
		if math.IsNaN(m) {
			fmt.Printf("  %s  %-7s  no tone (%2.0f%% pitched)  %s\n",
				file, tube, share*100, toneless)
			continue
		}
		pitch[file] = m
		fmt.Printf("  %s  %-7s  %6.1f Hz  %-12s %+3.0f cents  (%2.0f%%)\n",
			file, tube, hz(m), name(m), cents(m), share*100)
	}

	pcm, rate := load(aria)
	sung := split(frames(pcm, rate))
	fmt.Printf("\naria: %d notes in %.2f s\n", len(sung),
		float64(len(pcm))/float64(rate))
	for i, n := range sung {
		fmt.Printf("  %2d  %5.2f s  +%.2f s  %6.1f Hz  %-12s %+3.0f cents\n",
			i+1, n.at, n.dur, hz(n.midi), name(n.midi), cents(n.midi))
	}

	check(pitch, sung)

	fmt.Println("\nfor internal/app/minigame.go:")
	for i := range 9 {
		file := fmt.Sprintf("PIPE%02d.WAV", i)
		word := toneless
		if m, ok := pitch[file]; ok {
			word = name(m)
		}
		fmt.Printf("\t%q: %q,\n", strings.ToLower(file), word)
	}
	names := make([]string, len(sung))
	for i, n := range sung {
		names[i] = fmt.Sprintf("%q", name(n.midi))
	}
	fmt.Printf("\tariaNotes = []string{%s}\n", strings.Join(names, ", "))
}

// toneless is what the dud is heard as: a puff with no note in it.
const toneless = "глухо"

// check lays the drummer's phrase, the tubes standing as the engine wants
// them, against the aria: shifted by the interval most of their notes are
// apart, they must be one tune. Notes are paired by an alignment that lets a
// note go missing on either side.
func check(pitch map[string]float64, sung []note) {
	want := make([]int, len(phrase))
	for i, m := range phrase {
		v, ok := pitch[tubes[answer[m]]]
		if !ok {
			fmt.Printf("\ncheck: tube %d has no tone\n", answer[m])
			return
		}
		want[i] = int(math.Round(v))
	}
	got := make([]int, len(sung))
	for i, n := range sung {
		got[i] = int(math.Round(n.midi))
	}
	words := func(ms []int) string {
		s := make([]string, len(ms))
		for i, m := range ms {
			s[i] = name(float64(m))
		}
		return strings.Join(s, " ")
	}
	fmt.Println("\ncheck: the phrase with the tubes in place against the aria")
	fmt.Println("  phrase: " + words(want))
	fmt.Println("  aria:   " + words(got))

	// The shift is the commonest difference of notes in the same place.
	count := map[int]int{}
	for i := range min(len(want), len(got)) {
		count[got[i]-want[i]]++
	}
	shift, best := 0, -1
	for d, c := range count {
		if c > best || c == best && d < shift {
			shift, best = d, c
		}
	}
	fmt.Printf("  shift:  %+d semitones (%d octaves %+d)\n", shift,
		shift/12, shift%12)

	match := 0
	for _, p := range align(want, got, shift) {
		switch {
		case p[0] < 0:
			n := sung[p[1]]
			fmt.Printf("  extra in the aria: %s at %.2f s\n",
				name(n.midi), n.at)
		case p[1] < 0:
			fmt.Printf("  phrase note %d (%s) not in the aria\n", p[0]+1,
				name(float64(want[p[0]])))
		case got[p[1]]-want[p[0]] == shift:
			match++
		default:
			n := sung[p[1]]
			fmt.Printf("  phrase note %d: want %s, whistled %s "+
				"(%.0f cents off the note wanted)\n",
				p[0]+1, name(float64(want[p[0]]+shift)), name(n.midi),
				(n.midi-float64(want[p[0]]+shift))*100)
		}
	}
	fmt.Printf("  %d of %d phrase notes are in the aria as they are\n",
		match, len(want))
	if len(sung) == len(phrase) {
		rivals(pitch, sung, shift)
	}
}

// rivals seats the tubes every way there is and lists, by the notes of the
// mouths, those whose phrase matches the aria note for note in as many notes
// as the answer's does, or more: going by the names of the notes, an ear
// cannot tell them from the answer. How far off the whistle is from each, in
// all, is what could.
func rivals(pitch map[string]float64, sung []note, shift int) {
	var of [8]int // tube -> its note
	for t, f := range tubes {
		of[t] = int(math.Round(pitch[f]))
	}
	score := func(mouths [8]int) (int, float64) {
		n, off := 0, 0.0
		for i, m := range phrase {
			want := mouths[m] + shift
			if int(math.Round(sung[i].midi)) == want {
				n++
			}
			off += math.Abs(sung[i].midi - float64(want))
		}
		return n, off * 100
	}
	var answered [8]int
	for m, t := range answer {
		answered[m] = of[t]
	}
	best, _ := score(answered)
	seen := map[[8]int]bool{}
	var alike [][8]int
	var seat func(mouths [8]int, m, used int)
	seat = func(mouths [8]int, m, used int) {
		if m == len(mouths) {
			if n, _ := score(mouths); !seen[mouths] && n >= best {
				alike = append(alike, mouths)
			}
			seen[mouths] = true
			return
		}
		for t := range tubes {
			if used&(1<<t) == 0 {
				mouths[m] = of[t]
				seat(mouths, m+1, used|1<<t)
			}
		}
	}
	seat([8]int{}, 0, 0)
	fmt.Printf("  seated to match as well: %d of %d ways the mouths can "+
		"sound\n", len(alike), len(seen))
	for _, mouths := range alike {
		s := make([]string, len(mouths))
		for m, v := range mouths {
			s[m] = name(float64(v))
		}
		n, off := score(mouths)
		tag := ""
		if mouths == answered {
			tag = "  (the answer)"
		}
		fmt.Printf("    %s: %d notes, %.0f cents off in all%s\n",
			strings.Join(s, " "), n, off, tag)
	}
}

// align pairs the phrase with the aria (edit distance on the notes, the aria
// shifted back): a pair is {phrase index, aria index}, -1 for a gap.
func align(want, got []int, shift int) [][2]int {
	n, m := len(want), len(got)
	cost := make([][]int, n+1)
	for i := range cost {
		cost[i] = make([]int, m+1)
		cost[i][0] = i
	}
	for j := range m + 1 {
		cost[0][j] = j
	}
	sub := func(i, j int) int {
		if got[j]-want[i] == shift {
			return 0
		}
		return 1
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost[i][j] = min(cost[i-1][j-1]+sub(i-1, j-1),
				cost[i-1][j]+1, cost[i][j-1]+1)
		}
	}
	var out [][2]int
	for i, j := n, m; i > 0 || j > 0; {
		switch {
		case i > 0 && j > 0 && cost[i][j] == cost[i-1][j-1]+sub(i-1, j-1):
			i, j = i-1, j-1
			out = append(out, [2]int{i, j})
		case i > 0 && cost[i][j] == cost[i-1][j]+1:
			i--
			out = append(out, [2]int{i, -1})
		default:
			j--
			out = append(out, [2]int{-1, j})
		}
	}
	slices.Reverse(out)
	return out
}

// steady is the pitch of a sound that holds one note, as the median of its
// pitched frames, and the share of its audible frames that are pitched; NaN
// when fewer than half are.
func steady(fs []frame) (float64, float64) {
	loud := peak(fs) * quiet
	var ms []float64
	audible := 0
	for _, f := range fs {
		if f.rms < loud {
			continue
		}
		audible++
		if f.hz > 0 {
			ms = append(ms, midi(f.hz))
		}
	}
	share := float64(len(ms)) / float64(max(audible, 1))
	if share < 0.5 {
		return math.NaN(), share
	}
	return median(ms), share
}

// split cuts a whistled tune into its notes: a note is a run of pitched
// frames, and a run whose loudness sinks deep between two swells is two.
func split(fs []frame) []note {
	loud := peak(fs) * quiet
	var runs [][2]int // frame index ranges [from, to)
	from := -1
	for i := 0; i <= len(fs); i++ {
		on := i < len(fs) && fs[i].hz > 0 && fs[i].rms >= loud
		switch {
		case on && from < 0:
			from = i
		case !on && from >= 0:
			runs = append(runs, cutAtDips(fs, from, i)...)
			from = -1
		}
	}
	var out []note
	for _, r := range runs {
		dur := float64((r[1]-r[0])*hop) / rate22k
		if dur < minNote {
			continue
		}
		ms := make([]float64, 0, r[1]-r[0])
		for _, f := range fs[r[0]:r[1]] {
			ms = append(ms, midi(f.hz))
		}
		out = append(out, note{
			at:   float64(r[0]*hop) / rate22k,
			dur:  dur,
			midi: median(ms),
		})
	}
	return out
}

// rate22k is the rate of every sound of the bank; frames count time by it.
const rate22k = 22050

// cutAtDips splits the run [from, to) at each local minimum of loudness that
// lies under dip times the peaks on both of its sides.
func cutAtDips(fs []frame, from, to int) [][2]int {
	var out [][2]int
	start := from
	for i := from + 1; i < to-1; i++ {
		r := fs[i].rms
		if r > fs[i-1].rms || r > fs[i+1].rms {
			continue
		}
		left, right := 0.0, 0.0
		for _, f := range fs[start:i] {
			left = max(left, f.rms)
		}
		// The next swell, up to where it fades again.
		for _, f := range fs[i+1 : min(to, i+1+swell)] {
			right = max(right, f.rms)
		}
		if r < dip*left && r < dip*right {
			out = append(out, [2]int{start, i})
			start = i + 1
		}
	}
	return append(out, [2]int{start, to})
}

// swell is how far past a dip the next note's peak is looked for: 150 ms.
const swell = 13

// frames analyses a sound frame by frame.
func frames(pcm []float64, rate int) []frame {
	var out []frame
	lag := rate / minHz
	for at := 0; at+window+lag <= len(pcm); at += hop {
		x := pcm[at : at+window+lag]
		f := frame{rms: rms(x[:window])}
		if p, ap := yin(x, rate); ap < aperiodic {
			f.hz = p
		}
		out = append(out, f)
	}
	return out
}

// yin is the pitch of the window at the head of x, and how aperiodic it is
// (0 perfectly periodic, 1 noise). x holds the window and the longest lag
// after it.
func yin(x []float64, rate int) (float64, float64) {
	lo, hi := rate/maxHz, rate/minHz
	d := make([]float64, hi+1) // cumulative mean normalised difference
	d[0] = 1
	sum := 0.0
	for tau := 1; tau <= hi; tau++ {
		s := 0.0
		for j := range window {
			v := x[j] - x[j+tau]
			s += v * v
		}
		sum += s
		if sum == 0 {
			d[tau] = 1
			continue
		}
		d[tau] = s * float64(tau) / sum
	}
	deepest := lo
	for tau := lo; tau < hi; tau++ {
		if d[tau] < d[deepest] {
			deepest = tau
		}
	}
	best := deepest
	for tau := lo + 1; tau < deepest; tau++ {
		if d[tau] <= d[tau-1] && d[tau] <= d[tau+1] &&
			d[tau] <= d[deepest]+near {
			best = tau
			break
		}
	}
	// A parabola through the dip and its neighbours finds the period between
	// samples.
	t := float64(best)
	if a, b, c := d[best-1], d[best], d[best+1]; a-2*b+c != 0 {
		t += 0.5 * (a - c) / (a - 2*b + c)
	}
	return float64(rate) / t, d[best]
}

// decodeWAV reads a PCM .wav into mono samples in -1..1.
func decodeWAV(b []byte) ([]float64, int, error) {
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, 0, errors.New("not a WAVE file")
	}
	var channels, rate, bits int
	var data []byte
	for p := 12; p+8 <= len(b); {
		id := string(b[p : p+4])
		n := int(binary.LittleEndian.Uint32(b[p+4:]))
		body := b[p+8 : min(len(b), p+8+n)]
		switch {
		case id == "fmt " && len(body) >= 16:
			if binary.LittleEndian.Uint16(body) != 1 {
				return nil, 0, errors.New("not PCM")
			}
			channels = int(binary.LittleEndian.Uint16(body[2:]))
			rate = int(binary.LittleEndian.Uint32(body[4:]))
			bits = int(binary.LittleEndian.Uint16(body[14:]))
		case id == "data":
			data = body
		}
		p += 8 + n + n&1
	}
	if channels == 0 || bits != 8 && bits != 16 || rate != rate22k {
		return nil, 0, fmt.Errorf("unexpected format: %d ch, %d bit, %d Hz",
			channels, bits, rate)
	}
	step := channels * bits / 8
	out := make([]float64, len(data)/step)
	for i := range out {
		s := 0.0
		for c := range channels {
			o := i*step + c*bits/8
			if bits == 16 {
				s += float64(int16(binary.LittleEndian.Uint16(data[o:]))) /
					32768
			} else {
				s += (float64(data[o]) - 128) / 128
			}
		}
		out[i] = s / float64(channels)
	}
	return out, rate, nil
}

func rms(x []float64) float64 {
	s := 0.0
	for _, v := range x {
		s += v * v
	}
	return math.Sqrt(s / float64(len(x)))
}

func peak(fs []frame) float64 {
	p := 0.0
	for _, f := range fs {
		p = max(p, f.rms)
	}
	return p
}

func median(v []float64) float64 {
	s := slices.Clone(v)
	slices.Sort(s)
	if n := len(s); n%2 == 0 {
		return (s[n/2-1] + s[n/2]) / 2
	}
	return s[len(s)/2]
}

func midi(hz float64) float64 { return 69 + 12*math.Log2(hz/440) }

func hz(midi float64) float64 { return 440 * math.Pow(2, (midi-69)/12) }

// cents is how far a pitch is from the note it is named by.
func cents(midi float64) float64 { return (midi - math.Round(midi)) * 100 }

// steps are the twelve notes of an octave by their Russian names, sharps
// where a note has no name of its own.
var steps = [12]string{
	"до", "до-диез", "ре", "ре-диез", "ми", "фа",
	"фа-диез", "соль", "соль-диез", "ля", "ля-диез", "си",
}

// name is the nearest note, its octave numbered as in scientific pitch
// notation: до4 is middle C (до первой октавы), ля4 is 440 Hz.
func name(midi float64) string {
	m := int(math.Round(midi))
	return fmt.Sprintf("%s%d", steps[m%12], m/12-1)
}
