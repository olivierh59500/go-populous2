// Package gamcodec converts original Populous II save files at the file
// boundary. Simulation state remains ordinary engine values after conversion.
package gamcodec

import (
	"encoding/binary"
	"fmt"
	"strings"

	"go-populous2/internal/engine"
)

const FileSize = 56690
const fileStart = 0x0dd6

type AnimationRole struct {
	Name  string
	Frame int
}
type Catalog struct {
	Levels         []engine.Level
	Landscapes     [4]engine.Landscape
	Geometry       [256]uint8
	AnimationRoles map[uint16][]AnimationRole
}

// Metadata belongs to the file codec only. Original preserves reserved save
// fields so a no-change export need not invent their values. These user-data
// bytes are never interpreted as instructions or attached to the engine.
type Metadata struct {
	Original              []byte
	Profile               engine.Deity
	ProfileSide, GameMode int
	CameraX, CameraY      int
	initial               engine.Snapshot
	initialProfile        engine.Deity
	Catalog               Catalog
}
type Document struct {
	World    *engine.World
	Metadata Metadata
}

type fileReader struct{ data []byte }

func (r fileReader) byte(at int) uint8  { return r.data[at-fileStart] }
func (r fileReader) word(at int) uint16 { return binary.BigEndian.Uint16(r.data[at-fileStart:]) }
func (r fileReader) long(at int) uint32 { return binary.BigEndian.Uint32(r.data[at-fileStart:]) }
func (r fileReader) record(base, index, stride int) []byte {
	return r.data[base+index*stride-fileStart : base+(index+1)*stride-fileStart]
}

func reference(value uint16) (engine.ActorRef, error) {
	if value == 0 {
		return engine.ActorRef{}, nil
	}
	at := 0x76c0 + int(int16(value))
	for _, pool := range []struct {
		kind                engine.ActorKind
		base, count, stride int
	}{{engine.ActorWall, 0x5f50, 200, 16}, {engine.ActorScenery, 0x6bd0, 200, 14}, {engine.ActorFollower, 0x76c0, 400, 52}, {engine.ActorEffect, 0xc800, 250, 32}, {engine.ActorMagnet, 0xe74e, 2, 14}} {
		if at >= pool.base && at < pool.base+pool.count*pool.stride && (at-pool.base)%pool.stride == 0 {
			return engine.ActorRef{Kind: pool.kind, Index: uint16((at - pool.base) / pool.stride)}, nil
		}
	}
	return engine.ActorRef{}, fmt.Errorf("GAM actor reference %04x has no supported file record", value)
}
func fileReference(ref engine.ActorRef) (uint16, error) {
	if ref.Kind == engine.ActorNone {
		return 0, nil
	}
	for _, pool := range []struct {
		kind                engine.ActorKind
		base, count, stride int
	}{{engine.ActorWall, 0x5f50, 200, 16}, {engine.ActorScenery, 0x6bd0, 200, 14}, {engine.ActorFollower, 0x76c0, 400, 52}, {engine.ActorEffect, 0xc800, 250, 32}, {engine.ActorMagnet, 0xe74e, 2, 14}} {
		if ref.Kind == pool.kind && int(ref.Index) < pool.count {
			return uint16(int16(pool.base + int(ref.Index)*pool.stride - 0x76c0)), nil
		}
	}
	return 0, fmt.Errorf("unsupported semantic GAM actor reference")
}

