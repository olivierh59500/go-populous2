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

type scenarioScriptInput struct {
	Name, Mode              string
	Parameters              [60]uint8
	Offset, Clock, GameMode uint16
	Execute                 bool
	Seed                    uint32
	Updates                 int
}
type scenarioScriptSnapshot struct {
	Hash     string
	Offset   uint16
	RNG      uint32
	Commands [][4]uint8
	Changes  []nativeHeroChange
}
type scenarioScriptMemory [65536]byte

func (m *scenarioScriptMemory) read8(a int) (uint8, error) {
	if a < 0 || a >= len(m) {
		return 0, fmt.Errorf("scenario BSS byte outside image")
	}
	return m[a], nil
}
func (m *scenarioScriptMemory) read16(a int) (uint16, error) {
	if a < 0 || a+2 > len(m) {
		return 0, fmt.Errorf("scenario BSS word outside image")
	}
	return binary.BigEndian.Uint16(m[a : a+2]), nil
}
func (m *scenarioScriptMemory) read32(a int) (uint32, error) {
	if a < 0 || a+4 > len(m) {
		return 0, fmt.Errorf("scenario BSS long outside image")
	}
	return binary.BigEndian.Uint32(m[a : a+4]), nil
}
func (m *scenarioScriptMemory) write8(a int, v uint8) error {
	if a < 0 || a >= len(m) {
		return fmt.Errorf("scenario BSS byte outside image")
	}
	m[a] = v
	return nil
}
func (m *scenarioScriptMemory) write16(a int, v uint16) error {
	if a < 0 || a+2 > len(m) {
		return fmt.Errorf("scenario BSS word outside image")
	}
	binary.BigEndian.PutUint16(m[a:a+2], v)
	return nil
}
func (m *scenarioScriptMemory) write32(a int, v uint32) error {
	if a < 0 || a+4 > len(m) {
		return fmt.Errorf("scenario BSS long outside image")
	}
	binary.BigEndian.PutUint32(m[a:a+4], v)
	return nil
}
func (m *scenarioScriptMemory) callbacks() FollowerCleanupMemory {
	return FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
}

func (m *scenarioScriptMemory) link(ref NativeRecordReference) error {
	a := cleanupRecordAddress(ref)
	if err := m.write32(a+2, 0); err != nil {
		return err
	}
	x, err := m.read8(a + 6)
	if err != nil {
		return err
	}
	y, err := m.read8(a + 8)
	if err != nil {
		return err
	}
	cell := 0xf44 + int(int16(uint16(y)<<8|uint16(uint8(x<<2))))
	head, err := m.read16(cell + 2)
	if err != nil {
		return err
	}
	if head != 0 {
		if err := m.write16(a+2, head); err != nil {
			return err
		}
		if err := m.write16(cleanupRecordAddress(NativeRecordReference(head))+4, uint16(ref)); err != nil {
			return err
		}
	}
	return m.write16(cell+2, uint16(ref))
}

