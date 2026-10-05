package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type whirlwindNativeOperation struct {
	Kind              string
	Reference, Target uint16
	Repeat            int
	Initial           []nativeHeroPatch
}
type whirlwindNativeInput struct {
	Name         string
	Owner        uint16
	X, Y         uint8
	Seed         uint32
	Header, Tile uint8
	BusyFX       int
	Initial      []nativeHeroPatch
	Links        []uint16
	Operations   []whirlwindNativeOperation
}
type whirlwindNativeCall struct {
	Kind                  string
	Reference, X, Y, Mode uint16
}
type whirlwindNativeFrame struct {
	Operation, Iteration int
	Hash                 string
	RNG                  uint32
	RandomDraws          int
	Calls                []whirlwindNativeCall
	Changes              []nativeHeroChange
}
type whirlwindNativeFixture struct {
	Input       whirlwindNativeInput
	InitialHash string
	Frames      []whirlwindNativeFrame
}

// The command window follows the three retained deities. Neutral owner3
// cleanup aliases this window, and the native RNG long lives at $eb28.
type whirlwindNativeTestMemory struct {
	*cleanupMemory
	command [0x78]byte
}

func (m *whirlwindNativeTestMemory) read8(at int) (uint8, error) {
	if at >= 0xeb18 && at < 0xeb90 {
		return m.command[at-0xeb18], nil
	}
	return m.cleanupMemory.read8(at)
}
func (m *whirlwindNativeTestMemory) read16(at int) (uint16, error) {
	if at >= 0xeb18 && at <= 0xeb8e {
		return binary.BigEndian.Uint16(m.command[at-0xeb18:]), nil
	}
	return m.cleanupMemory.read16(at)
}
func (m *whirlwindNativeTestMemory) read32(at int) (uint32, error) {
	if at >= 0xeb18 && at <= 0xeb8c {
		return binary.BigEndian.Uint32(m.command[at-0xeb18:]), nil
	}
	return m.cleanupMemory.read32(at)
}
func (m *whirlwindNativeTestMemory) write8(at int, value uint8) error {
	if at >= 0xeb18 && at < 0xeb90 {
		m.command[at-0xeb18] = value
		return nil
	}
	return m.cleanupMemory.write8(at, value)
}
func (m *whirlwindNativeTestMemory) write16(at int, value uint16) error {
	if at >= 0xeb18 && at <= 0xeb8e {
		binary.BigEndian.PutUint16(m.command[at-0xeb18:], value)
		return nil
	}
	return m.cleanupMemory.write16(at, value)
}
func (m *whirlwindNativeTestMemory) write32(at int, value uint32) error {
	if at >= 0xeb18 && at <= 0xeb8c {
		binary.BigEndian.PutUint32(m.command[at-0xeb18:], value)
		return nil
	}
	return m.cleanupMemory.write32(at, value)
}
func (m *whirlwindNativeTestMemory) putByte(at int, value uint8) {
	if err := m.write8(at, value); err != nil {
		m.err = err
	}
}
func (m *whirlwindNativeTestMemory) putWord(at int, value uint16) {
	if err := m.write16(at, value); err != nil {
		m.err = err
	}
}

func whirlwindNativePatch(m *whirlwindNativeTestMemory, patches []nativeHeroPatch) {
	for _, p := range patches {
		switch p.Width {
		case 1:
			m.putByte(p.Address, uint8(p.Value))
		case 2:
			m.putWord(p.Address, uint16(p.Value))
		case 4:
			if err := m.write32(p.Address, p.Value); err != nil {
				m.err = err
			}
		}
	}
}

func whirlwindNativeFixtureMemory(c whirlwindNativeInput) *whirlwindNativeTestMemory {
	m := &whirlwindNativeTestMemory{cleanupMemory: &cleanupMemory{}}
	_ = m.write32(0xeb28, c.Seed)
	for i := range 4096 {
		m.putByte(0xf44+i*4, c.Header)
		m.putByte(0xf45+i*4, c.Tile)
		m.putByte(0x4f44+i, 0x77)
	}
	for i := range 250 {
		a := 0xc800 + i*32
		for j := range 32 {
			m.putByte(a+j, uint8(i*32+j+7))
		}
		owner := uint8(0)
		if i < c.BusyFX {
			owner = 1
		}
		m.putByte(a+12, owner)
	}
	for owner := range 3 {
		god, marker := 0xe76a+owner*314, 0xe740+owner*14
		m.putWord(god+10, uint16(marker-0x76c0))
		m.putWord(god+0x44, 120)
		m.putWord(god+0x46, 3)
		m.putByte(marker, 20)
		m.putByte(marker+12, uint8(owner))
		m.putWord(marker+6, uint16(10+owner)<<8|128)
		m.putWord(marker+8, 0x0a80)
	}
	whirlwindNativePatch(m, c.Initial)
	for _, ref := range append([]uint16{0x7080, 0x708e, 0x709c}, c.Links...) {
		if err := m.insert(NativeRecordReference(ref)); err != nil {
			m.err = err
		}
	}
	return m
}

