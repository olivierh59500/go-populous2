// simcheck exercises the game simulation without importing Ebitengine.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	legacy "go-populous2/internal/legacy"
	"go-populous2/internal/populous2"
)

type result struct {
	World      int    `json:"world"`
	Code       string `json:"code"`
	Terrain    int    `json:"terrain"`
	Ticks      int    `json:"ticks"`
	Population [2]int `json:"population"`
	Mana       [2]int `json:"mana"`
	Result     int    `json:"result"`
	Casts      int    `json:"casts"`
	StateHash  string `json:"state_hash"`
}

func main() {
	world := flag.Int("world", 0, "original campaign world index (0..999)")
	ticks := flag.Int("ticks", 4800, "maximum simulation steps (nominal PAL rate: 50 per second)")
	custom := flag.Bool("custom", false, "enable all 29 powers")
	flag.Parse()
	if *ticks < 0 {
		fail(fmt.Errorf("negative tick limit"))
	}
	bundle, err := populous2.Load()
	if err != nil {
		fail(err)
	}
	w, err := populous2.NewWorld(bundle, *world, *custom)
	if err != nil {
		fail(err)
	}
	w.Demo = true
	for range *ticks {
		if w.Core.ResultFor(0) != legacy.ResultOngoing {
			break
		}
		w.Tick()
	}
	data, err := json.Marshal(w.Snapshot())
	if err != nil {
		fail(err)
	}
	r := result{World: w.Level.Number, Code: w.Level.Code, Terrain: w.Level.Terrain, Ticks: w.Core.GameTurn, Population: w.Core.PlayerPopulations(), Mana: [2]int{w.Core.Magnets[0].Mana, w.Core.Magnets[1].Mana}, Result: w.Core.ResultFor(0), Casts: w.SpellSerial, StateHash: fmt.Sprintf("%x", sha256.Sum256(data))}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
