package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type wallNativeInput struct {
	whirlwindNativeInput
	SourceD7        uint16
	ImageReferences []uint16
}
type wallNativeImage struct {
	Reference, Offset uint16
	Layers            [][3]int
	Cue               uint16
}
type wallNativeFrame struct {
	whirlwindNativeFrame
	Scratch  [4]NativeRecordReference
	Result   int32
	Exit     string
	Images   []wallNativeImage
	Admitted bool
}
type wallNativeFixture struct {
	Input       wallNativeInput
	InitialHash string
	Frames      []wallNativeFrame
}

func wallNativeFixtureMemory(c wallNativeInput) *whirlwindNativeTestMemory {
	input := c.whirlwindNativeInput
	initial := make([]nativeHeroPatch, 0, 3400+len(input.Initial))
	for i := range 200 {
		at := 0x5f50 + i*16
		for j := range 16 {
			initial = append(initial, nativeHeroPatch{Address: at + j, Width: 1, Value: uint32(uint8(i*16 + j + 5))})
		}
		initial = append(initial, nativeHeroPatch{Address: at + 12, Width: 1, Value: 0})
	}
	input.Initial = append(initial, input.Initial...)
	return whirlwindNativeFixtureMemory(input)
}

// These are original relocated CPU bodies, including actual $1275a entry and
// every graph mutation. The image proof captures $e658/$e5ee/$eb58's chosen
// offsets and original composite layers; no extra wall-climbing height exists.
func TestNativeWallPoolCreationCrossingAndSpritesAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/wall_native_full.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		SourceCodeSHA256 string
		FullImageEnd     int
		Entries          map[string]int
		Cases            []wallNativeFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1193 || catalog.FullImageEnd != 0xeb90 || catalog.Entries["Pool"] != 0x161cc || catalog.Entries["Follower"] != 0x1156c || catalog.SourceCodeSHA256 != "86ad7f58200fc3295328d3a7ebad4e0ae6663be402d5ed090981b1e53b0a3841" {
		t.Fatal("native wall oracle provenance incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeNativeWallRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := DecodeFollowerEntryRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	frames, creates, removes, advances, crossings, attacks, bounces, redispatches, images := 0, 0, 0, 0, 0, 0, 0, 0, 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			c := fixture.Input
			m := wallNativeFixtureMemory(c)
			if m.err != nil {
				t.Fatal(m.err)
			}
			if whirlwindNativeHash(m) != fixture.InitialHash {
				t.Fatal("complete native wall initial BSS differs")
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := []whirlwindNativeCall{}
			link := func(ref NativeRecordReference) error {
				calls = append(calls, whirlwindNativeCall{Kind: "link", Reference: uint16(ref)})
				return m.insert(ref)
			}
			unlink := func(ref NativeRecordReference) error {
				calls = append(calls, whirlwindNativeCall{Kind: "unlink", Reference: uint16(ref)})
				return m.unlink(ref)
			}
			fail := func(name string) error { return fmt.Errorf("unexpected native wall fixture operation %s", name) }
			ec := FollowerEntryCallbacks{
				Head: func(tile NativePackedTile) (NativeRecordReference, error) {
					value, err := memory.Read16(nativeWhirlwindGrid(uint16(tile)) + 2)
					return NativeRecordReference(value), err
				},
				Node: func(ref NativeRecordReference) (FollowerEntryNode, error) {
					at := cleanupRecordAddress(ref)
					kind, err := memory.Read8(at)
					if err != nil {
						return FollowerEntryNode{}, err
					}
					owner, err := memory.Read8(at + 12)
					if err != nil {
						return FollowerEntryNode{}, err
					}
					next, err := memory.Read16(at + 2)
					return FollowerEntryNode{Kind: kind, Owner: owner, Next: NativeRecordReference(next)}, err
				},
				Read: func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m.cleanupMemory, ref) },
				Write: func(ref NativeRecordReference, a FollowerEntryActor) error {
					return winEntryWrite(m.cleanupMemory, ref, a)
				},
				ReadWord: func(ref NativeRecordReference, offset uint16) (uint16, error) {
					return memory.Read16(cleanupRecordAddress(ref) + int(offset))
				},
				ReadLong: func(ref NativeRecordReference, offset uint16) (uint32, error) {
					return memory.Read32(cleanupRecordAddress(ref) + int(offset))
				},
				WriteWord: func(ref NativeRecordReference, offset, value uint16) error {
					return memory.Write16(cleanupRecordAddress(ref)+int(offset), value)
				},
				Tile:      func(tile NativePackedTile) (uint8, error) { return memory.Read8(nativeWhirlwindGrid(uint16(tile)) + 1) },
				GodMode:   func(owner uint8) (uint16, error) { return memory.Read16(heroGodAddress(owner) + 12) },
				Tick:      func() uint16 { return m.word(0xf42) },
				SetLeader: func(uint8, NativeRecordReference) error { return fail("leader") }, Selected: func() NativeRecordReference { return 0 }, Select: func(NativeRecordReference) error { return fail("selection") }, Founded: func(uint8) error { return fail("founding") }, Unlink: unlink,
				EvaluateTown: func(NativeRecordReference) (int, error) { return 0, fail("town evaluation") }, ClearFarms: func(NativeRecordReference, uint8) error { return fail("farms") }, ClearHeroLinks: func(NativeRecordReference) error { return fail("hero links") }, Sound: func(uint16) error { return fail("sound") },
			}
			cb := NativeWallCallbacks{Memory: memory, SourceD7: c.SourceD7, Link: link, Unlink: unlink,
				Move: func(ref NativeRecordReference, x, y uint16) (bool, error) {
					calls = append(calls, whirlwindNativeCall{Kind: "move", Reference: uint16(ref), X: x, Y: y})
					at := cleanupRecordAddress(ref)
					changed := nativeOccupancyPackedTile(m.word(at+6), m.word(at+8)) != nativeOccupancyPackedTile(x, y)
					return changed, neutralComposedMove(m.cleanupMemory, ref, x, y)
				},
				Enter: func(ref NativeRecordReference) error {
					calls = append(calls, whirlwindNativeCall{Kind: "entry", Reference: uint16(ref)})
					_, err := entry.Enter(ref, ec)
					return err
				},
			}
			placement := NativeWallPlacementState{}
			frameIndex := 0
			for opIndex, op := range c.Operations {
				whirlwindNativePatch(m, op.Initial)
				for iteration := range max(1, op.Repeat) {
					before := whirlwindNativeBytes(m)
					calls = []whirlwindNativeCall{}
					ref := NativeRecordReference(op.Reference)
					created := false
					probe := int16(0)
					exit := "return"
					switch op.Kind {
					case "create", "createat":
						x, y := c.X, c.Y
						if op.Kind == "createat" {
							x, y = uint8(op.Target), uint8(op.Target>>8)
						}
						var step NativeWallCreation
						step, err = rules.Create(c.Owner, x, y, &placement, cb)
						created = step.Created
						if created {
							creates++
						}
					case "pool":
						var step NativeWallPass
						step, err = rules.TickPool(cb)
						removes += step.Removed
						advances += step.Advanced
					case "probe":
						probe, err = rules.Hero.Probe(ref, NativePackedTile(uint16(c.Y)<<8|uint16(c.X)), 1, 0, memory)
					case "follower", "fault":
						state := m.byte(cleanupRecordAddress(ref) + 22)
						if m.byte(cleanupRecordAddress(ref)+12) != 0 && (state == 4 || state == 0x2a) {
							var step NativeWallFollowerStep
							step, err = rules.TickFollower(ref, cb)
							exit = "123b4"
							if step.Redispatch {
								exit = "112b8"
								redispatches++
							}
							if step.Crossed {
								crossings++
							}
							if step.WallBroken {
								attacks++
							}
							if step.Blocked {
								bounces++
							}
						}
						if op.Kind == "fault" {
							if err == nil {
								t.Fatal("original odd-stage address fault was fabricated")
							}
							err = nil
							exit = "fault"
						}
					default:
						t.Fatalf("unknown native wall operation %s", op.Kind)
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
					rng, _ := memory.Read32(0xeb28)
					if whirlwindNativeHash(m) != frame.Hash || rng != frame.RNG || frame.RandomDraws != 0 {
						t.Fatalf("operation%d iteration%d full BSS/RNG differs", opIndex, iteration)
					}
					if !reflect.DeepEqual(calls, frame.Calls) {
						t.Fatalf("operation%d iteration%d native calls differ: actual%v native%v", opIndex, iteration, calls, frame.Calls)
					}
					if placement.Neighbors != frame.Scratch {
						t.Fatalf("native mutable scratch pointers differ: actual%v native%v", placement.Neighbors, frame.Scratch)
					}
					if (op.Kind == "create" || op.Kind == "createat") && created != frame.Admitted {
						t.Fatal("native wall admission flag differs")
					}
					if op.Kind == "probe" && int32(probe) != frame.Result {
						t.Fatalf("native signed probe returned%d instead of%d", probe, frame.Result)
					}
					if (op.Kind == "follower" || op.Kind == "fault") && exit != frame.Exit {
						t.Fatalf("native follower exit differs: actual%s native%s", exit, frame.Exit)
					}
					for _, image := range frame.Images {
						actual, offset, err := rules.Frame(NativeRecordReference(image.Reference), memory)
						if err != nil {
							t.Fatal(err)
						}
						layers := make([][3]int, len(actual.Layers))
						for i, layer := range actual.Layers {
							layers[i] = [3]int{layer.X, layer.Y, layer.Sprite}
						}
						if offset != image.Offset || !reflect.DeepEqual(layers, image.Layers) || uint16(actual.SoundCue*10) != image.Cue {
							t.Fatalf("original image reference%04x differs: offset%04x/%04x layers%v/%v cue%d/%d", image.Reference, offset, image.Offset, layers, image.Layers, actual.SoundCue*10, image.Cue)
						}
						images++
					}
				}
			}
			if frameIndex != len(fixture.Frames) {
				t.Fatal("native trace has extra frames")
			}
		})
	}
	if frames < 10000 || creates < 20 || removes < 100 || advances < 100 || crossings < 100 || attacks < 70 || bounces < 100 || redispatches < 10 || images < 1000 {
		t.Fatalf("native wall coverage incomplete: frames%d creates%d removes%d advances%d crossings%d attacks%d bounces%d redispatches%d images%d", frames, creates, removes, advances, crossings, attacks, bounces, redispatches, images)
	}
	t.Logf("original CPU cases%d frames%d creates%d removes%d advances%d crossings%d attacks%d bounces%d redispatches%d images%d", len(catalog.Cases), frames, creates, removes, advances, crossings, attacks, bounces, redispatches, images)
}
