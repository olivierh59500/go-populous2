package gamcodec

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/engine"
)

func animationFrame(token, base uint16, length int) (int, error) {
	if token < base || token >= base+uint16(length*4) || (token-base)%4 != 0 {
		return 0, fmt.Errorf("animation token%d is outside its file sequence", token)
	}
	return int((token - base) / 4), nil
}
func decodeEffect(record []byte, id int, s *engine.Snapshot, catalog Catalog) error {
	word := func(at int) uint16 { return binary.BigEndian.Uint16(record[at:]) }
	owner := record[12] - 1
	if owner > 2 {
		return fmt.Errorf("invalid effect owner")
	}
	x, y := int(word(6)), int(word(8))
	vx, vy := int(int16(word(14))), int(int16(word(16)))
	timer, life := int(int16(word(20))), int(int16(word(24)))
	token := word(10)
	phase := record[22]
	reserve := engine.EffectReservation{Owner: owner, Generation: 1, LastVelocityX: vx, LastVelocityY: vy}
	class, err := decodeInspectionClass(record[0])
	if err != nil {
		return err
	}
	reserve.InspectionClass = class
	fire := engine.FireEffect{Active: true, Owner: owner, X: x, Y: y, VX: vx, VY: vy, Timer: timer, Life: life}
	switch {
	case record[0] == 38 && (phase == 0x12 || phase == 0x14):
		reserve.Kind = engine.EffectFungus
		s.World.Nature.Fungi[id] = engine.FungusController{Active: true, Collecting: phase == 0x12, Owner: owner, Wait: timer, Period: int(record[18]), MinX: int(record[6]), MinY: int(record[8]), MaxX: int(record[6]) + int(record[26]), MaxY: int(record[8]) + int(record[27]), AgeMinX: int(record[7]), AgeMinY: int(record[9]), AgeMaxX: int(record[7]) + int(record[28]), AgeMaxY: int(record[9]) + int(record[29])}
	case record[0] != 40 && record[0] != 42 && (phase == 0x22 || phase == 0x24 || phase == 0x26):
		reserve.Kind = engine.EffectEarthquake
		p := engine.QuakeGrowing
		if phase == 0x24 {
			p = engine.QuakeWaiting
		}
		if phase == 0x26 {
			p = engine.QuakeFading
		}
		s.World.Earth.Quakes[id] = engine.QuakeEffect{Active: true, Owner: owner, X: x / 256, Y: y / 256, Direction: int(record[26]), Descriptor: int(token), Life: life, Delay: timer, Phase: p}
	case record[0] == 50 && (phase == 0x28 || phase == 0x2a):
		reserve.Kind = engine.EffectTidalWave
		direction := word(26)
		if direction > 12 || direction%4 != 0 {
			return fmt.Errorf("invalid tidal direction")
		}
		names := [4]string{"tidal/north", "tidal/east", "tidal/south", "tidal/west"}
		frame, found := 0, false
		for _, role := range catalog.AnimationRoles[token] {
			if role.Name == names[direction/4] {
				frame, found = role.Frame, true
				break
			}
		}
		if !found {
			return fmt.Errorf("tidal animation token has no semantic catalog role")
		}
		s.World.Water.Waves[id] = engine.TidalEffect{Active: true, Newborn: phase == 0x28, Owner: owner, X: x, Y: y, Direction: int(direction / 4), Frame: frame}
	case record[0] == 40 && (phase == 22 || phase == 26):
		reserve.Kind = engine.EffectLightning
		p := engine.LightningAppearing
		base, length := uint16(1760), 5
		if phase == 26 {
			p, base = engine.LightningDisappearing, 1824
		} else if token >= 1784 && token < 1820 {
			p, base, length = engine.LightningSteady, 1784, 9
		}
		frame, err := animationFrame(token, base, length)
		if err != nil {
			return err
		}
		first, err := effectIndex(word(26))
		if err != nil {
			return err
		}
		s.World.Air.Markers[id] = engine.LightningMarker{Active: true, Owner: owner, X: x, Y: y, Life: life, Phase: p, Frame: frame, FirstBolt: first}
	case record[0] == 42 && phase == 24:
		reserve.Kind = engine.EffectLightning
		next, err := effectIndex(word(26))
		if err != nil {
			return err
		}
		marker, err := effectIndex(word(28))
		if err != nil {
			return err
		}
		if marker == 0 {
			return fmt.Errorf("lightning bolt has no marker")
		}
		s.World.Air.Bolts[id] = engine.LightningBolt{Active: true, Owner: owner, X: x, Y: y, Marker: marker, Next: next, Random: word(30)}
	case record[0] == 34 && (phase == 2 || phase == 4 || phase == 6):
		reserve.Kind = engine.EffectFireColumn
		base, length := uint16(416), 9
		fire.Phase = engine.FireEmerging
		if phase == 4 {
			base, length, fire.Phase = 1208, 3, engine.FireMoving
		}
		if phase == 6 {
			base, length, fire.Phase = 1632, 10, engine.FireEnding
		}
		frame, err := animationFrame(token, base, length)
		if err != nil {
			return err
		}
		fire.Frame = frame
		s.World.Fire.Columns[id] = fire
	case record[0] == 32 && (phase == 8 || phase == 10 || phase == 12):
		reserve.Kind = engine.EffectWhirlwind
		base, length, p := uint16(1224), 2, engine.WhirlwindAppearing
		if phase == 10 {
			p = engine.WhirlwindMoving
		}
		if phase == 12 {
			base, length, p = 1740, 4, engine.WhirlwindDisappearing
		}
		frame, err := animationFrame(token, base, length)
		if err != nil {
			return err
		}
		s.World.Air.Whirlwinds[id] = engine.WhirlwindEffect{Active: true, Owner: owner, X: x, Y: y, VX: vx, VY: vy, Life: life, Timer: timer, Phase: p, Frame: frame}
	case phase == 0x1c || phase == 0x1e || phase == 0x20:
		reserve.Kind = engine.EffectFireRain
		base, length := uint16(2076), 18
		fire.Phase = engine.MeteorWaiting
		if phase == 0x1e {
			fire.Phase = engine.MeteorFalling
		}
		if phase == 0x20 {
			fire.Phase = engine.MeteorImpact
			base, length = 1180, 4
			if token >= 1516 && token < 1532 {
				base = 1516
				fire.WaterImpact = true
			}
		}
		frame, err := animationFrame(token, base, length)
		if err != nil {
			return err
		}
		fire.Frame = frame
		s.World.Fire.Rain[id] = fire
	case phase == 0x2c || phase == 0x2e:
		reserve.Kind = engine.EffectStorm
		frame, err := animationFrame(token, 3300, 4)
		if err != nil {
			return err
		}
		e := engine.StormEffect{Active: true, Ending: phase == 0x2e, Owner: owner, X: x, Y: y, Life: life, Timer: timer, Frame: frame}
		extra := word(26)
		if extra != 0 {
			base := uint16(1180)
			if extra >= 1516 && extra < 1532 {
				base = 1516
				e.WaterImpact = true
			}
			e.ImpactFrame, err = animationFrame(extra, base, 4)
			if err != nil {
				return err
			}
			e.ImpactActive = true
		}
		s.World.Air.Storms[id] = e
	case phase == 0x30 || phase == 0x32:
		reserve.Kind = engine.EffectVolcano
		fire.Phase = engine.VolcanoGrowing
		if phase == 0x32 {
			fire.Phase = engine.VolcanoFinished
		}
		if timer%4 != 0 || timer < 0 || timer/4 > 8 {
			return fmt.Errorf("invalid volcano file stage")
		}
		fire.Stage = timer / 4
		s.World.Fire.Volcano[id] = fire
	case phase == 0x34 || phase == 0x36:
		reserve.Kind = engine.EffectLava
		fire.Phase = engine.LavaFlowing
		direction := word(26)
		if direction > 6 || direction&1 != 0 {
			return fmt.Errorf("invalid lava file direction")
		}
		fire.Direction = int(direction / 2)
		fire.Frame = int(token&4) / 4
		s.World.Fire.Lava[id] = fire
	case record[0] == 48 && (phase == 0x38 || phase == 0x3a):
		reserve.Kind = engine.EffectBasalt
		direction := word(26)
		if direction > 6 || direction&1 != 0 {
			return fmt.Errorf("invalid basalt direction")
		}
		frame, err := animationFrame(token, 1516, 4)
		if err != nil {
			return err
		}
		s.World.Water.Basalt[id] = engine.BasaltEffect{Active: true, Owner: owner, X: x / 256, Y: y / 256, Direction: int(direction / 2), Life: life, Delay: timer, Frame: frame}
	case record[0] == 36 && phase == 0x0e:
		reserve.Kind = engine.EffectWhirlpool
		if token < 151 || token > 163 || (token-151)%4 != 0 {
			return fmt.Errorf("invalid whirlpool tile phase")
		}
		s.World.Water.Whirlpools[id] = engine.WhirlpoolEffect{Active: true, Owner: owner, X: x / 256, Y: y / 256, Life: life, Delay: timer, Frame: int(token-151) / 4}
	case phase == 0x3c:
		reserve.Kind = engine.EffectHurricane
		direction := word(26)
		if direction > 6 || direction&1 != 0 {
			return fmt.Errorf("invalid wind direction")
		}
		s.World.Wind[id] = engine.WindEffect{Active: true, Owner: owner, X: x / 256, Y: y / 256, Direction: int(direction / 2), Life: life}
	default:
		return fmt.Errorf("effect%d kind%d phase%d is not mapped yet", id, record[0], phase)
	}
	s.Reservations[id] = reserve
	return nil
}

