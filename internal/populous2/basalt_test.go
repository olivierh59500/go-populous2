package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativeBasaltCell struct {
	Index        int
	Header, Tile uint8
	Head         uint16
}

type nativeBasaltInput struct {
	Name, Mode   string
	X, Y         int
	Owner        uint8
	Life         int16
	Direction    uint16
	Occupied     int
	Seed         uint32
	Tile, Header uint8
	Follower     bool
	Ticks        int
	Obstacles    []nativeBasaltCell
}

type nativeBasaltActor struct {
	Index int
	Raw   string
}

type nativeBasaltTick struct {
	Tick                             int
	Actors                           []nativeBasaltActor
	Cells                            []nativeBasaltCell
	RNG                              uint32
	PoolSHA256, GridSHA256, Follower string
}

type nativeBasaltCase struct {
	Input                                                 nativeBasaltInput
	Accepted                                              bool
	InitialActors                                         []nativeBasaltActor
	InitialCells                                          []nativeBasaltCell
	InitialRNG                                            uint32
	InitialPoolSHA256, InitialGridSHA256, InitialFollower string
	Trace                                                 []nativeBasaltTick
}

func nativeBasaltFixtures(t *testing.T) []nativeBasaltCase {
	t.Helper()
	data, err := os.ReadFile("testdata/basalt_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeBasaltCase }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 358 {
		t.Fatal("native basalt fixture catalog is incomplete")
	}
	return catalog.Cases
}

// This adapter represents the original record/cell bytes independently of
// the controller. Link and unlink mirror the verified native list operations,
// while fixtures compare the complete pool, grid, and preexisting follower.
type basaltTestMemory struct {
	memory nativeInteractionMemory
	pool   [NativeEffectCapacity]NativeEffectActor
	state  BasaltState
	rng    uint32
}

func newBasaltTestMemory(input nativeBasaltInput) *basaltTestMemory {
	m := &basaltTestMemory{rng: input.Seed}
	for i := range m.pool {
		m.pool[i] = NativeEffectActor{Active: i < input.Occupied, VX: -19, VY: 71, Speed: 25}
		m.state.Directions[i] = 0xeeee
		m.sync(i)
	}
	if inside(input.X, input.Y) {
		cell := interactionGrid + (input.X+input.Y*64)*4
		m.memory.bytes[cell], m.memory.bytes[cell+1] = input.Header, input.Tile
		if input.Follower {
			at := interactionGroupBase + 52
			m.memory.bytes[at], m.memory.bytes[at+12] = 2, 1
			m.memory.bytes[at+6], m.memory.bytes[at+8] = uint8(input.X), uint8(input.Y)
			m.memory.setLong(at+26, 100)
			m.memory.setWord(cell+2, 52)
		}
	}
	for _, cell := range input.Obstacles {
		at := interactionGrid + cell.Index*4
		m.memory.bytes[at], m.memory.bytes[at+1] = cell.Header, cell.Tile
		m.memory.setWord(at+2, cell.Head)
	}
	return m
}

func (m *basaltTestMemory) sync(index int) {
	a := m.pool[index]
	at := interactionEffect + index*32
	m.memory.bytes[at] = a.Kind
	m.memory.setWord(at+6, uint16(a.X))
	m.memory.setWord(at+8, uint16(a.Y))
	m.memory.setWord(at+10, uint16(a.Animation))
	m.memory.bytes[at+12] = 0
	if a.Active {
		m.memory.bytes[at+12] = a.Player + 1
	}
	m.memory.setWord(at+14, uint16(a.VX))
	m.memory.setWord(at+16, uint16(a.VY))
	m.memory.bytes[at+18] = a.Speed
	m.memory.setWord(at+20, uint16(a.Timer))
	m.memory.bytes[at+22] = a.State
	m.memory.setWord(at+24, uint16(a.Life))
	m.memory.setWord(at+26, m.state.Directions[index])
}

func (m *basaltTestMemory) callbacks(rules BasaltRules) BasaltCallbacks {
	return BasaltCallbacks{
		ReadGeometry: func(x, y int) uint8 { return rules.Geometry[m.memory.bytes[interactionGrid+(x+y*64)*4+1]] },
		WriteTile:    func(x, y int, tile uint8) { m.memory.bytes[interactionGrid+(x+y*64)*4+1] = tile },
		Random: func() int {
			if m.rng == 0 {
				m.rng = 0xbc614e
			}
			m.rng *= 0xbb40e62d
			return int(m.rng >> 8 & 0x7fff)
		},
		Link: func(index int) {
			m.sync(index)
			at := interactionEffect + index*32
			m.memory.setLong(at+2, 0)
			cell := interactionCell(m.memory.packed(at))
			head := m.memory.word(cell + 2)
			if head != 0 {
				m.memory.setWord(at+2, head)
				m.memory.setWord(interactionGroupBase+int(head)+4, uint16(at-interactionGroupBase))
			}
			m.memory.setWord(cell+2, uint16(at-interactionGroupBase))
		},
		Unlink: func(index int) { m.sync(index); m.memory.unlink(interactionEffect + index*32) },
	}
}

