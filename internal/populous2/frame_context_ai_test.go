package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type frameAIInput struct {
	aiNativeInput
	D [8]uint32
}

func TestNativeFrameAllAIPolicyRegistersAgainstCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/frame_ai_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct {
		Cases []struct {
			Input frameAIInput
			D     [8]uint32
			Hash  string
		}
	}
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	rules, e := DecodeNativeAIRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 5248 {
		t.Fatalf("full AI policy context coverage %d", len(catalog.Cases))
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			raw := aiNativeFixtureMemory(f.Input.aiNativeInput)
			initial := make([]byte, 0x11280)
			copy(initial, raw[:])
			w := aiFixtureWorld(t, initial)
			w.nativeCallDepth++
			frame := NativeFrameRegisterContext{D: f.Input.D}
			low := NativeAIRegisterContext{D4: uint16(frame.D[4]), D5: uint16(frame.D[5]), Frame: &frame}
			cb := w.nativeAICallbacks(nil)
			cb.Frame = &frame
			random := cb.Random
			cb.Random = func() uint16 { value := random(); frame.D[0] = uint32(value); return value }
			god, command := 0xe76a+f.Input.Side*314, 0xeb56+(f.Input.Side-1)*10
			switch f.Input.Mode {
			case "urgent":
				_, e = rules.Urgent(god, command, cb)
			case "release":
				_, e = rules.ReleaseTown(god, command, cb)
			case "expand":
				_, e = rules.Expand(god, command, cb)
			case "offensive":
				_, e = rules.Offensive(god, command, &low, cb)
			case "magnet":
				_, e = rules.MagnetMode(god, command, cb)
			case "water":
				frame.Word(7, uint16(f.Input.Side))
				_, e = rules.WaterTarget(0x76f4, uint16(f.Input.Side), &low, cb.Memory)
			case "tick":
				_, e = rules.TickFrame(&frame, cb)
			default:
				t.Fatalf("unproven AI routine %s", f.Input.Mode)
			}
			w.nativeCallDepth--
			if e != nil {
				t.Fatal(e)
			}
			if frame.D != f.D {
				t.Fatalf("full policy registers got%08x native%08x", frame.D, f.D)
			}
			b := aiFixtureWorldBytes(w, initial)
			copy(b[0xeb90:], w.NativeRedrawBytes[:])
			if fmt.Sprintf("%x", sha256.Sum256(b)) != f.Hash {
				t.Fatal("full AI policy BSS/RNG differs")
			}
		})
	}
}
