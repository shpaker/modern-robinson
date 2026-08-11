package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/shpaker/modern-robinson/internal/types"
)

// savePath is the save file next to the game data (the original keeps its
// SAVE.SAV there too).
func (g *Game) savePath() string {
	return filepath.Join(g.res.Root(), "nrobinson.sav")
}

// save writes the current quest state; the disk button and F5 call this.
func (g *Game) save() {
	sd := g.gs.Snapshot(g.sceneName, g.cell)
	b, err := json.MarshalIndent(sd, "", " ")
	if err != nil {
		return
	}
	if os.WriteFile(g.savePath(), b, 0o644) == nil {
		g.msg, g.msgT = "Игра сохранена", 2
	}
}

// load restores the saved state and re-enters its scene; F9 calls this.
func (g *Game) load() {
	b, err := os.ReadFile(g.savePath())
	if err != nil {
		return
	}
	var sd types.SaveData
	if json.Unmarshal(b, &sd) != nil || sd.Scene == "" {
		return
	}
	g.gs = types.Restore(sd)
	g.loadScene(sd.Scene, &sd.Cell, "", "")
	g.msg, g.msgT = "Игра загружена", 2
}
