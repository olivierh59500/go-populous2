package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	legacy "go-populous2/internal/legacy"
)

type volcanoTerrainCall struct {
	X, Y  int
	Raise bool
}
type volcanoSnapshot struct {
	PoolHash, GridHash, HeightHash, SourceRaw string
	RNG                                       uint32
	Phase, Owner                              uint8
	Calls                                     []volcanoTerrainCall
}
type volcanoFixture struct {
	Input struct {
		Name, Terrain   string
		Owner, XP, X, Y uint8
		Occupied        int
		Seed            uint32
	}
	Accepted  bool
	Reference uint16
	Initial   volcanoSnapshot
	Trace     []volcanoSnapshot
}
type volcanoTestMemory struct {
	m         *cleanupMemory
	core      *legacy.World
	rng       uint32
	volcano   VolcanoRules
	lava      NativeLavaRules
	primitive NativePrimitiveCreatorRules
	basalt    BasaltRules
	calls     []volcanoTerrainCall
}

func newVolcanoTestMemory(t *testing.T, f volcanoFixture) *volcanoTestMemory {
	t.Helper()
	v := &volcanoTestMemory{m: &cleanupMemory{}, rng: f.Input.Seed, calls: []volcanoTerrainCall{}}
	var err error
	exe := testBundle(t).Executable
	v.volcano, err = DecodeVolcanoRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	v.lava, err = DecodeNativeLavaRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	v.primitive, err = DecodeNativePrimitiveCreatorRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	v.basalt, err = DecodeBasaltRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	var alt [legacy.EndWidth * legacy.EndWidth]int
	for y := range 65 {
		for x := range 65 {
			height := 2
			if f.Input.Terrain == "water" {
				height = 0
			}
			if f.Input.Terrain == "mound" {
				height += max(0, 4-max(abs(x-int(f.Input.X)), abs(y-int(f.Input.Y))))
			}
			alt[x+y*65] = height
		}
	}
	v.core = legacy.WorldFromSnapshot(legacy.WorldSnapshot{Alt: alt}, legacy.DefaultTerrainRules())
	for y := range 64 {
		for x := range 64 {
			height, shape := volcanoHeightShape(alt, x, y)
			at := 0xf44 + (x+y*64)*4
			v.m.putByte(at, 0xf8|height)
			v.m.putByte(at+1, shape)
		}
	}
	for i := range NativeEffectCapacity {
		at := 0xc800 + i*32
		for j := range 32 {
			v.m.putByte(at+j, uint8(0x31+j))
		}
		v.m.putByte(at+12, 0)
		if i < f.Input.Occupied {
			v.m.putByte(at+12, 2)
		}
	}
	for owner := range 3 {
		v.m.putByte(0xe76a+owner*314+86, f.Input.XP)
	}
	if v.m.err != nil {
		t.Fatal(v.m.err)
	}
	return v
}

func volcanoHeightShape(alt [legacy.EndWidth * legacy.EndWidth]int, x, y int) (uint8, uint8) {
	a := x + y*65
	heights := [4]int{alt[a], alt[a+1], alt[a+66], alt[a+65]}
	height := min(heights[0], heights[1], heights[2], heights[3])
	shape := uint8(0)
	for i, value := range heights {
		if value > height {
			shape |= 1 << i
		}
	}
	if shape == 0 && height > 0 {
		height--
		shape = 15
	}
	return uint8(height), shape
}

func (v *volcanoTestMemory) memory() FollowerCleanupMemory {
	m := v.m
	return FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
}
func (v *volcanoTestMemory) random() uint16 {
	if v.rng == 0 {
		v.rng = 0xbc614e
	}
	v.rng *= 0xbb40e62d
	return uint16(v.rng >> 8 & 0x7fff)
}
func (v *volcanoTestMemory) cell(p NativePackedTile) (NativeOccupancyCell, error) {
	if uint16(p)&0xc0c0 != 0 {
		return NativeOccupancyCell{}, fmt.Errorf("fixture cell outside native map")
	}
	at := 0xf44 + (int(uint8(p))+int(uint8(p>>8))*64)*4
	return NativeOccupancyCell{Header: v.m.byte(at), Tile: v.m.byte(at + 1), Head: NativeRecordReference(v.m.word(at + 2))}, v.m.err
}
func (v *volcanoTestMemory) writeTile(p NativePackedTile, tile uint8) error {
	return v.m.write8(0xf44+(int(uint8(p))+int(uint8(p>>8))*64)*4+1, tile)
}

func (v *volcanoTestMemory) terrain(x, y int, raise bool) error {
	v.calls = append(v.calls, volcanoTerrainCall{x, y, raise})
	before := v.core.Alt
	if raise {
		v.core.DirectRaiseTerrain(x, y)
	} else {
		v.core.DirectLowerTerrain(x, y)
	}
	for yy := range 64 {
		for xx := range 64 {
			a := xx + yy*65
			if before[a] == v.core.Alt[a] && before[a+1] == v.core.Alt[a+1] && before[a+66] == v.core.Alt[a+66] && before[a+65] == v.core.Alt[a+65] {
				continue
			}
			height, shape := volcanoHeightShape(v.core.Alt, xx, yy)
			at := 0xf44 + (xx+yy*64)*4
			old := v.m.byte(at + 1)
			if old&0xf0 == 0xe0 {
				shape |= 0xe0
			}
			v.m.putByte(at, height)
			v.m.putByte(at+1, shape)
		}
	}
	return v.m.err
}

