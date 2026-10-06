package populous2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func runtimeFilesCampaign(t *testing.T, h *NativeRuntimeHost, device *NativeAudioDevice) NativeFrameRegisterContext {
	t.Helper()
	data, e := os.ReadFile("testdata/startup_campaign_host_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []startupCampaignFixture }
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	var fixture *startupCampaignFixture
	for i := range corpus.Cases {
		f := &corpus.Cases[i]
		if f.Frames[len(f.Frames)-1].PC == 0 {
			fixture = f
			break
		}
	}
	if fixture == nil {
		t.Fatal("actual source campaign input missing")
	}
	in := fixture.Input
	_ = h.Memory.BSS.Write16(0xeb44, 2)
	_ = h.Memory.BSS.Write16(0xeb42, in.Profile)
	_ = h.Memory.BSS.Write16(0xeb46, in.World)
	frame := NativeFrameRegisterContext{D: in.D, AddressBase: h.Memory.BSSBase}
	rules, e := DecodeNativeStartupCampaignHostRules(h.Bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	state := NativeStartupCampaignHostState{}
	for i := range state.Startup.Startup.A {
		state.Startup.Startup.A[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
	}
	audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &frame)
	supplied := NativeStartupCampaignHostCallbacks{NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		return NativeCommandFrameResult{}, fmt.Errorf("unexpected constructor child%x", call.Routine)
	}}}, Campaign: NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: audio, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}}}, Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }}
	step, e := state.Advance(h, &rules, &frame, supplied)
	if e != nil {
		t.Fatal(e)
	}
	for _, event := range in.Events {
		if event.Action != 0 {
			runtimeFilesClick(t, h, &frame, event.Action)
		} else if event.VBlank {
			p := h.Session.Presentation
			if _, e = p.VBlank(NativeMouseSample{CounterX: uint8(p.Input.Mouse.CounterX), CounterY: uint8(p.Input.Mouse.CounterY)}, h.Memory.BSS, &frame); e != nil {
				t.Fatal(e)
			}
		}
		step, e = state.Advance(h, &rules, &frame, supplied)
		if e != nil {
			t.Fatal(e)
		}
	}
	if !step.Complete {
		t.Fatal("actual campaign input did not reach source return")
	}
	raw, e := h.Memory.SnapshotBSS()
	if e != nil {
		t.Fatal(e)
	}
	if fileFrameHash(raw) != fixture.Frames[len(fixture.Frames)-1].BSSHash {
		t.Fatal("actual constructor fixture backing differs")
	}
	return frame
}

func runtimeFilesDrive(t *testing.T, h *NativeRuntimeHost, store *NativeRuntimeFileStore, state *NativeRuntimeFilesState, frame *NativeFrameRegisterContext, device *NativeAudioDevice, routine int, name string) {
	t.Helper()
	for i, v := range append([]byte(name), 0) {
		if e := h.Memory.Code.Write8(0x43c0+i, v); e != nil {
			t.Fatal(e)
		}
	}
	a := [7]NativeRequesterAddress{}
	a[0] = NativeRequesterAddress{Address: h.Memory.CodeBase + 0x43c0, Code: true}
	phase := uint32(0)
	deadline := time.Now().Add(10 * time.Second)
	for {
		result, e := state.AdvanceChild(h, store, NativeStartupResetFrameCall{Routine: routine, Frame: frame, A: &a}, &phase, NativeRuntimeFilesCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}})
		if e != nil {
			t.Fatal(e)
		}
		if result.Complete {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("actual file leaf remained pending")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestNativeRuntimeFilesLoadRefreshesExistingViewsWithoutRawMutation(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(fmt.Sprint(async), func(t *testing.T) {
			h := nativeRuntimeHostTest(t)
			device := runtimeFilesPrelude(t, h)
			frame := runtimeFilesCampaign(t, h, device)
			root := t.TempDir()
			store, e := NewNativeRuntimeFileStore(root, []string{"SAVES"}, async)
			if e != nil {
				t.Fatal(e)
			}
			defer store.Close()
			// These are the real $4324/$434e caller operations, outside the DOS body.
			rebase := func(add bool) {
				t.Helper()
				for _, at := range []int{0xf32, 0xf36} {
					v, e := h.Memory.BSS.Read32(at)
					if e != nil {
						t.Fatal(e)
					}
					if v != 0 {
						base := h.Memory.BSSBase + 0x76c0
						if add {
							v += base
						} else {
							v -= base
						}
						if e = h.Memory.BSS.Write32(at, v); e != nil {
							t.Fatal(e)
						}
					}
				}
			}
			rebase(false)
			expected, e := h.Memory.SnapshotBSS()
			if e != nil {
				t.Fatal(e)
			}
			state := NativeRuntimeFilesState{}
			runtimeFilesDrive(t, h, store, &state, &frame, device, 0x19afc, "SAVES:SESSION.GAM")
			rebase(true)
			saved, e := os.ReadFile(filepath.Join(root, "SESSION.GAM"))
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(saved, expected[NativeGAMStart:NativeGAMEnd]) {
				t.Fatal("actual save regenerated source state")
			}
			// Corrupt both typed projections and live raw values before the real load.
			world, session, memory, code := h.World, h.Session, h.Memory, h.Code
			h.World.Core.Peeps = nil
			h.World.Level.Number = 999
			h.World.Experience = [2][6]uint8{}
			_ = h.Memory.BSS.Write32(0x76f4+26, 0x87654321)
			_ = h.Memory.BSS.Write32(0xe8a4, 0x12345678)
			runtimeFilesDrive(t, h, store, &state, &frame, device, 0x19c1c, "SAVES:SESSION.GAM")
			if !state.RefreshPending {
				t.Fatal("actual read did not request post-return hydration")
			}
			afterLeaf, e := h.Memory.SnapshotBSS()
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(afterLeaf[NativeGAMStart:NativeGAMEnd], saved) {
				t.Fatal("DOS load changed the native file bytes")
			}
			// While the source still owns its continuation, a cache refresh is rejected.
			h.World.nativeCallDepth++
			if e = state.RefreshLoadedViews(h); e == nil {
				t.Fatal("cache refresh admitted borrowed raw memory")
			}
			h.World.nativeCallDepth--
			rebase(true)
			before, e := h.Memory.SnapshotBSS()
			if e != nil {
				t.Fatal(e)
			}
			beforeCode := append([]byte(nil), h.Code.RawData()...)
			town := h.Session.Followers.Town
			town.Property13550 = 0x1234
			town.Scratch136E8[4] = 0x9876
			h.Session.Followers.Town = town
			if e = state.RefreshLoadedViews(h); e != nil {
				t.Fatal(e)
			}
			after, e := h.Memory.SnapshotBSS()
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(before, after) || !bytes.Equal(beforeCode, h.Code.RawData()) {
				t.Fatal("loaded view hydration wrote source BSS/CODE")
			}
			if h.World != world || h.Session != session || h.Memory != memory || h.Code != code {
				t.Fatal("load replaced live owners")
			}
			if state.RefreshPending || h.Session.Followers.Town != town {
				t.Fatal("load replayed or reset unsaved town context")
			}
			rawWorld, _ := h.Memory.BSS.Read16(0xeb46)
			rawLand, _ := h.Memory.BSS.Read16(0xeb22)
			if h.World.Level.Number != int(rawWorld) || h.World.Level.Terrain != int(rawLand) || len(h.World.Core.Peeps) == 0 {
				t.Fatal("saved typed views remained stale")
			}
			for player := range 2 {
				for element := range 6 {
					xp, _ := h.Memory.BSS.Read8(0xe8a4 + player*314 + 0x52 + element)
					if h.World.Experience[player][element] != xp {
						t.Fatal("raw profile XP was regenerated")
					}
				}
				mana, _ := h.Memory.BSS.Read32(0xe8a4 + player*314)
				if h.World.Core.Magnets[player].Mana != int(int32(mana)) {
					t.Fatal("raw mana cache differs")
				}
			}
			if e = h.Session.BeginRaw(h.World, frame); e != nil {
				t.Fatal("loaded actual session cannot resume", e)
			}
			beforeFrame, e := h.Memory.SnapshotBSS()
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(before, beforeFrame) {
				t.Fatal("BeginRaw flushed typed views after load")
			}
			h.Session.finish(nil)
		})
	}
}