func TestScenarioScriptAgainstOriginalSchedulerAndActualCommands(t *testing.T) {
	data, err := os.ReadFile("testdata/scenario_script_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input scenarioScriptInput
			Trace []scenarioScriptSnapshot
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 272 {
		t.Fatal("native scenario script catalog incomplete")
	}
	exe := testBundle(t).Executable
	primitive, err := DecodeNativePrimitiveCreatorRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	fireRain, err := DecodeFireRainRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	storm, err := DecodeStormRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	volcano, err := DecodeVolcanoRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	updates, actual := 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := &scenarioScriptMemory{}
			for i := range 4096 {
				m[0xf44+i*4] = 0xa8
				m[0xf45+i*4] = 15
			}
			copy(m[0xdde:], f.Input.Parameters[:])
			_ = m.write16(0xf0a, f.Input.Offset)
			_ = m.write16(0xf42, f.Input.Clock)
			_ = m.write16(0xeb44, f.Input.GameMode)
			_ = m.write32(0xdc4, 0xaabbccdd)
			_ = m.write32(0xeb28, f.Input.Seed)
			memory := m.callbacks()
			random := func() uint16 {
				rng, _ := m.read32(0xeb28)
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				_ = m.write32(0xeb28, rng)
				return uint16(rng >> 8 & 0x7fff)
			}
			for _, want := range f.Trace {
				updates++
				before := *m
				commands := [][4]uint8{}
				if f.Input.Mode == "load" {
					if err := LoadScenarioScript(f.Input.Parameters, memory); err != nil {
						t.Fatal(err)
					}
				} else {
					_, err := TickScenarioScript(ScenarioScriptCallbacks{Memory: memory, Execute: func(a int) error {
						command := [4]uint8{m[a], m[a+1], m[a+2], m[a+3]}
						commands = append(commands, command)
						if !f.Input.Execute {
							return nil
						}
						actual++
						owner, x, y := uint16(command[0]), command[2], command[3]
						switch command[1] {
						case 0:
							return nil
						case 6:
							_, err := primitive.CreateFireColumn(owner, x, y, NativePrimitiveCreatorCallbacks{Memory: memory, Random: random, Link: m.link})
							return err
						case 22:
							_, err := primitive.CreateWhirlwind(owner, x, y, NativePrimitiveCreatorCallbacks{Memory: memory, Random: random, Link: m.link})
							return err
						case 38:
							_, err := fireRain.Create(owner, x, y, FireRainCallbacks{Memory: memory, Random: random})
							return err
						case 62:
							_, _, err := volcano.Create(uint8(owner), x, y, VolcanoCallbacks{Memory: memory, FireExperience: func(o uint8) (uint8, error) { return m.read8(primitiveDeityAddress(uint16(o)) + 0x56) }})
							return err
						case 64:
							_, err := storm.Create(owner, x, y, a, StormCallbacks{Memory: memory, Random: random, Link: m.link})
							return err
						default:
							return fmt.Errorf("unproven scenario command in composed fixture")
						}
					}})
					if err != nil {
						t.Fatal(err)
					}
				}
				cursor, _ := m.read16(0xf0a)
				rng, _ := m.read32(0xeb28)
				if cursor != want.Offset || rng != want.RNG || fmt.Sprintf("%x", sha256.Sum256(m[:])) != want.Hash {
					t.Fatalf("native scenario scheduler image/cursor/RNG differs: offset%d want%d", cursor, want.Offset)
				}
				if !reflect.DeepEqual(commands, want.Commands) {
					t.Fatalf("native scratch command boundary differs: got%v want%v", commands, want.Commands)
				}
				changes := []nativeHeroChange{}
				for i, v := range before {
					if v != m[i] {
						changes = append(changes, nativeHeroChange{Address: i, Value: m[i]})
					}
				}
				if !reflect.DeepEqual(changes, want.Changes) {
					t.Fatal("native scenario changed byte ranges differ")
				}
			}
		})
	}
	if updates != 278 || actual != 36 {
		t.Fatalf("native scenario trace coverage differs: %d updates/%d actual commands", updates, actual)
	}
}

func TestScenarioScriptDecodedRecordsAndOddCursorFault(t *testing.T) {
	var raw [60]byte
	copy(raw[:], []byte{0x12, 0x34, 0xaa, 38, 62, 63})
	events := DecodeScenarioScript(raw)
	if events[0] != (ScenarioScriptEvent{Time: 0x1234, Reserved: 0xaa, Command: 38, X: 62, Y: 63}) || events[1].Time != 0 {
		t.Fatal("native six-byte world event layout differs")
	}
	m := &scenarioScriptMemory{}
	_ = m.write16(0xf0a, 0xffff)
	if _, err := TickScenarioScript(ScenarioScriptCallbacks{Memory: m.callbacks()}); err == nil {
		t.Fatal("native odd-word address fault was normalized away")
	}
}