func (v *volcanoTestMemory) createBasalt(owner uint8, p NativePackedTile, direction uint16) error {
	var pool [NativeEffectCapacity]NativeEffectActor
	var state BasaltState
	for i := range pool {
		at := 0xc800 + i*32
		pool[i] = NativeEffectActor{Active: v.m.byte(at+12) != 0, VX: int16(v.m.word(at + 14)), VY: int16(v.m.word(at + 16)), Speed: v.m.byte(at + 18)}
	}
	_, _ = v.basalt.Create(&pool, &state, int(owner)-1, int(uint8(p)), int(uint8(p>>8)), int16(v.basalt.BaseLife), direction, BasaltCallbacks{
		ReadGeometry: func(x, y int) uint8 { return v.basalt.Geometry[v.m.byte(0xf44+(x+y*64)*4+1)] },
		WriteTile:    func(x, y int, tile uint8) { v.m.putByte(0xf44+(x+y*64)*4+1, tile) },
		Random:       func() int { return int(v.random()) },
		Link: func(i int) {
			a, at := pool[i], 0xc800+i*32
			v.m.putByte(at, a.Kind)
			v.m.putByte(at+12, owner)
			v.m.putWord(at+6, uint16(a.X))
			v.m.putWord(at+8, uint16(a.Y))
			v.m.putWord(at+10, uint16(a.Animation))
			v.m.putWord(at+20, uint16(a.Timer))
			v.m.putByte(at+22, a.State)
			v.m.putWord(at+24, uint16(a.Life))
			v.m.putWord(at+26, state.Directions[i])
			if err := v.m.insert(NativeRecordReference(at - 0x76c0)); err != nil {
				v.m.err = err
			}
		},
	})
	return v.m.err
}

func (v *volcanoTestMemory) lavaCallbacks() NativeLavaCallbacks {
	return NativeLavaCallbacks{Memory: v.memory(), Cell: v.cell, Random: v.random, Link: v.m.insert, Unlink: v.m.unlink, CreateBasalt: v.createBasalt}
}
func (v *volcanoTestMemory) callbacks() VolcanoCallbacks {
	return VolcanoCallbacks{Memory: v.memory(), Cell: v.cell, WriteTile: v.writeTile, Random: v.random,
		FireExperience: func(owner uint8) (uint8, error) { return v.m.read8(0xe76a + int(owner)*314 + 86) },
		Lower:          func(x, y int) error { return v.terrain(x, y, false) }, Raise: func(x, y int) error { return v.terrain(x, y, true) },
		CreateFireColumn: func(owner uint16, x, y uint8) error {
			_, err := v.primitive.CreateFireColumn(owner, x, y, NativePrimitiveCreatorCallbacks{Memory: v.memory(), Random: v.random, Link: v.m.insert})
			return err
		},
		CreateLava: func(owner uint8, p NativePackedTile, d uint16) error {
			_, err := v.lava.Create(owner, p, d, v.lavaCallbacks())
			return err
		},
	}
}

func (v *volcanoTestMemory) verify(t *testing.T, ref NativeRecordReference, accepted bool, expected volcanoSnapshot) {
	t.Helper()
	bytes := v.m.records.Bytes[0xc800-NativeRecordImageStart:]
	if fmt.Sprintf("%x", sha256.Sum256(bytes)) != expected.PoolHash {
		t.Fatal("complete original volcano shared pool differs")
	}
	if fmt.Sprintf("%x", sha256.Sum256(v.m.lower[0xf44:0x4f44])) != expected.GridHash {
		t.Fatal("complete original volcano grid/header/head differs")
	}
	if nativeDirectHeightHash(v.core.Alt) != expected.HeightHash {
		t.Fatal("complete original volcano4225 heights differ")
	}
	if v.rng != expected.RNG {
		t.Fatalf("volcano RNG got%x want%x", v.rng, expected.RNG)
	}
	if !reflect.DeepEqual(v.calls, expected.Calls) {
		t.Fatalf("original terrain primitive sequence differs: got%d want%d", len(v.calls), len(expected.Calls))
	}
	if accepted {
		at := 0x76c0 + int(int16(ref)) - NativeRecordImageStart
		if hex.EncodeToString(v.m.records.Bytes[at:at+32]) != expected.SourceRaw {
			t.Fatal("native volcano retained32-byte source differs")
		}
	}
}

func TestVolcanoAgainstOriginal68000Sequences(t *testing.T) {
	data, err := os.ReadFile("testdata/volcano_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []volcanoFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 30 {
		t.Fatal("native volcano fixture catalog incomplete")
	}
	updates := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			v := newVolcanoTestMemory(t, f)
			ref, accepted, err := v.volcano.Create(f.Input.Owner, f.Input.X, f.Input.Y, v.callbacks())
			if err != nil {
				t.Fatal(err)
			}
			if accepted != f.Accepted || accepted && uint16(ref) != f.Reference {
				t.Fatal("original volcano allocation differs")
			}
			v.verify(t, ref, accepted, f.Initial)
			for _, snapshot := range f.Trace {
				updates++
				v.calls = []volcanoTerrainCall{}
				step, err := v.volcano.Tick(ref, v.callbacks())
				if err != nil {
					t.Fatal(err)
				}
				if step.Finished != (snapshot.Owner == 0) || step.Erupted != (snapshot.Phase == 0x32 && snapshot.Owner != 0) {
					t.Fatal("original volcano phase transition differs")
				}
				v.verify(t, ref, true, snapshot)
			}
		})
	}
	if updates != 236 {
		t.Fatal("native volcano trace empty")
	}
}
