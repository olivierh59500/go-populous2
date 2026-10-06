package populous2

import (
	"fmt"
	"testing"
)

func TestNativeRuntimeWinningResultReturnsThroughAwardAndReset(t *testing.T) {
	for _, initialWorld := range []uint16{32, 999} {
		t.Run(fmt.Sprintf("world-%d", initialWorld), func(t *testing.T) {
			testNativeRuntimeWinningResultReturnsThroughAwardAndReset(t, initialWorld)
		})
	}
}

func testNativeRuntimeWinningResultReturnsThroughAwardAndReset(t *testing.T, initialWorld uint16) {
	h := nativeRuntimeHostTest(t)
	device := runtimeFilesPrelude(t, h)
	c := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	rules, err := DecodeNativeStartupCampaignHostRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.InterruptVectors(0x39e, &c, func(uint32) (uint16, error) { return 0, nil }, nil); err != nil {
		t.Fatal(err)
	}
	audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &c)
	campaign := NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: audio, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}}}
	startup := NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }}}, Campaign: campaign, Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }}}
	// This controlled caller begins after the proven population recount;
	// original score, award, deity and reset children still execute in full.
	for _, patch := range []nativeHeroPatch{{0xeb42, 2, 1}, {0xeb44, 2, 2}, {0xeb46, 2, uint32(initialWorld)}, {0xeb24, 4, 0x12345678}, {0xeb6e, 2, 4}, {0xeb70, 2, 0x0820}, {0xeb72, 2, 0x1821}, {0xe8a8, 4, 1000}, {0xe9e2, 4, 0}, {0xe8e0, 4, 90000}, {0xe8e4, 4, 100000}, {0xe8a4 + 0x48, 2, 0}, {0xe9de + 0x48, 2, 0}, {0xe8a4 + 0x58, 2, 5}} {
		renderFramePatch(h.Memory.BSS, patch)
	}
	// Retain the real follower-result caller while the result and all reset
	// requesters yield; decoding must wait for the source return boundary.
	if err := h.Session.BeginRaw(h.World, c); err != nil {
		t.Fatal(err)
	}
	h.Session.Phase = NativeFrameSessionPhysics
	h.Session.Pass.Stage = NativeFrameFollowers
	h.Session.followersCompleted, h.Session.resultPending = true, true
	h.Session.resultIdentity = 2
	defer h.Session.finish(nil)
	if err := h.RefreshWorldCaches(); err == nil {
		t.Fatal("idle cache refresh accepted a retained winning result")
	}
	var deity NativeRuntimeDeity
	var progression NativeRuntimeProgressionState
	operations := NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}
	pc := NativeRuntimeProgressionCallbacks{Audio: operations, Ownership: startup.Ownership, Child: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		if call.Routine == 0xb740 {
			return deity.AdvanceChild(h, call, phase, campaign)
		}
		err := RunNativeProgressionChild(call.Routine, h, call.Frame, call.A)
		return NativeCommandFrameResult{Complete: err == nil}, err
	}}
	result := NativeRuntimeResultHost{Rules: rules, Startup: startup, Callbacks: NativeRuntimeResultCallbacks{Audio: operations, Sound: device.DirectCue, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}, Progression: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		return progression.AdvanceChild(h, call, phase, pc)
	}}
	blank := func(left bool) {
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY), Left: left}, h.Memory.BSS, &c); err != nil {
			t.Fatal(err)
		}
	}
	for tick := 0; tick < 102; tick++ {
		done, err := result.Advance(h, 2, &c)
		if err != nil || done {
			t.Fatal("result wait failed", tick, done, err)
		}
		blank(false)
	}
	if result.Result == nil || result.Result.PC != 0x39fa {
		t.Fatal("winning result requester missing")
	}
	nativeRuntimeClickAction(t, h, &c, 2)
	endingSeen := false
	for tick := 0; tick < 140 && deity.State == nil; tick++ {
		done, err := result.Advance(h, 2, &c)
		if err != nil || done {
			t.Fatal("award failed", tick, done, err)
		}
		endingSeen = endingSeen || progression.Ending != nil
		blank(tick == 51)
	}
	if deity.State == nil {
		t.Fatalf("winning result did not enter real deity allocation: progression=$%x ending=%v", progression.PC, progression.Ending)
	}
	if endingSeen != (initialWorld == 999) {
		t.Fatal("winning result took the wrong ending branch", initialWorld, endingSeen)
	}
	for tick := 0; tick < 40; tick++ {
		blank(false)
		_, err := result.Advance(h, 2, &c)
		if err != nil {
			t.Fatal(err)
		}
	}
	nativeRuntimeClickAction(t, h, &c, 66)
	for tick := 0; tick < 40 && result.Reset == nil; tick++ {
		done, err := result.Advance(h, 2, &c)
		if err != nil || done {
			t.Fatal(done, err)
		}
		blank(false)
	}
	if result.Reset == nil || result.Reset.Menu == nil {
		t.Fatal("winning award did not return to original reset menu")
	}
	worldAfterAward, err := h.Memory.BSS.Read16(0xeb46)
	if err != nil || (initialWorld == 32 && worldAfterAward <= initialWorld) || (initialWorld == 999 && worldAfterAward != 0) {
		t.Fatal("native winning world progression missing", initialWorld, worldAfterAward, err)
	}
	if result.RefreshPending {
		t.Fatal("winning result refreshed before the actual reset returned")
	}
	for range 18 {
		blank(false)
		done, err := result.Advance(h, 2, &c)
		if err != nil || done {
			t.Fatal("reset menu fade failed", done, err)
		}
	}
	nativeRuntimeClickAction(t, h, &c, 4)
	complete := false
	for tick := 0; tick < 40 && !complete; tick++ {
		complete, err = result.Advance(h, 2, &c)
		if err != nil {
			t.Fatal("reset campaign entry failed", err)
		}
		if !complete {
			blank(false)
		}
	}
	if complete || result.Reset == nil || result.Reset.Selection == nil {
		t.Fatal("winning reset did not retain the real campaign chooser", complete)
	}
	for range 18 {
		blank(false)
		complete, err = result.Advance(h, 2, &c)
		if err != nil || complete {
			t.Fatal("reset campaign chooser wait failed", complete, err)
		}
	}
	nativeRuntimeClickAction(t, h, &c, 6)
	for tick := 0; tick < 40 && !complete; tick++ {
		complete, err = result.Advance(h, 2, &c)
		if err != nil {
			t.Fatal("reset campaign creation failed", err)
		}
		if !complete {
			blank(false)
		}
	}
	if !complete || result.Result != nil || result.Reset != nil || !result.RefreshPending || h.World.nativeCallDepth != 1 {
		t.Fatal("winning result/reset did not return inside the retained frame", complete, result.RefreshPending)
	}
	worldAfterReset, err := h.Memory.BSS.Read16(0xeb46)
	if err != nil || worldAfterReset != worldAfterAward {
		t.Fatal("campaign chooser changed the winning world", worldAfterAward, worldAfterReset, err)
	}
	bssBefore, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	codeBefore := nativeRuntimeResultCodeBytes(t, h)
	registers := c.D
	refreshes := 0
	// Match the Game caller: consume this request only after the real source
	// result returns, while the follower pass still owns raw memory.
	for range 2 {
		if result.RefreshPending {
			if err := h.RefreshResultWorldCaches(); err != nil {
				t.Fatal(err)
			}
			result.RefreshPending = false
			refreshes++
		}
	}
	if refreshes != 1 || result.RefreshPending {
		t.Fatal("winning result cache request was not consumed once", refreshes)
	}
	bssAfter, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	if fileFrameHash(bssBefore) != fileFrameHash(bssAfter) || fileFrameHash(codeBefore) != fileFrameHash(nativeRuntimeResultCodeBytes(t, h)) || registers != c.D || h.World.nativeCallDepth != 1 || !h.Session.resultPending || h.Session.Pass.Stage != NativeFrameFollowers {
		t.Fatal("winning cache refresh changed raw state or continuation")
	}
	if h.World.Level.Number != int(worldAfterReset) || h.World.NativeGameMode != 2 || h.World.Custom {
		t.Fatal("winning reset decoded world remains stale", h.World.Level.Number, worldAfterReset)
	}
	created := 0
	for at := 0x76f4; at < 0xc800; at += 52 {
		owner, err := h.Memory.BSS.Read8(at + 12)
		if err != nil {
			t.Fatal(err)
		}
		if owner != 0 {
			created++
		}
	}
	if created == 0 {
		t.Fatal("winning reset returned without creating native followers")
	}
	if len(h.Files.handles) != 0 {
		t.Fatal("winning transition leaked resource handles")
	}
}