func effectIndex(value uint16) (int, error) {
	ref, err := reference(value)
	if err != nil {
		return 0, err
	}
	if ref.Kind == engine.ActorNone {
		return 0, nil
	}
	if ref.Kind != engine.ActorEffect {
		return 0, fmt.Errorf("GAM effect relation points into another pool")
	}
	return int(ref.Index) + 1, nil
}

func effectReference(index int) (int, error) {
	if index == 0 {
		return 0, nil
	}
	if index < 0 || index > engine.EffectCapacity {
		return 0, fmt.Errorf("GAM effect index is invalid")
	}
	ref, err := fileReference(engine.ActorRef{Kind: engine.ActorEffect, Index: uint16(index - 1)})
	return int(ref), err
}

func encodeEffect(w *engine.World, id int, kind engine.EffectKind, catalog Catalog) ([32]byte, error) {
	var record [32]byte
	word := func(at int, value int) { binary.BigEndian.PutUint16(record[at:], uint16(int16(value))) }
	common := func(owner uint8, x, y, vx, vy, life, timer int) {
		record[12] = owner + 1
		word(6, x)
		word(8, y)
		word(14, vx)
		word(16, vy)
		word(20, timer)
		word(24, life)
	}
	switch kind {
	case engine.EffectFungus:
		e := w.Nature.Fungi[id]
		record[0], record[12], record[18], record[22] = 38, e.Owner+1, uint8(e.Period), 20
		if e.Collecting {
			record[22] = 18
		}
		record[6], record[8], record[7], record[9] = uint8(e.MinX), uint8(e.MinY), uint8(e.AgeMinX), uint8(e.AgeMinY)
		record[26], record[27], record[28], record[29] = uint8(e.MaxX-e.MinX), uint8(e.MaxY-e.MinY), uint8(e.AgeMaxX-e.AgeMinX), uint8(e.AgeMaxY-e.AgeMinY)
		word(20, e.Wait)
	case engine.EffectEarthquake:
		e := w.Earth.Quakes[id]
		common(e.Owner, e.X*256, e.Y*256, 0, 0, e.Life, e.Delay)
		record[22] = 34
		if e.Phase == engine.QuakeWaiting {
			record[22] = 36
		}
		if e.Phase == engine.QuakeFading {
			record[22] = 38
		}
		record[26] = uint8(e.Direction)
		word(10, e.Descriptor)
	case engine.EffectTidalWave:
		e := w.Water.Waves[id]
		common(e.Owner, e.X, e.Y, 0, 0, 0, 0)
		record[0], record[22] = 50, 42
		if e.Newborn {
			record[22] = 40
		}
		word(26, e.Direction*4)
		name := "tidal/" + [4]string{"north", "east", "south", "west"}[e.Direction]
		found := false
		for token, roles := range catalog.AnimationRoles {
			for _, role := range roles {
				if role.Name == name && role.Frame == e.Frame {
					word(10, int(token))
					found = true
					break
				}
			}
		}
		if !found {
			return record, fmt.Errorf("tidal artwork token is unavailable")
		}
	case engine.EffectLightning:
		if e := w.Air.Markers[id]; e.Active {
			common(e.Owner, e.X, e.Y, 0, 0, e.Life, 0)
			record[0], record[22] = 40, 22
			base := 1760
			if e.Phase == engine.LightningSteady {
				base = 1784
			}
			if e.Phase == engine.LightningDisappearing {
				base = 1824
				record[22] = 26
			}
			word(10, base+e.Frame*4)
			first, err := effectReference(e.FirstBolt)
			if err != nil {
				return record, err
			}
			word(26, first)
		} else {
			e := w.Air.Bolts[id]
			common(e.Owner, e.X, e.Y, 0, 0, 0, 0)
			record[0], record[22] = 42, 24
			next, err := effectReference(e.Next)
			if err != nil {
				return record, err
			}
			marker, err := effectReference(e.Marker)
			if err != nil {
				return record, err
			}
			word(26, next)
			word(28, marker)
			word(30, int(e.Random))
		}
	case engine.EffectFireColumn:
		e := w.Fire.Columns[id]
		common(e.Owner, e.X, e.Y, e.VX, e.VY, e.Life, e.Timer)
		record[0], record[18] = 34, 16
		base, phase := 416, 2
		if e.Phase == engine.FireMoving {
			base, phase = 1208, 4
		}
		if e.Phase == engine.FireEnding {
			base, phase = 1632, 6
		}
		record[22] = uint8(phase)
		word(10, base+e.Frame*4)
	case engine.EffectWhirlwind:
		e := w.Air.Whirlwinds[id]
		common(e.Owner, e.X, e.Y, e.VX, e.VY, e.Life, e.Timer)
		record[0], record[18] = 32, 24
		base, phase := 1224, 8
		if e.Phase == engine.WhirlwindMoving {
			phase = 10
		}
		if e.Phase == engine.WhirlwindDisappearing {
			base, phase = 1740, 12
		}
		record[22] = uint8(phase)
		word(10, base+e.Frame*4)
	case engine.EffectFireRain:
		e := w.Fire.Rain[id]
		common(e.Owner, e.X, e.Y, 0, 0, e.Life, e.Timer)
		record[0] = 44
		base, phase := 2076, 28
		if e.Phase == engine.MeteorFalling {
			phase = 30
		}
		if e.Phase == engine.MeteorImpact {
			base, phase = 1180, 32
			if e.WaterImpact {
				base = 1516
			}
		}
		record[22] = uint8(phase)
		word(10, base+e.Frame*4)
	case engine.EffectStorm:
		e := w.Air.Storms[id]
		common(e.Owner, e.X, e.Y, 0, 0, e.Life, e.Timer)
		record[0], record[22] = 54, 44
		if e.Ending {
			record[22] = 46
		}
		word(10, 3300+e.Frame*4)
		if e.ImpactActive {
			base := 1180
			if e.WaterImpact {
				base = 1516
			}
			word(26, base+e.ImpactFrame*4)
		}
	case engine.EffectVolcano:
		e := w.Fire.Volcano[id]
		common(e.Owner, e.X, e.Y, 0, 0, 0, e.Stage*4)
		record[22] = 48
		if e.Phase == engine.VolcanoFinished {
			record[22] = 50
		}
	case engine.EffectLava:
		e := w.Fire.Lava[id]
		common(e.Owner, e.X, e.Y, 0, 0, e.Life, e.Timer)
		record[0], record[22] = 56, 52
		word(26, e.Direction*2)
		name := "lava/slope-" + fmt.Sprint(w.Cell(e.X/256, e.Y/256).Shape)
		if w.Cell(e.X/256, e.Y/256).Shape == 15 {
			name = "lava/" + [4]string{"north", "east", "south", "west"}[e.Direction]
		}
		found := false
		for token, roles := range catalog.AnimationRoles {
			for _, role := range roles {
				if role.Name == name && role.Frame == e.Frame {
					word(10, int(token))
					found = true
					break
				}
			}
		}
		if !found {
			return record, fmt.Errorf("lava artwork token is unavailable")
		}
	case engine.EffectBasalt:
		e := w.Water.Basalt[id]
		common(e.Owner, e.X*256+128, e.Y*256+128, 0, 0, e.Life, e.Delay)
		record[0], record[22] = 48, 56
		word(10, 1516+e.Frame*4)
		word(26, e.Direction*2)
	case engine.EffectWhirlpool:
		e := w.Water.Whirlpools[id]
		common(e.Owner, e.X*256, e.Y*256, 0, 0, e.Life, e.Delay)
		record[0], record[22], record[18] = 36, 14, 16
		word(10, 151+e.Frame*4)
	case engine.EffectHurricane:
		e := w.Wind[id]
		common(e.Owner, e.X*256, e.Y*256, 0, 0, e.Life, 0)
		record[22], record[18] = 60, 16
		word(26, e.Direction*2)
	default:
		return record, fmt.Errorf("GAM effect%d family%d is not mapped yet", id, kind)
	}
	return record, nil
}

