package populous

// OlympianTownRules keeps the second game's economy independent of the first
// game's block IDs. Extending FirstTown through 50 would collide with the old
// rock/tree overlays, so an actor retains its own native settlement stage.
type OlympianTownRules struct {
	ManaAdd, PopulationAdd, PopulationLimit, EmigrationDivisor, WorkTicks [19]int
	Stages                                                                [27]uint8
	Footprint                                                             [49][2]int
}

// OlympianTownStage follows the three support passes and outer-clearance check
// at CODE:$133c2. The adapter translates existing flat/farm overlays into the
// native land-support test. Original linked occupancy and farm repainting are
// still independent porting targets.
func (w *World) OlympianTownStage(player, pos int) int {
	if w.OlympianTowns == nil || player < 0 || player > 1 || !inMap(pos) {
		return 0
	}
	r := w.OlympianTowns
	support := func(offset [2]int) bool {
		x, y := pos%MapWidth+offset[0], pos/MapWidth+offset[1]
		if x < 0 || y < 0 || x >= MapWidth || y >= MapHeight {
			return false
		}
		p := x + y*MapWidth
		if w.HabitatTerrainAllowed != nil && !w.HabitatTerrainAllowed(player, p) {
			return false
		}
		if w.HabitatBlocked != nil && w.HabitatBlocked(p) {
			return false
		}
		if w.MapBlk[p] != FlatBlock && int(w.MapBlk[p]) != FarmBlock+player {
			return false
		}
		if w.MapBk2[p] == RockBlock || w.MapBk2[p] == TreeBlock {
			return false
		}
		return true
	}
	count := 0
	if support(r.Footprint[0]) {
		count++
	}
	if count == 0 {
		return 0
	}
	for i := 1; i < 9; i++ {
		if support(r.Footprint[i]) {
			count++
		}
	}
	if count == 9 {
		for i := 9; i < 25; i++ {
			if support(r.Footprint[i]) {
				count++
			}
		}
		if count == 25 {
			clear := true
			for i := 25; i < 49; i++ {
				if !support(r.Footprint[i]) {
					clear = false
					break
				}
			}
			if clear {
				count = 26
			}
		}
	}
	return int(r.Stages[count])
}

func (w *World) processOlympianTown(index int, landAI bool) {
	p := &w.Peeps[index]
	player := int(p.Player)
	stage := w.OlympianTownStage(player, p.AtPos)
	if w.War || p.HeadFor != 0 || stage == 0 {
		w.setTown(index, true)
		p.Flags = OnMove
		p.Frame = 0
		p.TownStage = 0
		p.TownWork = 0
		return
	}
	p.TownStage = stage
	// The inherited terrain bookkeeping still uses its own eleven visual
	// block IDs; the renderer and economy use TownStage without collapsing it.
	oldFrame := p.Frame
	p.Frame = FirstTown + stage*10/18
	if w.MapWho[p.AtPos] == 0 {
		w.MapWho[p.AtPos] = uint16(index + 1)
	}
	w.Magnets[player].NoTowns++
	w.collectLegacyTown(index, w.OlympianTowns.PopulationLimit[stage])
	// Keep the supplied land AI active while its II-specific decision rules
	// are translated separately. Bypassing it would leave both demonstration
	// sides permanently confined to their starting settlements.
	if landAI && w.ComputerControlled[player] && w.computerActionReady(player) {
		if !p.LandComplete || oldFrame != p.Frame {
			p.LandComplete = w.computerMakeLevel(p.AtPos, player)
		}
	}
	if oldFrame != p.Frame || int(w.MapBk2[p.AtPos]) != p.Frame || w.townHasFlatFootprint(p.AtPos) {
		w.setTown(index, false)
	}
	p.TownWork++
	r := w.OlympianTowns
	if p.TownWork < r.WorkTicks[stage] {
		return
	}
	p.TownWork = 0
	if !p.Plague {
		w.Magnets[player].Mana += r.ManaAdd[stage]
	}
	grown := p.Population + r.PopulationAdd[stage]
	if grown <= r.PopulationLimit[stage] && !p.ForceEmigration {
		p.Population = grown
		return
	}
	p.ForceEmigration = false
	divisor := r.EmigrationDivisor[stage]
	if divisor <= 0 {
		return
	}
	population := grown / divisor
	if population <= 0 || population >= p.Population {
		return
	}
	// Native $11838 subtracts the new group from the old population; it does
	// not add growth to both groups. A full pool leaves the original unchanged.
	newIndex := w.AllocateHeroClone(index)
	if newIndex < 0 {
		return
	}
	w.Peeps[index].Population -= population
	w.Peeps[newIndex].Population = population
	w.initializeOlympianChild(newIndex, stage)
	w.Peeps[newIndex].TownStage = 0
	w.Peeps[newIndex].TownWork = 0
	w.Peeps[newIndex].ForceEmigration = false
	if w.Magnets[player].Carried == index+1 {
		w.Magnets[player].Carried = newIndex + 1
	}
}

// initializeOlympianChild follows $118c8: the town stage supplies weapon
// strength and twice that stage supplies search byte $18. The parent town's
// current weapon/search bytes are not inherited by this ordinary emigrant.
func (w *World) initializeOlympianChild(index, stage int) {
	w.Peeps[index].Weapons = int(uint8(stage))
	w.Peeps[index].IQ = int(uint8(stage * 2))
}
