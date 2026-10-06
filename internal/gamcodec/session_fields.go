package gamcodec

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/engine"
)

var aiCommandPowers = map[uint16]engine.PowerID{54: engine.Swamp, 22: engine.Whirlwind, 38: engine.FireRain, 62: engine.Volcano, 40: engine.Earthquake, 6: engine.FireColumn, 52: engine.Baptism, 50: engine.Batholith, 24: engine.Whirlpool, 56: engine.Tsunami, 64: engine.Storm, 78: engine.Plague, 76: engine.Wind, 72: engine.Armageddon, 36: engine.Perseus, 58: engine.Adonis, 60: engine.Heracles, 66: engine.Odysseus, 68: engine.Achilles, 70: engine.Helen}

func decodeScenarioFlags(value uint16) engine.ScenarioOptions {
	return engine.ScenarioOptions{BuildAnywhere: value&1 != 0, SeaLevelOnly: value&2 != 0, ForbidEnemyTerrain: value&4 != 0, ForbidRaise: value&8 != 0, ForbidLower: value&16 != 0, FatalWater: value&32 != 0, HideEnemy: value&64 != 0, DisableEmigration: value&128 != 0, HideDisasters: value&256 != 0, ShallowSwamps: value&512 != 0}
}
func scenarioFlags(s engine.ScenarioOptions) uint16 {
	var value uint16
	for bit, enabled := range []bool{s.BuildAnywhere, s.SeaLevelOnly, s.ForbidEnemyTerrain, s.ForbidRaise, s.ForbidLower, s.FatalWater, s.HideEnemy, s.DisableEmigration, s.HideDisasters, s.ShallowSwamps} {
		if enabled {
			value |= 1 << bit
		}
	}
	return value
}

func decodeAIStatistics(r fileReader, s *engine.Snapshot, owner int) error {
	god := 0xe76a + (owner+1)*314
	p := &s.World.Players[owner]
	p.Statistics = engine.CampaignStatistics{Population: r.long(god + 4), PeakPopulation: r.long(god + 0x3c), PeakMana: r.long(god + 0x40), Metric: r.word(god + 0x44), LeaderLosses: r.word(god + 0x46), BattleWins: r.word(god + 0x48), ScenarioOptions: r.word(god + 0x4a), WeightedPowerUse: r.word(god + 0x138)}
	p.Population, p.Towns, p.BattlesWon = int(p.Statistics.Population), int(r.word(god+0x24)), int(p.Statistics.BattleWins)
	p.Computer = r.word(god+0x1a) == 4
	p.Assisted = r.word(god+0x1a) == 18
	options := &s.World.Level.Players[owner]
	options.Groups = int(r.word(god + 0x5a))
	options.Population = int(r.word(god + 0x5c))
	options.MovementSpeed = uint8(r.word(god + 0x5e))
	options.Weapons = int(uint8(r.word(god + 0x60)))
	options.Mana = int(r.word(god + 0x62))
	for index := range options.Extra {
		options.Extra[index] = r.word(god + 0x66 + index*2)
	}
	options.Scenario = decodeScenarioFlags(r.word(0xeb2c + owner*2))
	options.ReactionDelay = int(r.word(god + 0x68))
	options.ArmageddonDeadline = int(r.word(god + 0x6a))
	magnet := r.word(god + 0x6e)
	options.FixedMagnet = int16(magnet) >= 0
	options.MagnetX, options.MagnetY = int(uint8(magnet>>8)), int(uint8(magnet))
	a := &s.World.AI[owner]
	request, err := reference(r.word(god + 0x32))
	if err != nil {
		return err
	}
	if request.Kind != engine.ActorNone && request.Kind != engine.ActorFollower {
		return fmt.Errorf("GAM crossing terrain request references a non-follower")
	}
	if request.Kind == engine.ActorFollower {
		a.TerrainRequestFollower = int(request.Index)
	}
	a.TerrainRequestX, a.TerrainRequestY = int(r.byte(god+0x35)), int(r.byte(god+0x34))
	water, err := reference(r.word(god + 0x36))
	if err != nil {
		return err
	}
	if water.Kind != engine.ActorNone && water.Kind != engine.ActorFollower {
		return fmt.Errorf("GAM water request references a non-follower")
	}
	if water.Kind == engine.ActorFollower {
		a.WaterRequestFollower = int(water.Index)
	}
	a.Reaction = int(int16(r.word(god + 0x4c)))
	a.ExpansionCooldown = int(int16(r.word(god + 0x2c)))
	a.ReleaseCooldown = int(int16(r.word(god + 0x30)))
	a.MagnetCooldown = int(int16(r.word(god + 0x28)))
	a.BestPopulation = int(int16(r.word(god + 0x20)))
	a.ChoiceIndex = int(r.word(god+0x26)) / 4
	for offset, target := range map[int]*int{0x1e: &a.BestTown, 0x2e: &a.ExpansionTown} {
		ref, err := reference(r.word(god + offset))
		if err != nil {
			return err
		}
		if ref.Kind == engine.ActorFollower {
			*target = int(ref.Index)
		}
	}
	a.ChoiceCount, a.LeaderChoiceCount = int(r.word(god+0x94)), int(r.word(god+0x96))
	if a.ChoiceCount+a.LeaderChoiceCount > len(a.Choices) || a.ChoiceCount < 0 {
		return fmt.Errorf("GAM AI choice counts are invalid")
	}
	for index := 1; index < a.ChoiceCount+a.LeaderChoiceCount; index++ {
		at := god + 0x98 + index*4
		command := r.word(at)
		power, ok := aiCommandPowers[command]
		if !ok {
			return fmt.Errorf("GAM AI command%d is not mapped", command)
		}
		target := r.word(at + 2)
		if target > 10 || target&1 != 0 {
			return fmt.Errorf("GAM AI target type is invalid")
		}
		a.Choices[index] = engine.AIPowerChoice{Power: power, Target: engine.AITargetKind(target / 2)}
	}
	prepared := r.word(god + 0x38)
	if prepared != 0 {
		power, ok := aiCommandPowers[prepared]
		if !ok {
			return fmt.Errorf("GAM prepared AI power is not mapped")
		}
		a.Prepared, a.PreparedPower = true, power
		a.PreparedTarget = engine.PowerTarget{X: int(r.byte(god + 0x3a)), Y: int(r.byte(god + 0x3b))}
	}
	return nil
}