func effectFileFields(kind engine.EffectKind, w *engine.World, id int) [][2]int {
	common := [][2]int{{6, 4}, {12, 1}, {20, 2}, {22, 1}, {24, 2}}
	switch kind {
	case engine.EffectFungus:
		return [][2]int{{0, 1}, {6, 4}, {12, 1}, {18, 1}, {20, 2}, {22, 1}, {26, 4}}
	case engine.EffectEarthquake:
		return [][2]int{{6, 4}, {10, 2}, {12, 1}, {20, 2}, {22, 1}, {24, 2}, {26, 1}}
	case engine.EffectTidalWave:
		return append(common, [2]int{0, 1}, [2]int{10, 2}, [2]int{26, 2})
	case engine.EffectFireColumn, engine.EffectWhirlwind:
		return append(common, [2]int{0, 1}, [2]int{10, 2}, [2]int{14, 5})
	case engine.EffectFireRain:
		return append(common, [2]int{0, 1}, [2]int{10, 2})
	case engine.EffectStorm:
		return append(common, [2]int{0, 1}, [2]int{10, 2}, [2]int{26, 2})
	case engine.EffectLightning:
		if w.Air.Bolts[id].Active {
			return [][2]int{{0, 1}, {6, 7}, {22, 1}, {26, 6}}
		}
		return append(common, [2]int{0, 1}, [2]int{10, 2}, [2]int{26, 2})
	case engine.EffectVolcano:
		return [][2]int{{6, 1}, {8, 1}, {12, 1}, {20, 2}, {22, 1}}
	case engine.EffectLava, engine.EffectBasalt:
		return append(common, [2]int{0, 1}, [2]int{10, 2}, [2]int{26, 2})
	case engine.EffectWhirlpool:
		return append(common, [2]int{0, 1}, [2]int{10, 2}, [2]int{18, 1})
	case engine.EffectHurricane:
		return append(common, [2]int{18, 1}, [2]int{26, 2})
	}
	return common
}
