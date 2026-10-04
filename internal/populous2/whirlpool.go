package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

const WhirlpoolActorKind uint8 = 0x24

type WhirlpoolRules struct {
	BaseLife       int
	Speed          uint8
	SoundCue       int
	Footprint      [4]uint16
	Neighbors      [8]uint16
	CornerWords    [24]uint16
	TileProperties [256]uint16
}

func DecodeWhirlpoolRules(exe *amiga.Executable) (WhirlpoolRules, error) {
	var rules WhirlpoolRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return rules, fmt.Errorf("native whirlpool tables missing")
	}
	code := exe.Hunks[0].Data
	rules.BaseLife = int(binary.BigEndian.Uint16(code[0x20d4c:]))
	rules.Speed = code[0x20d4f]
	rules.SoundCue = (0x18a8a - 0x185a8) / 10
	if rules.BaseLife < 1 || rules.BaseLife > 32000 || rules.Speed == 0 {
		return WhirlpoolRules{}, fmt.Errorf("invalid native whirlpool lifetime or speed")
	}
	for index := range rules.Footprint {
		rules.Footprint[index] = binary.BigEndian.Uint16(code[0x20ed0+index*2:])
	}
	if binary.BigEndian.Uint16(code[0x20ed8:]) != 0xff9d {
		return WhirlpoolRules{}, fmt.Errorf("native whirlpool footprint terminator missing")
	}
	for index := range rules.Neighbors {
		rules.Neighbors[index] = binary.BigEndian.Uint16(code[0x20dd6+index*2:])
	}
	for index := range rules.CornerWords {
		// $14de0 retries an out-of-bounds corner without decrementing the
		// four-call counter. The read can enter the following release table.
		rules.CornerWords[index] = binary.BigEndian.Uint16(code[0x20eda+index*2:])
	}
	for index := range rules.TileProperties {
		rules.TileProperties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
	}
	// Prove that the bounded word window covers every reachable map position.
	for y := range 64 {
		for x := range 64 {
			origin := uint16(x | y<<8)
			for quadrant, footprint := range rules.Footprint {
				if (origin+footprint)&0xc0c0 != 0 {
					continue
				}
				at, accepted := quadrant*4, 0
				for accepted < 4 {
					if at >= len(rules.CornerWords) {
						return WhirlpoolRules{}, fmt.Errorf("native whirlpool corner window exceeds decoded words")
					}
					if (origin+rules.CornerWords[at])&0xc0c0 == 0 {
						accepted++
					}
					at++
				}
			}
		}
	}
	return rules, nil
}

// Create translates $15cc8. All four parcels must have exact tile code zero,
// including when another animated whirlpool covers otherwise level water.
// Creation stamps the first terrain frame and consumes no random numbers.
func (rules WhirlpoolRules) Create(pool *[NativeEffectCapacity]NativeEffectActor, player, x, y int, waterExperience uint8, readTile func(int, int) uint8, writeTile func(int, int, uint8)) bool {
	if pool == nil || player < 0 || player > 1 || !inside(x, y) || readTile == nil || writeTile == nil {
		return false
	}
	origin := uint16(x | y<<8)
	for _, offset := range rules.Footprint {
		parcel := origin + offset
		if parcel&0xc0c0 != 0 || readTile(int(uint8(parcel)), int(uint8(parcel>>8))) != 0 {
			return false
		}
	}
	for index := range pool {
		actor := &pool[index]
		if actor.Active {
			continue
		}
		vx, vy := actor.VX, actor.VY
		*actor = NativeEffectActor{Active: true, Kind: WhirlpoolActorKind, Player: uint8(player), X: int16(x * 256), Y: int16(y * 256), VX: vx, VY: vy, Speed: rules.Speed, Timer: int16(rules.Speed), Life: int16(rules.BaseLife + int(waterExperience)), State: 0x0e, Animation: 0x97}
		for quadrant, offset := range rules.Footprint {
			parcel := origin + offset
			if rules.TileProperties[readTile(int(uint8(parcel)), int(uint8(parcel>>8)))]&8 != 0 {
				writeTile(int(uint8(parcel)), int(uint8(parcel>>8)), uint8(actor.Animation+quadrant+1))
			}
		}
		return true
	}
	return false
}

type WhirlpoolCallbacks struct {
	ReadTile  func(int, int) uint8
	WriteTile func(int, int, uint8)
	// ReadGridByte addresses the native four-byte parcel grid at $f44. The
	// controller's $14d74 barrier check deliberately uses packedTile+1 as a
	// raw byte offset, without converting the tile coordinate to a cell index.
	ReadGridByte func(uint16) uint8
	// LowerVertex runs the direct, unpriced native terrain lowering operation,
	// including its geometry propagation. It is separate from player admission.
	LowerVertex  func(int, int)
	Random       func() int
	ViewX, ViewY int
}