func whirlwindNativeBytes(m *whirlwindNativeTestMemory) []byte {
	image := append([]byte(nil), m.lower[:]...)
	image = append(image, m.records.Bytes[:]...)
	image = append(image, m.globals.Bytes[:]...)
	return append(image, m.command[:]...)
}

func whirlwindNativeHash(m *whirlwindNativeTestMemory) string {
	return fmt.Sprintf("%x", sha256.Sum256(whirlwindNativeBytes(m)))
}

// Every frame comes from the original relocated CPU bodies, with real $15cc8,
// $135ca, $124a2 and graph primitives. Complete BSS hashes include all 250 effect
// slots, 400 followers, scenery/walls, three magnets/deities, map and overlays.
func TestNativeWhirlwindFullLifecycleAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/whirlwind_native_full.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		SourceCodeSHA256 string
		FullImageEnd     int
		Entries          map[string]int
		Cases            []whirlwindNativeFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 789 {
		t.Fatalf("incomplete native whirlwind catalog: %d", len(catalog.Cases))
	}
	if catalog.SourceCodeSHA256 != "86ad7f58200fc3295328d3a7ebad4e0ae6663be402d5ed090981b1e53b0a3841" || catalog.FullImageEnd != 0xeb90 || catalog.Entries["Release"] != 0x14c20 || catalog.Entries["Water"] != 0x15cc8 {
		t.Fatal("native whirlwind oracle provenance differs")
	}
	b := testBundle(t)
	rules, err := DecodeNativeWhirlwindRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	primitives, err := DecodeNativePrimitiveCreatorRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	frames, births, lifts, deaths, towns, transports, landings := 0, 0, 0, 0, 0, 0, 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			c := fixture.Input
			m := whirlwindNativeFixtureMemory(c)
			if m.err != nil {
				t.Fatal(m.err)
			}
			if whirlwindNativeHash(m) != fixture.InitialHash {
				t.Fatal("native initial complete BSS image differs")
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			rng, draws := c.Seed, 0
			random := func() uint16 {
				draws++
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				_ = m.write32(0xeb28, rng)
				return uint16(rng >> 8 & 0x7fff)
			}
			calls := []whirlwindNativeCall{}
			link := func(ref NativeRecordReference) error {
				calls = append(calls, whirlwindNativeCall{Kind: "link", Reference: uint16(ref)})
				return m.insert(ref)
			}
			unlink := func(ref NativeRecordReference) error {
				calls = append(calls, whirlwindNativeCall{Kind: "unlink", Reference: uint16(ref)})
				return m.unlink(ref)
			}
			farms := func(ref NativeRecordReference, tile uint8) error {
				calls = append(calls, whirlwindNativeCall{Kind: "farms", Reference: uint16(ref), Mode: uint16(tile)})
				return town.ClearFarms(ref, tile, m.town(t))
			}
			cb := NativeWhirlwindCallbacks{Memory: memory, Random: random, SourceD2: c.Owner, Unlink: unlink, ClearFarms: farms}
			cb.Move = func(ref NativeRecordReference, x, y uint16) (bool, error) {
				calls = append(calls, whirlwindNativeCall{Kind: "move", Reference: uint16(ref), X: x, Y: y})
				at := cleanupRecordAddress(ref)
				changed := nativeOccupancyPackedTile(m.word(at+6), m.word(at+8)) != nativeOccupancyPackedTile(x, y)
				return changed, neutralComposedMove(m.cleanupMemory, ref, x, y)
			}
			cb.Cleanup = func(ref NativeRecordReference, registers FollowerCleanupRegisters) (FollowerCleanupStep, error) {
				calls = append(calls, whirlwindNativeCall{Kind: "cleanup", Reference: uint16(ref), Mode: uint16(uint8(registers.D0))})
				return CleanupFollower(ref, registers, FollowerCleanupCallbacks{Memory: memory, Unlink: unlink, Insert: link, ClearFarms: farms})
			}
			frameIndex := 0
			for opIndex, op := range c.Operations {
				whirlwindNativePatch(m, op.Initial)
				repeat := max(1, op.Repeat)
				for iteration := range repeat {
					before := whirlwindNativeBytes(m)
					calls = []whirlwindNativeCall{}
					ref := NativeRecordReference(op.Reference)
					switch op.Kind {
					case "create":
						_, err = primitives.CreateWhirlwind(c.Owner, c.X, c.Y, NativePrimitiveCreatorCallbacks{Memory: memory, Random: random, Link: link})
					case "water":
						var step NativePrimitiveCreation
						step, err = rules.CreateWaterChild(c.Owner, c.X, c.Y, memory)
						if step.Created {
							births++
						}
					case "lift":
						var lifted bool
						lifted, err = rules.Lift(ref, NativeRecordReference(op.Target), memory)
						if lifted {
							lifts++
						}
					case "release":
						var step WhirlwindReleaseResult
						step, err = rules.Release(ref, cb)
						deaths += step.Removed
					case "tick":
						var step NativeWhirlwindStep
						step, err = rules.Tick(ref, cb)
						lifts += step.Lifted
						deaths += step.Release.Removed
						towns += step.Towns
						if step.WaterCreated {
							births++
						}
					case "follower":
						var step NativeWhirlwindFollowerStep
						step, err = rules.TickFollower(ref, cb)
						if step.Moving {
							transports++
						}
						if step.ReadyNextUpdate {
							landings++
						}
					default:
						t.Fatalf("unknown fixture operation %s", op.Kind)
					}
					if err != nil {
						t.Fatalf("operation%d iteration%d: %v", opIndex, iteration, err)
					}
					if frameIndex >= len(fixture.Frames) {
						t.Fatal("native trace ended early")
					}
					frame := fixture.Frames[frameIndex]
					frameIndex++
					frames++
					if frame.Operation != opIndex || frame.Iteration != iteration {
						t.Fatal("native trace ordering differs")
					}
					after := whirlwindNativeBytes(m)
					changes := []nativeHeroChange{}
					for address, v := range before {
						if v != after[address] {
							changes = append(changes, nativeHeroChange{Address: address, Value: after[address]})
						}
					}
					if !reflect.DeepEqual(changes, frame.Changes) {
						for i := 0; i < max(len(changes), len(frame.Changes)); i++ {
							if i >= len(changes) || i >= len(frame.Changes) || changes[i] != frame.Changes[i] {
								t.Fatalf("operation%d iteration%d complete BSS write%d differs: actual%v native%v", opIndex, iteration, i, changes, frame.Changes)
							}
						}
					}
					if whirlwindNativeHash(m) != frame.Hash || rng != frame.RNG || draws != frame.RandomDraws {
						t.Fatalf("operation%d iteration%d full BSS/RNG differs: RNG%08x/%08x draws%d/%d", opIndex, iteration, rng, frame.RNG, draws, frame.RandomDraws)
					}
					if !reflect.DeepEqual(calls, frame.Calls) {
						t.Fatalf("operation%d iteration%d primitive calls differ: actual%v native%v", opIndex, iteration, calls, frame.Calls)
					}
				}
			}
			if frameIndex != len(fixture.Frames) {
				t.Fatal("native trace has extra frames")
			}
		})
	}
	if frames < 15000 || births < 36 || lifts < 300 || deaths < 100 || towns < 50 || transports < 500 || landings < 500 {
		t.Fatal(fmt.Sprintf("native lifecycle coverage incomplete: frames%d births%d lifts%d deaths%d towns%d transports%d landings%d", frames, births, lifts, deaths, towns, transports, landings))
	}
	t.Logf("original CPU cases%d frames%d births%d lifts%d deaths%d towns%d transports%d landings%d", len(catalog.Cases), frames, births, lifts, deaths, towns, transports, landings)
}

