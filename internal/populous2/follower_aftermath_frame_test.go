package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

type aftermathFrameFixture struct {
	aftermathFixture
	Trace []struct {
		aftermathFixtureFrame
		InputD, D [8]uint32
	}
}

func TestNativeFollowerAftermathFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_aftermath_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []aftermathFrameFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 603 {
		t.Fatal("native aftermath full-register corpus incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeNativeFollowerAftermathFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	towns, err := DecodeTownCombatRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, fixture := range fixtures {
		m := aftermathMemory(fixture.aftermathFixture)
		memory := FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return m.bytes[at], nil }, Read16: func(at int) (uint16, error) { return m.word(at), nil }, Read32: func(at int) (uint32, error) { return m.long(at), nil }, Write8: func(at int, v uint8) error { m.bytes[at] = v; return nil }, Write16: func(at int, v uint16) error { m.putWord(at, v); return nil }, Write32: func(at int, v uint32) error { m.putLong(at, v); return nil }}
		unlink := func(ref NativeRecordReference) error { return aftermathUnlink(m, ref) }
		insert := func(ref NativeRecordReference) error { m.insert(cleanupRecordAddress(ref)); return nil }
		clear := func(ref NativeRecordReference, tile uint8) error {
			return evaluator.ClearFarms(ref, tile, townCombatEvaluatorCallbacks(m))
		}
		for _, reference := range fixture.Trace {
			if reference.Exit != "123b4" && reference.Exit != "12462" {
				break
			}
			frame := NativeFrameRegisterContext{D: reference.InputD, AddressBase: 0x200000}
			cb := NativeFollowerAftermathFrameCallbacks{Memory: memory, Frame: &frame, Unlink: unlink,
				Cleanup: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					_, err := CleanupFollowerWithFrame(ref, c, FollowerCleanupCallbacks{Memory: memory, Unlink: unlink, Insert: insert, ClearFarms: clear})
					return err
				},
				ClearLeader: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					_, err := ClearFollowerLeaderWithFrame(ref, c, FollowerLeaderCallbacks{Memory: memory, Unlink: unlink, Insert: insert})
					return err
				},
				DestroyTown: func(ref NativeRecordReference) error {
					_, err := towns.Destroy(ref, TownCombatCallbacks{Read: m.read, Write: m.write, ClearFarms: clear,
						EvaluateTown: func(ref NativeRecordReference) (int, error) {
							value, err := evaluator.Evaluate(ref, 123, townCombatEvaluatorCallbacks(m))
							return int(value), err
						},
						Cleanup: func(ref NativeRecordReference, mode uint16) error {
							_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: unlink, Insert: insert, ClearFarms: clear})
							return err
						},
					})
					return err
				},
			}
			flow, err := rules.Tick(52, cb)
			if err != nil {
				t.Fatal(fixture.Input.Name, reference.Tick, err)
			}
			wantFlow := NativeFollowerNext
			if reference.Exit == "123b4" {
				wantFlow = NativeFollowerCount
			}
			if flow != wantFlow || frame.D != reference.D {
				t.Fatalf("%s frame%d aftermath continuation differs: flow%d/%d D%x / %x", fixture.Input.Name, reference.Tick, flow, wantFlow, frame.D, reference.D)
			}
			assertAftermathFrame(t, m, reference.aftermathFixtureFrame)
			checked++
		}
	}
	if checked != 6729 {
		t.Fatalf("native aftermath body coverage differs: %d frames", checked)
	}
	t.Logf("Compared %d complete native aftermath body frames", checked)
}

func TestWorldFollowerAftermathFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_aftermath_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []aftermathFrameFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeNativeFollowerAftermathFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, fixture := range fixtures {
		m := aftermathMemory(fixture.aftermathFixture)
		w := installNativeFixtureWorld(t, m.bytes[:], nil)
		for _, reference := range fixture.Trace {
			if reference.Exit != "123b4" && reference.Exit != "12462" {
				break
			}
			frame := NativeFrameRegisterContext{D: reference.InputD, AddressBase: 0x200000}
			w.nativeCallDepth++
			flow, err := rules.Tick(52, w.nativeAftermathFrameCallbacks(&frame))
			w.nativeCallDepth--
			if err != nil {
				t.Fatal(fixture.Input.Name, reference.Tick, err)
			}
			wantFlow := NativeFollowerNext
			if reference.Exit == "123b4" {
				wantFlow = NativeFollowerCount
			}
			if flow != wantFlow || frame.D != reference.D {
				t.Fatalf("World%s frame%d continuation differs: flow%d/%d D%x / %x", fixture.Input.Name, reference.Tick, flow, wantFlow, frame.D, reference.D)
			}
			all := nativeFixtureWorldImage(w, m.bytes[:])
			var result entryFixtureMemory
			copy(result.bytes[:], all)
			assertAftermathFrame(t, &result, reference.aftermathFixtureFrame)
			checked++
		}
	}
	if checked != 6729 {
		t.Fatalf("World aftermath coverage differs: %d frames", checked)
	}
}