func (m *basaltTestMemory) verify(t *testing.T, actors []nativeBasaltActor, poolHash, gridHash, follower string, rng uint32) {
	t.Helper()
	for index := range m.pool {
		m.sync(index)
	}
	for _, golden := range actors {
		at := interactionEffect + golden.Index*32
		if got := hex.EncodeToString(m.memory.bytes[at : at+32]); got != golden.Raw {
			t.Fatalf("native actor%d differs\ngot %s\nwant%s", golden.Index, got, golden.Raw)
		}
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(m.memory.bytes[interactionEffect:interactionEffect+8000])); got != poolHash {
		t.Fatalf("complete native pool hash differs: got%s want%s", got, poolHash)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(m.memory.bytes[interactionGrid:interactionGrid+16384])); got != gridHash {
		t.Fatalf("complete native grid hash differs: got%s want%s", got, gridHash)
	}
	if got := hex.EncodeToString(m.memory.bytes[interactionGroupBase+52 : interactionGroupBase+104]); got != follower {
		t.Fatalf("preexisting follower record differs: got%s want%s", got, follower)
	}
	if m.rng != rng {
		t.Fatalf("native RNG differs: got%x want%x", m.rng, rng)
	}
}

func TestBasaltCreatorMatchesAllNativeGeometryCodesAndSharedSlots(t *testing.T) {
	rules, err := DecodeBasaltRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range nativeBasaltFixtures(t) {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			m := newBasaltTestMemory(input)
			beforePool, beforeState, beforeMemory, beforeRNG := m.pool, m.state, m.memory.bytes, m.rng
			index, created := rules.Create(&m.pool, &m.state, int(input.Owner)-1, input.X, input.Y, input.Life, input.Direction, m.callbacks(rules))
			if created != fixture.Accepted {
				t.Fatal("native basalt admission differs")
			}
			if created && index != input.Occupied {
				t.Fatal("creation did not use the first free shared slot")
			}
			if !created && (index != -1 || m.pool != beforePool || m.state != beforeState || m.memory.bytes != beforeMemory || m.rng != beforeRNG) {
				t.Fatal("rejected creation changed a native pool, graph, terrain, or RNG field")
			}
			m.verify(t, fixture.InitialActors, fixture.InitialPoolSHA256, fixture.InitialGridSHA256, fixture.InitialFollower, fixture.InitialRNG)
		})
	}
}

// The original $1482e runs all shared slots in ascending order. These fixtures
// therefore catch both a later-slot child's immediate first tick and a reused
// earlier slot waiting until the next pass. No fixed-length line is predicted.
func TestBasaltPropagationMatchesNativeOrderedPoolPasses(t *testing.T) {
	rules, err := DecodeBasaltRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	cases, passes, checkpoints := 0, 0, 0
	for _, fixture := range nativeBasaltFixtures(t) {
		if len(fixture.Trace) == 0 {
			continue
		}
		cases++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			m := newBasaltTestMemory(input)
			callbacks := m.callbacks(rules)
			if _, created := rules.Create(&m.pool, &m.state, int(input.Owner)-1, input.X, input.Y, input.Life, input.Direction, callbacks); !created {
				t.Fatal("native initial creator rejected")
			}
			next := 0
			for tick := 1; tick <= fixture.Trace[len(fixture.Trace)-1].Tick; tick++ {
				passes++
				for index := range m.pool {
					if !m.pool[index].Active || m.pool[index].Kind != BasaltActorKind {
						continue
					}
					step, err := rules.Tick(&m.pool, &m.state, index, callbacks)
					if err != nil {
						t.Fatal(err)
					}
					if step.ChildCreated && step.ChildIndex == index {
						t.Fatal("child reused its still-active parent slot")
					}
					if step.Finished && m.pool[index].Active {
						t.Fatal("finished parent remained active")
					}
				}
				if fixture.Trace[next].Tick == tick {
					golden := fixture.Trace[next]
					checkpoints++
					m.verify(t, golden.Actors, golden.PoolSHA256, golden.GridSHA256, golden.Follower, golden.RNG)
					next++
				}
			}
		})
	}
	if cases != 40 || passes != 7228 || checkpoints != 1149 {
		t.Fatalf("native coverage cases/passes/checkpoints=%d/%d/%d", cases, passes, checkpoints)
	}
}

func TestBasaltNativeDirectionAndAnimationBank(t *testing.T) {
	b := testBundle(t)
	rules, err := DecodeBasaltRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if rules.BaseLife != 100 || rules.DelayModulus != 18 || rules.DelayBias != 4 || rules.Directions != [4]uint16{0xff00, 1, 0x100, 0xffff} || rules.SequenceLength != 4 || rules.LoopOffset != -16 {
		t.Fatal("native basalt controller tables differ")
	}
	for _, frame := range rules.Frames {
		if frame.SoundCue != 33 {
			t.Fatal("native basalt animation lost cue33")
		}
		for _, bank := range b.Sprites {
			for _, layer := range frame.Layers {
				if layer.Sprite < 0 || layer.Sprite >= len(bank) {
					t.Fatal("native basalt composite sprite missing in landscape")
				}
			}
		}
	}
	for _, direction := range []uint16{1, 3, 5, 7, 8, 0xffff} {
		m := newBasaltTestMemory(nativeBasaltInput{Life: 100})
		before := *m
		if _, ok := rules.Create(&m.pool, &m.state, 0, 32, 32, 100, direction, m.callbacks(rules)); ok || *m != before {
			t.Fatal("a direction outside the native four-cardinal encoding changed the cast")
		}
	}
	if binary.BigEndian.Uint16(b.Executable.Hunks[0].Data[0x172a2:]) != 100 {
		t.Fatal("native usercast lifetime constant changed")
	}
}
