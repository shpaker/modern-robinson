package repositories

import (
	"encoding/binary"
	"strings"
)

// bgiRecord is one object's initial state inside BEGIN.BGI: a 20-byte
// NUL-padded name followed by four int32 fields (gx, gy, z, visible).
type bgiRecord struct {
	Name    string
	Visible bool
}

const bgiRecSize = 36

// bgiName validates and extracts a 20-byte NUL-padded object name.
func bgiName(b []byte) (string, bool) {
	if len(b) < 20 {
		return "", false
	}
	end := 0
	for end < 20 && b[end] != 0 {
		end++
	}
	if end == 0 || end == 20 {
		return "", false
	}
	for _, c := range b[:end] {
		lc := c | 0x20
		if (lc < 'a' || lc > 'z') && (c < '0' || c > '9') && c != '_' {
			return "", false
		}
	}
	for _, c := range b[end:20] {
		if c != 0 {
			return "", false
		}
	}
	return string(b[:end]), true
}

// parseBGI scans BEGIN.BGI for contiguous clusters of object records. Each
// cluster is one scene's ObjectList initial state, in STARTUP.INF scene order;
// callers match clusters to scenes by name overlap.
func parseBGI(d []byte) [][]bgiRecord {
	var clusters [][]bgiRecord
	var cur []bgiRecord
	lastEnd := -1
	off := 0
	for off+bgiRecSize <= len(d) {
		nm, ok := bgiName(d[off : off+20])
		if !ok {
			off += 4
			continue
		}
		gx := int32(binary.LittleEndian.Uint32(d[off+20:]))
		gy := int32(binary.LittleEndian.Uint32(d[off+24:]))
		if gx < 0 || gx > 63 || gy < 0 || gy > 63 {
			off += 4
			continue
		}
		vis := int32(binary.LittleEndian.Uint32(d[off+32:]))
		if off != lastEnd && cur != nil {
			clusters = append(clusters, cur)
			cur = nil
		}
		cur = append(cur, bgiRecord{Name: strings.ToLower(nm), Visible: vis != 0})
		off += bgiRecSize
		lastEnd = off
	}
	if cur != nil {
		clusters = append(clusters, cur)
	}
	return clusters
}

// InitialVisibility returns the start-of-game visibility for a scene's objects
// from DATA/BEGIN.BGI, keyed by lower-case object name. Objects absent from
// the map (or scenes without a BGI cluster) default to visible. The cluster
// belonging to the scene is picked by best name overlap with objectNames.
func (r *Resources) InitialVisibility(objectNames []string) map[string]bool {
	if r.bgi == nil {
		d, err := readFileUpper(r.root, "DATA", "BEGIN.BGI")
		if err != nil {
			r.bgi = [][]bgiRecord{}
		} else {
			r.bgi = parseBGI(d)
		}
	}
	want := map[string]bool{}
	for _, n := range objectNames {
		want[strings.ToLower(n)] = true
	}
	best, bestHit := -1, 0
	for i, cl := range r.bgi {
		hit := 0
		for _, rec := range cl {
			if want[rec.Name] {
				hit++
			}
		}
		// Prefer the cluster covering most of the scene's objects; require a
		// clear majority to avoid mismatches between similar scenes.
		if hit > bestHit {
			best, bestHit = i, hit
		}
	}
	out := map[string]bool{}
	if best < 0 || bestHit*2 < len(objectNames) {
		return out
	}
	for _, rec := range r.bgi[best] {
		out[rec.Name] = rec.Visible
	}
	return out
}
