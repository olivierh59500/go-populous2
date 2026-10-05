package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type groundNativeCall struct {
	Kind                  string
	Reference, X, Y, Mode uint16
	Raw                   string
}
type groundNativeFrame struct {
	Operation, Iteration int
	Hash                 string
	RNG                  uint32
	RandomDraws          int
	Calls                []groundNativeCall
	Changes              []nativeHeroChange
	Exit                 string
}
type groundNativeFixture struct {
	Input       groundNativeInput
	InitialHash string
	Frames      []groundNativeFrame
}

type groundNativeInput struct {
	whirlwindNativeInput
	SourceD2Upper uint16
}

func TestNativeGroundCastsAndRetainedConsumersAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/ground_native_full.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		SourceCodeSHA256 string
		FullImageEnd     int
		Entries          map[string]int
		Cases            []groundNativeFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 5430 || catalog.FullImageEnd != 0xeb90 || catalog.Entries["Font"] != 0x16938 || catalog.Entries["Swamp"] != 0x169cc || catalog.SourceCodeSHA256 != "86ad7f58200fc3295328d3a7ebad4e0ae6663be402d5ed090981b1e53b0a3841" {
		t.Fatal("native ground oracle provenance incomplete")
	}
	rules, err := DecodeNativeGroundRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	towns, err := DecodeNativeTownEvaluator(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	frames, casts, attempts, writes, conversion, swamps, restores, deaths, leaders := 0, 0, 0, 0, 0, 0, 0, 0, 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			c := fixture.Input
			m := whirlwindNativeFixtureMemory(c.whirlwindNativeInput)
			if m.err != nil {
				t.Fatal(m.err)
			}
			if whirlwindNativeHash(m) != fixture.InitialHash {
				t.Fatal("complete native ground initial BSS differs")
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := []groundNativeCall{}
			rng, draws := c.Seed, 0
			random := func() uint16 {
				draws++
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				_ = memory.Write32(0xeb28, rng)
				return uint16(rng >> 8 & 0x7fff)
			}
			link := func(ref NativeRecordReference) error {
				calls = append(calls, groundNativeCall{Kind: "link", Reference: uint16(ref)})
				return m.insert(ref)
			}
			unlink := func(ref NativeRecordReference) error {
				calls = append(calls, groundNativeCall{Kind: "unlink", Reference: uint16(ref)})
				return m.unlink(ref)
			}
			farms := func(ref NativeRecordReference, tile uint8) error {
				calls = append(calls, groundNativeCall{Kind: "farms", Reference: uint16(ref), Mode: uint16(tile)})
				return towns.ClearFarms(ref, tile, m.town(t))
			}
			read := func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m.cleanupMemory, ref) }
			write := func(ref NativeRecordReference, a FollowerEntryActor) error {
				return winEntryWrite(m.cleanupMemory, ref, a)
			}
			leader := func(ref NativeRecordReference) error {
				calls = append(calls, groundNativeCall{Kind: "leader", Reference: uint16(ref)})
				leaders++
				_, err := ClearFollowerLeader(ref, FollowerCleanupRegisters{}, FollowerLeaderCallbacks{Memory: memory, Unlink: unlink, Insert: link})
				return err
			}
			cleanup := func(ref NativeRecordReference, mode uint16) error {
				calls = append(calls, groundNativeCall{Kind: "cleanup", Reference: uint16(ref), Mode: mode})
				deaths++
				if m.byte(cleanupRecordAddress(ref)+13)&1 != 0 {
					calls = append(calls, groundNativeCall{Kind: "leader", Reference: uint16(ref)})
					leaders++
				}
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: unlink, Insert: link, ClearFarms: farms})
				return err
			}
			tile := func(packed NativePackedTile) (uint8, error) {
				return memory.Read8(nativeWhirlwindGrid(uint16(packed)) + 1)
			}
			scenario := func(owner uint8) (uint16, error) {
				at := 0xeb2e
				if owner == 1 {
					at = 0xeb2c
				}
				return memory.Read16(at)
			}
			sound := func(argument uint16) error {
				raw := whirlwindNativeBytes(m)
				calls = append(calls, groundNativeCall{Kind: "sound", Mode: argument, Raw: fmt.Sprintf("%x", raw[0x76f4:0x76f4+52])})
				return nil
			}
			move := func(ref NativeRecordReference, x, y uint16) error {
				calls = append(calls, groundNativeCall{Kind: "move", Reference: uint16(ref), X: x, Y: y})
				return neutralComposedMove(m.cleanupMemory, ref, x, y)
			}
			cb := NativeGroundCallbacks{Memory: memory, Random: random, SourceD2Upper: c.SourceD2Upper,
				Prepass: CommonPrepassCallbacks{Read: read, Write: write, Tile: tile, WriteTile: func(packed NativePackedTile, value uint8) error {
					restores++
					return memory.Write8(nativeWhirlwindGrid(uint16(packed))+1, value)
				}, Scenario: scenario, ClearFarms: farms, Cleanup: cleanup, ClearLeader: leader, Unlink: unlink, Sound: sound},
				Terrain: FollowerTerrainCallbacks{Read: read, Write: write, Tile: tile, Scenario: scenario, Attrition: func(owner uint8) (uint32, error) { return memory.Read32(heroGodAddress(owner) + 20) }, SetWaterReference: func(owner uint8, ref NativeRecordReference) error {
					return memory.Write16(heroGodAddress(owner)+0x36, uint16(ref))
				}, Cleanup: cleanup, ClearLeader: leader, Move: move},
				Aftermath: FollowerAftermathCallbacks{Read: read, Write: write, ClearLeader: leader, Unlink: unlink, Cleanup: cleanup,
					Head: func(packed NativePackedTile) (NativeRecordReference, error) {
						v, e := memory.Read16(nativeWhirlwindGrid(uint16(packed)) + 2)
						return NativeRecordReference(v), e
					}, Tile: tile, DestroyTown: func(NativeRecordReference) error { return fmt.Errorf("unexpected non-ground town destruction") }},
			}
			frameIndex := 0
			for opIndex, op := range c.Operations {
				whirlwindNativePatch(m, op.Initial)
				for iteration := range max(1, op.Repeat) {
					before := whirlwindNativeBytes(m)
					calls = []groundNativeCall{}
					ref := NativeRecordReference(op.Reference)
					switch op.Kind {
					case "font", "swamp":
						id := Baptism
						if op.Kind == "swamp" {
							id = Swamp
						}
						var step NativeGroundCreation
						step, err = rules.Create(id, c.Owner, c.X, c.Y, cb)
						casts++
						attempts += step.Attempts
						writes += step.Written
					case "prepass":
						_, err = rules.Prepass.Tick(ref, cb.Prepass)
					case "consumer":
						var step NativeGroundFollowerStep
						step, err = rules.TickFollower(ref, cb)
						if step.Prepass.Hazard == CommonPrepassConversion {
							conversion++
						}
						if step.Prepass.Hazard == CommonPrepassSwamp {
							swamps++
						}
					default:
						t.Fatalf("unknown native ground operation %s", op.Kind)
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
						t.Fatal("native trace order differs")
					}
					after := whirlwindNativeBytes(m)
					changes := []nativeHeroChange{}
					for at, value := range before {
						if value != after[at] {
							changes = append(changes, nativeHeroChange{Address: at, Value: after[at]})
						}
					}
					if !reflect.DeepEqual(changes, frame.Changes) {
						for i := 0; i < max(len(changes), len(frame.Changes)); i++ {
							if i >= len(changes) || i >= len(frame.Changes) || changes[i] != frame.Changes[i] {
								t.Fatalf("operation%d iteration%d complete BSS write%d differs: actual%v native%v", opIndex, iteration, i, changes, frame.Changes)
							}
						}
					}
					state, _ := memory.Read32(0xeb28)
					if whirlwindNativeHash(m) != frame.Hash || state != frame.RNG || draws != frame.RandomDraws {
						t.Fatalf("operation%d iteration%d full BSS/RNG differs", opIndex, iteration)
					}
					if !reflect.DeepEqual(calls, frame.Calls) {
						t.Fatalf("operation%d iteration%d native call sequence differs: actual%v native%v", opIndex, iteration, calls, frame.Calls)
					}
				}
			}
			if frameIndex != len(fixture.Frames) {
				t.Fatal("native trace has extra frames")
			}
		})
	}
	if frames < 20000 || casts < 500 || attempts < 5000 || writes < 1000 || conversion < 100 || swamps < 100 || restores < 50 || deaths < 100 || leaders < 50 {
		t.Fatalf("native ground coverage incomplete: frames%d casts%d attempts%d writes%d conversion%d swamp%d restored%d cleanup%d leader%d", frames, casts, attempts, writes, conversion, swamps, restores, deaths, leaders)
	}
	t.Logf("original CPU cases%d frames%d casts%d attempts%d writes%d conversion%d swamp%d restored%d cleanup%d leader%d", len(catalog.Cases), frames, casts, attempts, writes, conversion, swamps, restores, deaths, leaders)
}