func Decode(data []byte, catalog Catalog) (*Document, error) {
	if len(data) < FileSize {
		return nil, fmt.Errorf("GAM save contains %d bytes; expected at least%d", len(data), FileSize)
	}
	r := fileReader{data[:FileSize]}
	level, land, side, mode := int(r.word(0xeb46)), int(r.word(0xeb22)), int(r.word(0xeb42)), int(r.word(0xeb44))
	if level < 0 || level >= len(catalog.Levels) || land > 3 || side < 1 || side > 2 {
		return nil, fmt.Errorf("GAM campaign/profile/landscape is invalid")
	}
	if mode != 2 && mode != 4 && mode != 6 && mode != 8 && mode != 10 {
		return nil, fmt.Errorf("unsupported GAM game mode%d", mode)
	}
	w, err := engine.NewWorld(catalog.Levels[level], catalog.Landscapes[land])
	if err != nil {
		return nil, err
	}
	snapshot := w.Snapshot()
	snapshot.World.Level.Landscape = land
	snapshot.World.Level.Seed = r.long(0xeb24)
	snapshot.World.Landscape = catalog.Landscapes[land]
	snapshot.World.Tick = uint64(r.long(0xf40))
	snapshot.Random = r.long(0xeb28)
	snapshot.World.Followers = [engine.FollowerCapacity]engine.Follower{}
	snapshot.Motion = [engine.FollowerCapacity]engine.FollowerMotionSnapshot{}
	snapshot.World.Actors = engine.ActorRegistry{}
	snapshot.World.Occupants = [engine.MapSize * engine.MapSize]uint16{}
	snapshot.World.Nature.Scenery = [engine.SceneryCapacity]engine.SceneryActor{}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			at := x + y*64
			base := r.byte(0xf44+at*4) & 7
			tile := r.byte(0xf45 + at*4)
			shape := catalog.Geometry[tile] & 15
			corners := [4]uint8{}
			for i := range corners {
				corners[i] = base + ((shape >> i) & 1)
			}
			snapshot.World.Tiles[at] = engine.Cell{BaseAltitude: base, Shape: shape, Code: tile, Corners: corners}
			vertex := x + y*65
			snapshot.World.Heights[vertex] = corners[0]
			if x == 63 {
				snapshot.World.Heights[vertex+1] = corners[1]
			}
			if y == 63 {
				snapshot.World.Heights[vertex+65] = corners[3]
			}
			if x == 63 && y == 63 {
				snapshot.World.Heights[vertex+66] = corners[2]
			}
			snapshot.World.Pressure[at] = r.byte(0x4f44 + at)
			if tile == 47 {
				snapshot.World.Farms[at] = 1
			} else if tile == 63 {
				snapshot.World.Farms[at] = 2
			} else {
				snapshot.World.Farms[at] = 0
			}
			switch tile {
			case 95:
				snapshot.World.Nature.Ground[at] = engine.GroundParcel{Mark: engine.GroundScorched}
			case 245:
				snapshot.World.Nature.Ground[at] = engine.GroundParcel{Mark: engine.GroundFlowers}
			case 168, 169, 170, 171:
				snapshot.World.Nature.Ground[at] = engine.GroundParcel{Mark: engine.GroundSwamp}
			}
		}
	}
	for id := 1; id < engine.FollowerCapacity; id++ {
		record := r.record(0x76c0, id, 52)
		owner := record[12]
		if owner == 0 {
			continue
		}
		if owner > 2 {
			return nil, fmt.Errorf("GAM follower%d uses unsupported neutral record kind%d", id, record[0])
		}
		f, motion, err := decodeFollower(record, id)
		if err != nil {
			return nil, err
		}
		snapshot.World.Followers[id], snapshot.Motion[id] = f, motion
	}
	for id := 0; id < engine.SceneryCapacity; id++ {
		record := r.record(0x6bd0, id, 14)
		if record[12] == 0 {
			continue
		}
		kind := engine.SceneryNone
		switch record[0] {
		case 22:
			kind = engine.SceneryTree
		case 24:
			kind = engine.SceneryBoulder
		case 30:
			kind = engine.SceneryBurningTree
		default:
			return nil, fmt.Errorf("unsupported GAM scenery kind%d", record[0])
		}
		snapshot.World.Nature.Scenery[id] = engine.SceneryActor{Kind: kind, X: record[6], Y: record[8], Age: int8(record[1])}
	}
	for id := 0; id < engine.EffectCapacity; id++ {
		record := r.record(0xc800, id, 32)
		if record[12] != 0 {
			if err := decodeEffect(record, id, &snapshot, catalog); err != nil {
				return nil, err
			}
		}
	}
	for id := 0; id < engine.WallCapacity; id++ {
		record := r.record(0x5f50, id, 16)
		if record[12] != 0 {
			wall, err := decodeWall(record, catalog)
			if err != nil {
				return nil, fmt.Errorf("GAM wall%d: %w", id, err)
			}
			snapshot.World.Earth.Walls[id] = wall
		}
	}
	if err := decodePlayers(r, &snapshot); err != nil {
		return nil, err
	}
	if err := decodeChains(r, &snapshot); err != nil {
		return nil, err
	}
	world, err := snapshot.Restore()
	if err != nil {
		return nil, fmt.Errorf("GAM typed world validation: %w", err)
	}
	name := string(data[0xeb30-fileStart : 0xeb40-fileStart])
	name = strings.SplitN(name, "\x00", 2)[0]
	profile := engine.NewDeity(name)
	god := 0xe76a + side*314
	profile.Bolts = r.word(god + 0x58)
	for i := range profile.FaceParts {
		profile.FaceParts[i] = r.byte(god + 0x4e + i)
	}
	profile.Experience = world.Players[side-1].Experience
	metadata := Metadata{Original: append([]byte(nil), data[:FileSize]...), Profile: profile, ProfileSide: side - 1, GameMode: mode, CameraX: int(int16(r.word(0x5f44))), CameraY: int(int16(r.word(0x5f46))), initial: world.Snapshot(), initialProfile: profile, Catalog: catalog}
	return &Document{World: world, Metadata: metadata}, nil
}
