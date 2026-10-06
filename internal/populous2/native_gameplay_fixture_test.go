package populous2

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type nativeGameplayCPUFixture struct {
	Input struct {
		Controlled   bool
		Initial      []commandNativePatch
		AfterPackets []commandNativePatch
		Links        []uint16
		Packets      []struct {
			Tick                 int
			Owner, Command, X, Y uint8
		}
		Placement, PlacementOwner uint8
		Name                      string
		Land                      int
		Seed                      uint32
		Frames                    int
		Commands                  []byte
		Interval                  int
		View                      uint16
	}
	Startup        nativeGameplayCPUPoint
	Frames         []nativeGameplayCPUPoint
	ResultBoundary bool
	ErrorPC        uint32
	FollowerStates map[string]int
	FXStates       map[string]int
}
type nativeGameplayCPUPoint struct {
	Tick                                   int
	Routine                                uint32
	D                                      [8]uint32
	A                                      [7]uint32
	BSSHash, CodeHash, ChipHash, HeapHash  string
	RNG                                    uint32
	TownFlag, TownProperty, MinimapVariant uint16
}

func runNativeGameplayCPUCorpus(t *testing.T, filename string, expectedCases int) {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []nativeGameplayCPUFixture
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != expectedCases {
		t.Fatalf("native gameplay corpus changed: %d/%d", len(corpus.Cases), expectedCases)
	}
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			h, device, frame := nativeGameplayIntegrationStartup(t, f.Input.Land, f.Input.Seed)
			check := func(want nativeGameplayCPUPoint) {
				t.Helper()
				bss, err := h.Memory.SnapshotBSS()
				if err != nil {
					t.Fatal(err)
				}
				// Snapshot the physical allocation and each explicit callback-owned
				// span. This is a diagnostic copy; gameplay reads remain live.
				code := append([]byte(nil), h.Code.RawData()[:0x3fa2c]...)
				for _, span := range [][2]int{{0x3ea, 2}, {0x77a, 8}, {0xa2a, 12}, {0xeee0, 2}, {0x1117c, 4}, {0x124a0, 2}, {0x13350, 2}, {0x13550, 2}, {0x136e8, 100}, {0x18426, 8}, {0x185a8, 1330}} {
					for i := span[0]; i < span[0]+span[1]; i++ {
						code[i], err = h.Memory.Code.Read8(i)
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				if want.Tick == -1 {
					for i, value := range code {
						observed, e := h.Memory.Code.Read8(i)
						if e != nil || value != observed {
							t.Fatalf("diagnostic CODE snapshot missed live byte%x", i)
						}
					}
				}
				audioAllocation, err := h.Host.Span(0x700000, 0x1e0dc)
				if err != nil {
					t.Fatal(err)
				}
				backgroundAllocation, err := h.Host.Span(0x71e0dc, 0x7d00)
				if err != nil {
					t.Fatal(err)
				}
				heap := append(append([]byte(nil), audioAllocation...), backgroundAllocation...)
				d := frame.D
				if h.Session.world != nil {
					d = h.Session.Frame.D
				}
				if d != want.D {
					t.Fatalf("tick%d routine%x D differs: %08x/%08x", want.Tick, want.Routine, d, want.D)
				}
				if fileFrameHash(bss) != want.BSSHash {
					t.Fatalf("tick%d routine%x BSS differs", want.Tick, want.Routine)
				}
				if fileFrameHash(code) != want.CodeHash {
					if dir := os.Getenv("NATIVE_GAMEPLAY_DEBUG"); dir != "" && (want.Tick == -1 || want.Tick == 0 || want.Tick == 6 || want.Tick == 15 || want.Tick == 88 || want.Tick == 102) {
						_ = os.MkdirAll(dir, 0755)
						_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("go-%s-%d-%x-code.bin", f.Input.Name, want.Tick, want.Routine)), code, 0600)
					}
					t.Fatalf("tick%d routine%x CODE differs; caches13350=%d/%d 13550=%d/%d 124a0=%d/%d", want.Tick, want.Routine, h.Session.Followers.Pass.TownCacheFlag, want.TownFlag, h.Session.Followers.Town.Property13550, want.TownProperty, h.Session.Followers.Pass.MinimapVariant, want.MinimapVariant)
				}
				if fileFrameHash(h.Session.Presentation.Chip) != want.ChipHash {
					t.Errorf("tick%d routine%x chip differs", want.Tick, want.Routine)
				}
				if fileFrameHash(heap) != want.HeapHash {
					t.Errorf("tick%d routine%x heap differs", want.Tick, want.Routine)
				}
			}
			check(f.Startup)
			_ = h.Memory.BSS.Write16(0xf0c, f.Input.View)
			for side := 1; side <= 2; side++ {
				god := 0xe76a + side*314
				_ = h.Memory.BSS.Write32(god, 1000000)
				for i := 0; i < 6; i++ {
					_ = h.Memory.BSS.Write8(god+0x52+i, 255)
				}
			}
			if f.Input.Controlled {
				for at := 0x5f50; at < 0xe740; at++ {
					_ = h.Memory.BSS.Write8(at, 0)
				}
				for at := 0x4f44; at < 0x5f44; at++ {
					_ = h.Memory.BSS.Write8(at, 0)
				}
				for pos := 0; pos < 4096; pos++ {
					_ = h.Memory.BSS.Write32(0xf44+pos*4, 0x010f0000)
				}
				_ = h.Memory.BSS.Write16(0xeb44, 8)
				for _, at := range []int{0xe8a4 + 4, 0xe9de + 4} {
					_ = h.Memory.BSS.Write32(at, 0)
				}
				for _, at := range []int{0xe8a4 + 8, 0xe9de + 8} {
					_ = h.Memory.BSS.Write16(at, 0)
				}
				for _, p := range f.Input.Initial {
					renderFramePatch(h.Memory.BSS, nativeHeroPatch{p.Address, p.Width, p.Value})
				}
				for _, ref := range append([]uint16{0x7080, 0x708e, 0x709c}, f.Input.Links...) {
					if err := h.World.nativeRuntimeInsert(NativeRecordReference(ref)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := h.Session.BeginRaw(h.World, frame); err != nil {
				t.Fatal(err)
			}
			defer h.Session.finish(nil)
			resultBoundary := errors.New("actual381E result entry")
			callbacks := NativeFrameSessionCallbacks{Result: func(uint16, *NativeFrameRegisterContext) error { return resultBoundary }, Audio: NativeFrameAudioCallbacks{Command: device.Command}, DirectSound: func(cue uint16) error { return device.DirectCue(cue, &h.Session.Frame) }, Bitmap: h.Bitmap}
			h.Session.directSound = callbacks.DirectSound
			h.Session.bitmapResolver = h.Bitmap
			bindings := NativeCommandWorldBindings{WallRules: &h.Session.wallRules, WallPlacement: &h.Session.wallPlacement}
			index := 0
			for tick := 0; tick < f.Input.Frames; tick++ {
				interval := f.Input.Interval
				if interval == 0 {
					interval = 8
				}
				for _, packet := range f.Input.Packets {
					if packet.Tick == tick {
						data := []byte{packet.Owner, packet.Command, packet.X, packet.Y, 0, 0, 0, 0, 2, 0}
						for i, v := range data {
							_ = h.Memory.BSS.Write8(0xeb56+i, v)
						}
						context := h.Session.Frame.CommandContext()
						if _, err := h.Session.commandRules.Execute(0xeb56, &context, h.World.nativeNormalCommandCallbacks(bindings)); err != nil {
							t.Fatal(err)
						}
						h.Session.Frame.SetCommandContext(context)
						check(f.Frames[index])
						index++
						_ = h.Memory.BSS.Write8(0xeb57, 0)
						for _, p := range f.Input.AfterPackets {
							renderFramePatch(h.Memory.BSS, nativeHeroPatch{p.Address, p.Width, p.Value})
						}
						if f.Input.Placement != 0 && tick == 0 {
							chosen := -1
							for pos := 0; pos < 4096; pos++ {
								tile, _ := h.Memory.BSS.Read8(0xf45 + pos*4)
								if tile == f.Input.Placement {
									chosen = pos
									break
								}
							}
							if chosen < 0 {
								t.Fatal("actual native creator did not place target ground")
							}
							at := 0x76f4
							for i := 0; i < 52; i++ {
								_ = h.Memory.BSS.Write8(at+i, 0)
							}
							for _, p := range []nativeHeroPatch{{at, 1, 2}, {at + 6, 2, uint32(chosen%64<<8 | 128)}, {at + 8, 2, uint32(chosen/64<<8 | 128)}, {at + 12, 1, uint32(f.Input.PlacementOwner)}, {at + 18, 1, 20}, {at + 22, 1, 2}, {at + 26, 4, 1000}} {
								renderFramePatch(h.Memory.BSS, p)
							}
							if err := h.World.nativeRuntimeInsert(52); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				if tick%interval == 0 && tick/interval < len(f.Input.Commands) {
					packet := []byte{1, f.Input.Commands[tick/interval], 32, 32, 0, 0, 0, 0, 2, 0}
					for i, v := range packet {
						_ = h.Memory.BSS.Write8(0xeb56+i, v)
					}
					context := h.Session.Frame.CommandContext()
					if _, err := h.Session.commandRules.Execute(0xeb56, &context, h.World.nativeNormalCommandCallbacks(bindings)); err != nil {
						t.Fatal(err)
					}
					h.Session.Frame.SetCommandContext(context)
					check(f.Frames[index])
					index++
					_ = h.Memory.BSS.Write8(0xeb57, 0)
				}
				h.Session.Audio.Entries = h.Session.Image.AudioBank
				if err := h.ImageAudioCode.SetOwner(NativeAudioCodeOwner); err != nil {
					t.Fatal(err)
				}
				bitmap, err := h.Session.Presentation.BackBuffer()
				if err != nil {
					t.Fatal(err)
				}
				cb := h.Session.physicsCallbacks(callbacks, bitmap)
				cb.Swap = func(*NativeFrameRegisterContext) (bool, error) { return false, nil }
				h.Session.Pass = NativeFramePassState{}
				done, err := h.Session.Pass.TickFramePass(&h.Session.Frame, cb)
				atResult := errors.Is(err, resultBoundary)
				if err != nil && !atResult {
					t.Fatalf("tick%d physics: %v", tick, err)
				}
				if !atResult && (done || h.Session.Pass.Stage != NativeFrameSwap) {
					t.Fatal("source before-swap boundary lost")
				}
				check(f.Frames[index])
				index++
				if atResult {
					if !f.ResultBoundary {
						t.Fatal("unexpected actual result entry")
					}
					break
				}
				h.Session.Image.AudioBank = h.Session.Audio.Entries
				if err := h.ImageAudioCode.SetOwner(NativeImageCodeOwner); err != nil {
					t.Fatal(err)
				}
				if _, err := h.Session.Presentation.Swap(&h.Session.Frame); err != nil {
					t.Fatal(err)
				}
				clock, err := h.Memory.BSS.Read32(0xf40)
				if err != nil {
					t.Fatal(err)
				}
				_ = h.Memory.BSS.Write32(0xf40, clock+1)
			}
			if index != len(f.Frames) || f.ErrorPC != 0 {
				t.Fatalf("source frame count/fault: %d/%d pc%x", index, len(f.Frames), f.ErrorPC)
			}
		})
	}
}
