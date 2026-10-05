package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// The raw memory format is shared with the committed Whirlwind harness. It
// retains the command window as well as all actor pools, maps and deities.
type fireColumnNativeFixture struct {
	Input       fireColumnNativeInput
	InitialHash string
	Frames      []whirlwindNativeFrame
}

type fireColumnNativeInput struct {
	whirlwindNativeInput
	Pattern int
}

func fireColumnNativeFixtureMemory(c fireColumnNativeInput) *whirlwindNativeTestMemory {
	input := c.whirlwindNativeInput
	if c.Pattern != 0 {
		patches := make([]nativeHeroPatch, 0, 8192+len(input.Initial))
		for i := range 4096 {
			patches = append(patches, nativeHeroPatch{Address: 0xf44 + i*4, Width: 1, Value: uint32(0xe8 + (i*13+i/64)%8)})
			if c.Pattern == 2 {
				tile := (i * 7) % 16
				if tile == 0 {
					tile = 15
				}
				patches = append(patches, nativeHeroPatch{Address: 0xf45 + i*4, Width: 1, Value: uint32(tile)})
			}
		}
		input.Initial = append(patches, input.Initial...)
	}
	return whirlwindNativeFixtureMemory(input)
}

func TestNativeFireColumnFullControllerAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/fire_column_native_full.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		SourceCodeSHA256 string
		FullImageEnd     int
		Entries          map[string]int
		Cases            []fireColumnNativeFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1169 || catalog.FullImageEnd != 0xeb90 || catalog.Entries["Active"] != 0x148bc || catalog.Entries["Damage"] != 0x16542 || catalog.SourceCodeSHA256 != "86ad7f58200fc3295328d3a7ebad4e0ae6663be402d5ed090981b1e53b0a3841" {
		t.Fatal("native fire column oracle catalog/provenance incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeNativeFireColumnRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	primitives, err := DecodeNativePrimitiveCreatorRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	towns, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := DecodeTownCombatRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	forest, err := DecodeForestNativeRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	frames, moves, routes, water, expired, removed, scans, hits, spread, destroyed := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			c := fixture.Input
			m := fireColumnNativeFixtureMemory(c)
			if m.err != nil {
				t.Fatal(m.err)
			}
			if whirlwindNativeHash(m) != fixture.InitialHash {
				t.Fatal("complete native initial BSS differs")
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
				return towns.ClearFarms(ref, tile, m.town(t))
			}
			cleanup := func(ref NativeRecordReference, mode uint16) error {
				calls = append(calls, whirlwindNativeCall{Kind: "cleanup", Reference: uint16(ref), Mode: mode})
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: unlink, Insert: link, ClearFarms: farms})
				return err
			}
			destroy := func(ref NativeRecordReference) error {
				calls = append(calls, whirlwindNativeCall{Kind: "destroy", Reference: uint16(ref)})
				destroyed++
				_, err := combat.Destroy(ref, TownCombatCallbacks{
					Read: func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m.cleanupMemory, ref) },
					Write: func(ref NativeRecordReference, a FollowerEntryActor) error {
						return winEntryWrite(m.cleanupMemory, ref, a)
					},
					ClearFarms: farms, Cleanup: cleanup,
				})
				return err
			}
			cb := NativeFireColumnCallbacks{Memory: memory, Random: random, Unlink: unlink, DestroyTown: destroy, Move: func(ref NativeRecordReference, x, y uint16) (bool, error) {
				calls = append(calls, whirlwindNativeCall{Kind: "move", Reference: uint16(ref), X: x, Y: y})
				at := cleanupRecordAddress(ref)
				changed := nativeOccupancyPackedTile(m.word(at+6), m.word(at+8)) != nativeOccupancyPackedTile(x, y)
				return changed, neutralComposedMove(m.cleanupMemory, ref, x, y)
			}}
			frameIndex := 0
			for opIndex, op := range c.Operations {
				whirlwindNativePatch(m, op.Initial)
				for iteration := range max(1, op.Repeat) {
					before := whirlwindNativeBytes(m)
					calls = []whirlwindNativeCall{}
					ref := NativeRecordReference(op.Reference)
					switch op.Kind {
					case "create":
						_, err = primitives.CreateFireColumn(c.Owner, c.X, c.Y, NativePrimitiveCreatorCallbacks{Memory: memory, Random: random, Link: link})
					case "scorch":
						err = rules.Storm.Scorch(ref, memory)
					case "damage":
						_, err = rules.Storm.Damage(ref, StormCallbacks{Memory: memory, DestroyTown: destroy})
					case "town":
						err = destroy(ref)
					case "burnneighbors":
						err = forest.BurnNeighbors(ref, ForestNativeCallbacks{Memory: memory, DestroyTown: destroy})
					case "tree":
						var step ForestNativeStep
						step, err = forest.Tick(ref, m.word(0xf42), ForestNativeCallbacks{Memory: memory, Random: random, Link: link, Unlink: unlink, DestroyTown: destroy})
						if step.SpreadFire {
							spread++
						}
					case "tick":
						var step NativeFireColumnStep
						step, err = rules.Tick(ref, cb)
						if step.Moved {
							moves++
						}
						if step.Routed {
							routes++
						}
						if step.Water {
							water++
						}
						if step.Expired {
							expired++
						}
						if step.Removed {
							removed++
						}
						scans += step.DamageScans
						hits += int(step.Hits)
						// Complete original calls below also prove there is no
						// premature walker/hero cleanup or neighbor radius burn.
					case "faulttick":
						_, err = rules.Tick(ref, cb)
						if err == nil {
							t.Fatal("native zero-speed DIVU fault was fabricated as movement")
						}
						err = nil
					default:
						t.Fatalf("unknown native fire column operation %s", op.Kind)
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
	if frames < 10000 || moves < 1000 || routes < 100 || water < 20 || expired < 100 || removed < 100 || scans < 1000 || hits < 200 || spread < 6 || destroyed < 100 {
		t.Fatal(fmt.Sprintf("native fire coverage incomplete: frames%d moves%d routes%d water%d expiry%d removal%d scans%d hits%d spreads%d towns%d", frames, moves, routes, water, expired, removed, scans, hits, spread, destroyed))
	}
	t.Logf("original CPU cases%d frames%d moves%d routes%d water%d expiry%d removal%d scans%d hits%d spreads%d towns%d", len(catalog.Cases), frames, moves, routes, water, expired, removed, scans, hits, spread, destroyed)
}
