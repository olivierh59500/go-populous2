package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
	legacy "go-populous2/internal/legacy"
)

func DecodeTownRules(exe *amiga.Executable, land Landscape) (*legacy.OlympianTownRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x13684+98 {
		return nil, fmt.Errorf("native town support tables missing")
	}
	code := exe.Hunks[0].Data
	r := &legacy.OlympianTownRules{ManaAdd: land.ManaAdd, PopulationAdd: land.PopulationAdd, PopulationLimit: land.PopulationLimit, EmigrationDivisor: land.EmigrationDivisor, WorkTicks: land.WorkTicks}
	copy(r.Stages[:], code[0x13668:0x13668+27])
	for i, stage := range r.Stages {
		if stage >= TownStages {
			return nil, fmt.Errorf("invalid settlement stage %d at support %d", stage, i)
		}
	}
	for i := range r.Footprint {
		offset := int(int16(binary.BigEndian.Uint16(code[0x13684+i*2:])))
		x := int(int8(byte(offset)))
		y := (offset - x) / 256
		if x < -3 || x > 3 || y < -3 || y > 3 {
			return nil, fmt.Errorf("invalid settlement footprint offset")
		}
		r.Footprint[i] = [2]int{x, y}
	}
	for i := range r.WorkTicks {
		if r.WorkTicks[i] < 1 || r.EmigrationDivisor[i] < 1 {
			return nil, fmt.Errorf("invalid town work/division table")
		}
	}
	return r, nil
}