// Failed raw aliases remain failures. In particular a bad transport source
// cannot fabricate coordinates 0,0 or insert the follower into that parcel.
func TestNativeWhirlwindBoundsPreserveNativeWriteOrder(t *testing.T) {
	bundle := testBundle(t)
	rules, err := DecodeNativeWhirlwindRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	m := whirlwindNativeFixtureMemory(whirlwindNativeInput{Header: 0xa8, Tile: 15, Initial: []nativeHeroPatch{
		{Address: 0x76f4, Width: 1, Value: 8}, {Address: 0x7700, Width: 1, Value: 1},
		{Address: 0x76fa, Width: 2, Value: 0x2080}, {Address: 0x76fc, Width: 2, Value: 0x2080},
		{Address: 0x76fe, Width: 2, Value: 0x4d4}, {Address: 0x770a, Width: 1, Value: 0x14},
		{Address: 0x7714, Width: 2, Value: 0x8000}, {Address: 0x770e, Width: 4, Value: 123},
	}, Links: []uint16{52}})
	memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
	moves := 0
	cb := NativeWhirlwindCallbacks{Memory: memory, Move: func(NativeRecordReference, uint16, uint16) (bool, error) { moves++; return false, nil }}
	if _, err := rules.TickFollower(52, cb); err == nil {
		t.Fatal("out-of-window raw transport source was fabricated")
	}
	if moves != 0 || m.word(0x76fa) != 0x2080 || m.word(0x76fc) != 0x2080 || m.word(0x76fe) != 0x4d8 || m.byte(0x7700) != 1 || m.word(0x770e) != 0 || m.word(0x7710) != 123 {
		t.Fatal("failed raw source changed movement/population or lost the preceding animation write")
	}
	before := whirlwindNativeBytes(m)
	if _, err := rules.Release(0x5140, NativeWhirlwindCallbacks{Memory: memory}); err == nil {
		t.Fatal("incomplete raw release callbacks admitted")
	}
	if !reflect.DeepEqual(before, whirlwindNativeBytes(m)) {
		t.Fatal("incomplete release mutated retained memory")
	}
}