func encodeAIStatistics(data []byte, w *engine.World, owner int) error {
	word := func(at int, value uint16) { binary.BigEndian.PutUint16(data[at-fileStart:], value) }
	long := func(at int, value uint32) { binary.BigEndian.PutUint32(data[at-fileStart:], value) }
	god := 0xe76a + (owner+1)*314
	p := w.Players[owner]
	stats := p.Statistics
	for offset, value := range map[int]uint32{4: stats.Population, 0x3c: stats.PeakPopulation, 0x40: stats.PeakMana} {
		long(god+offset, value)
	}
	for offset, value := range map[int]uint16{0x44: stats.Metric, 0x46: stats.LeaderLosses, 0x48: stats.BattleWins, 0x4a: stats.ScenarioOptions, 0x138: stats.WeightedPowerUse, 0x24: uint16(p.Towns)} {
		word(god+offset, value)
	}
	control := uint16(2)
	if p.Computer && p.Assisted {
		return fmt.Errorf("GAM player cannot be both computer-controlled and assisted")
	}
	if p.Computer {
		control = 4
	} else if p.Assisted {
		control = 18
	}
	word(god+0x1a, control)
	word(0xeb2c+owner*2, scenarioFlags(w.Level.Players[owner].Scenario))
	word(god+0x68, uint16(w.Level.Players[owner].ReactionDelay))
	word(god+0x6a, uint16(w.Level.Players[owner].ArmageddonDeadline))
	options := w.Level.Players[owner]
	if options.ReactionDelay < 0 || options.ReactionDelay > 65535 || options.ArmageddonDeadline < 0 || options.ArmageddonDeadline > 65535 || options.Weapons > 255 {
		return fmt.Errorf("GAM custom player option is outside its file range")
	}
	for offset, value := range map[int]int{0x5a: options.Groups, 0x5c: options.Population, 0x5e: int(options.MovementSpeed), 0x60: options.Weapons, 0x62: options.Mana, 0x64: options.Attrition} {
		if value < 0 || value > 65535 {
			return fmt.Errorf("GAM player template value is outside its word field")
		}
		word(god+offset, uint16(value))
	}
	for index, value := range options.Extra {
		word(god+0x66+index*2, value)
	}
	word(god+0x66, scenarioFlags(options.Scenario))
	word(god+0x68, uint16(options.ReactionDelay))
	word(god+0x6a, uint16(options.ArmageddonDeadline))
	magnet := uint16(0xffff)
	if options.FixedMagnet {
		if options.MagnetX < 0 || options.MagnetX > 63 || options.MagnetY < 0 || options.MagnetY > 63 {
			return fmt.Errorf("GAM fixed magnet target is invalid")
		}
		magnet = uint16(options.MagnetX<<8 | options.MagnetY)
	}
	word(god+0x6e, magnet)
	a := w.AI[owner]
	request := engine.ActorRef{}
	if a.TerrainRequestFollower != 0 {
		if a.TerrainRequestFollower < 1 || a.TerrainRequestFollower >= engine.FollowerCapacity || a.TerrainRequestX < 0 || a.TerrainRequestX > 63 || a.TerrainRequestY < 0 || a.TerrainRequestY > 63 {
			return fmt.Errorf("GAM crossing terrain request is invalid")
		}
		request = engine.ActorRef{Kind: engine.ActorFollower, Index: uint16(a.TerrainRequestFollower)}
	}
	requestRef, err := fileReference(request)
	if err != nil {
		return err
	}
	word(god+0x32, requestRef)
	data[god+0x34-fileStart], data[god+0x35-fileStart] = byte(a.TerrainRequestY), byte(a.TerrainRequestX)
	water := engine.ActorRef{}
	if a.WaterRequestFollower != 0 {
		if a.WaterRequestFollower < 1 || a.WaterRequestFollower >= engine.FollowerCapacity {
			return fmt.Errorf("GAM water request follower is invalid")
		}
		water = engine.ActorRef{Kind: engine.ActorFollower, Index: uint16(a.WaterRequestFollower)}
	}
	waterRef, err := fileReference(water)
	if err != nil {
		return err
	}
	word(god+0x36, waterRef)
	for offset, value := range map[int]int{0x4c: a.Reaction, 0x2c: a.ExpansionCooldown, 0x30: a.ReleaseCooldown, 0x28: a.MagnetCooldown, 0x20: a.BestPopulation, 0x26: a.ChoiceIndex * 4, 0x94: a.ChoiceCount, 0x96: a.LeaderChoiceCount} {
		word(god+offset, uint16(int16(value)))
	}
	for offset, index := range map[int]int{0x1e: a.BestTown, 0x2e: a.ExpansionTown} {
		ref := engine.ActorRef{}
		if index > 0 {
			ref = engine.ActorRef{Kind: engine.ActorFollower, Index: uint16(index)}
		}
		value, err := fileReference(ref)
		if err != nil {
			return err
		}
		word(god+offset, value)
	}
	for index := 1; index < a.ChoiceCount+a.LeaderChoiceCount; index++ {
		choice := a.Choices[index]
		command, ok := aiPowerCommand(choice.Power)
		if !ok {
			return fmt.Errorf("unsupported AI power in GAM export")
		}
		word(god+0x98+index*4, command)
		word(god+0x9a+index*4, uint16(choice.Target)*2)
	}
	if a.Prepared {
		command, ok := aiPowerCommand(a.PreparedPower)
		if !ok {
			return fmt.Errorf("unsupported prepared AI power")
		}
		word(god+0x38, command)
		data[god+0x3a-fileStart], data[god+0x3b-fileStart] = byte(a.PreparedTarget.X), byte(a.PreparedTarget.Y)
	} else {
		word(god+0x38, 0)
	}
	return nil
}
func aiPowerCommand(power engine.PowerID) (uint16, bool) {
	for command, id := range aiCommandPowers {
		if id == power {
			return command, true
		}
	}
	return 0, false
}