func TestNativeRuntimeFilesSuppliedNativeGAMLoadsIntoExistingOwner(t *testing.T) {
	data, e := os.ReadFile("../../.local/native-audit/extracted-pop2-b/ARNY 1.GAM")
	if os.IsNotExist(e) {
		t.Skip("supplied original GAM remains private")
	}
	if e != nil {
		t.Fatal(e)
	}
	h := nativeRuntimeHostTest(t)
	device := runtimeFilesPrelude(t, h)
	root := t.TempDir()
	if e = os.WriteFile(filepath.Join(root, "REFERENCE.GAM"), data, 0600); e != nil {
		t.Fatal(e)
	}
	store, e := NewNativeRuntimeFileStore(root, []string{"SAVES"}, true)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	frame := NativeFrameRegisterContext{D: [8]uint32{0x12340000, 1, 2, 3, 4, 5, 6, 7}, AddressBase: h.Memory.BSSBase}
	state := NativeRuntimeFilesState{}
	runtimeFilesDrive(t, h, store, &state, &frame, device, 0x19c1c, "SAVES:REFERENCE.GAM")
	raw, e := h.Memory.SnapshotBSS()
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(raw[NativeGAMStart:NativeGAMEnd], data[:NativeGAMSize]) {
		t.Fatal("actual supplied GAM read differs")
	}
	// Actual successful $42d6 adds the follower base once; DOS does not.
	for _, at := range []int{0xf32, 0xf36} {
		v, e := h.Memory.BSS.Read32(at)
		if e != nil {
			t.Fatal(e)
		}
		if v != 0 {
			if e = h.Memory.BSS.Write32(at, v+h.Memory.BSSBase+0x76c0); e != nil {
				t.Fatal(e)
			}
		}
	}
	before, e := h.Memory.SnapshotBSS()
	if e != nil {
		t.Fatal(e)
	}
	if e = state.RefreshLoadedViews(h); e != nil {
		t.Fatal(e)
	}
	after, e := h.Memory.SnapshotBSS()
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("supplied GAM cache refresh rewrote native bytes")
	}
	w := h.World
	if w.Level.Number != 27 || w.Level.Terrain != 1 || w.NativeClock != 112 || w.NativeProfileSide != 1 || w.NativeGameMode != 2 || w.Core.RandomState() != 0xdb9e731b || w.Deity.Name != "DAMOCLES" || w.Deity.Bolts != 13 || w.Deity.FaceParts != [3]uint8{1, 4, 1} {
		t.Fatal("supplied native session cache differs")
	}
	if w.Core.Magnets[0].Mana != 2020 || w.Core.Magnets[1].Mana != 116 || w.Core.FollowerAttrition(0, false) != 3 || w.Core.FollowerAttrition(1, false) != 7 || w.Rules[0].Raw != 35 || w.Rules[1].Raw != 4 {
		t.Fatal("supplied ledger/rules cache differs")
	}
	if e = w.Occupancy.Validate(); e != nil {
		t.Fatal(e)
	}
}