type WhirlpoolStep struct {
	Moved, Finished bool
	// SoundCue is -1 outside the native 8 by 8 view, otherwise descriptor 125.
	// The visible trigger precedes life expiry and can occur on the last tick.
	SoundCue int
}

// Tick translates $14cae-$14e12. Whirlpool frames are four terrain tiles,
// with bases $97/$9b/$9f/$a3; these are not sprite animation-table offsets.
func (rules WhirlpoolRules) Tick(actor *NativeEffectActor, callbacks WhirlpoolCallbacks) (WhirlpoolStep, error) {
	step := WhirlpoolStep{SoundCue: -1}
	if actor == nil || !actor.Active {
		return step, nil
	}
	if actor.Kind != WhirlpoolActorKind || actor.State != 0x0e || callbacks.ReadTile == nil || callbacks.WriteTile == nil || callbacks.ReadGridByte == nil || callbacks.LowerVertex == nil || callbacks.Random == nil {
		return step, fmt.Errorf("invalid native whirlpool actor or callbacks")
	}
	x, y := int(uint16(actor.X)>>8), int(uint16(actor.Y)>>8)
	if !inside(x, y) {
		return step, fmt.Errorf("native whirlpool position outside map")
	}
	if x >= callbacks.ViewX && x < callbacks.ViewX+8 && y >= callbacks.ViewY && y < callbacks.ViewY+8 {
		step.SoundCue = rules.SoundCue
	}
	oldFrame := actor.Animation
	actor.Animation += 4
	if actor.Animation >= 0xa7 {
		actor.Animation = 0x97
	}
	origin := uint16(x | y<<8)
	for quadrant, offset := range rules.Footprint {
		parcel := origin + offset
		if parcel&0xc0c0 != 0 {
			continue
		}
		xx, yy := int(uint8(parcel)), int(uint8(parcel>>8))
		if callbacks.ReadTile(xx, yy) == uint8(oldFrame+quadrant+1) {
			callbacks.WriteTile(xx, yy, 0)
		}
	}
	previousLife := actor.Life
	actor.Life--
	// Native SUBI.W/BGT uses the overflow flag. Comparing the original signed
	// word also preserves its -32768 boundary, where the stored result wraps.
	if previousLife <= 1 {
		actor.Active = false
		step.Finished = true
		return step, nil
	}
	previousTimer := actor.Timer
	actor.Timer--
	if previousTimer <= 1 {
		// $14d4a replaces only the low byte of the decremented timer word.
		actor.Timer = int16(uint16(actor.Timer)&0xff00 | uint16(actor.Speed))
		neighbor := rules.Neighbors[(uint16(callbacks.Random())&0x0e)/2]
		candidate := origin + neighbor
		if candidate&0xc0c0 == 0 && callbacks.ReadGridByte(candidate+1) != 0xe0 {
			actor.X = int16(uint16(uint8(candidate))<<8 | uint16(actor.X)&0xff)
			actor.Y = int16(candidate & 0xff00)
			step.Moved = candidate != origin
			origin = candidate
		}
	}
	for quadrant, offset := range rules.Footprint {
		parcel := origin + offset
		if parcel&0xc0c0 != 0 {
			continue
		}
		xx, yy := int(uint8(parcel)), int(uint8(parcel>>8))
		if rules.TileProperties[callbacks.ReadTile(xx, yy)]&8 != 0 {
			callbacks.WriteTile(xx, yy, uint8(actor.Animation+quadrant+1))
			continue
		}
		at, lowered := quadrant*4, 0
		for lowered < 4 {
			if at >= len(rules.CornerWords) {
				return step, fmt.Errorf("native whirlpool corner window exhausted")
			}
			vertex := origin + rules.CornerWords[at]
			at++
			if vertex&0xc0c0 != 0 {
				continue
			}
			callbacks.LowerVertex(int(uint8(vertex)), int(uint8(vertex>>8)))
			lowered++
		}
		// Both branches reach $14e12 after the first coastline parcel. Even
		// when lowering made it water, later quadrants wait for another tick.
		if rules.TileProperties[callbacks.ReadTile(xx, yy)]&8 != 0 {
			callbacks.WriteTile(xx, yy, uint8(actor.Animation+quadrant+1))
		}
		return step, nil
	}
	return step, nil
}
