package populous2

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"
)

// This uses the same TCP endpoint and byte-stream adapter as NativeGame.
// The two native runtimes retain their own clocks, rendering, actor pools,
// serial contexts and delayed commands throughout the complete main frames.
func TestNativeRuntimeTCPPairRunsTwoThousandNativeMainFrames(t *testing.T) {
	for _, unlocked := range []bool{false, true} {
		name := "initial-terrain-and-magnet"
		if unlocked {
			name = "campaign-unlocked-mixed-powers"
		}
		t.Run(name, func(t *testing.T) { nativeRuntimePairMainFrames(t, unlocked) })
	}
}

func nativeRuntimePairMainFrames(t *testing.T, unlocked bool) {
	left, right := nativeRuntimePairTCP(t)
	var hosts [2]*NativeRuntimeHost
	var transports [2]*NativeRuntimeTransport
	var frames [2]NativeFrameRegisterContext
	var directors [2]NativeRuntimeDirector
	var rules [2]NativeStartupCampaignHostRules
	var supplied [2]NativeRuntimeDirectorCallbacks
	var devices [2]*NativeAudioDevice
	for side, connection := range []net.Conn{left, right} {
		h := nativeRuntimeHostTest(t)
		hosts[side] = h
		if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
			t.Fatal(err)
		}
		frames[side] = NativeFrameRegisterContext{AddressBase: 0x200000}
		if done, err := h.AdvanceAllocations(0x1a43e, &frames[side], NativeErrorFrameCallbacks{}); err != nil || !done {
			t.Fatal(done, err)
		}
		device, _, err := h.InitializeAudio(&frames[side], 0)
		if err != nil {
			t.Fatal(err)
		}
		devices[side] = device
		rules[side], err = DecodeNativeStartupCampaignHostRules(h.Bundle.Executable)
		if err != nil {
			t.Fatal(err)
		}
		audio := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, &frames[side])
		supplied[side] = NativeRuntimeDirectorCallbacks{NativeStartupCampaignHostCallbacks: NativeStartupCampaignHostCallbacks{NativeStartupHostFrameCallbacks: NativeStartupHostFrameCallbacks{Audio: &audio, NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }}}, Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }}}
		prelude := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10a10}}
		pre, err := h.StartupCallbacks(&frames[side], NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Hardware: func(NativeFrameHardwareWrite) error { return nil }, Call: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
			if call.Routine != 0x3b64 {
				t.Fatal("unexpected prelude child")
			}
			return NativeCommandFrameResult{}, nil
		}}}, NativeErrorFrameCallbacks{})
		if err != nil {
			t.Fatal(err)
		}
		if step, err := prelude.Advance(&rules[side].Startup, pre); err != nil || step.Complete {
			t.Fatal(step, err)
		}
		if unlocked {
			nativeRuntimePairUnlockCampaignPowers(t, h, &frames[side], &rules[side], device, &supplied[side])
			nativeRuntimePairConfigureDeity(t, h, &frames[side], supplied[side].Campaign)
			if side == 1 {
				// The native side selector swaps the edited profile into God2.
				// Assigning EB42 alone would leave its XP on the other deity.
				panel, err := DecodeNativeProfilePanelFrameRules(h.Bundle.Executable)
				if err != nil {
					t.Fatal(err)
				}
				frames[side].Word(0, 2)
				if _, err := panel.SwitchProfile(h.Memory.BSS, &frames[side]); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, p := range []nativeHeroPatch{{0x15a, 2, 4800}, {0x15e, 2, 380}, {0xeb42, 2, uint32(side + 1)}, {0xeb44, 2, 4}, {0xeb22, 2, 0}, {0xeb24, 4, 0x058028af}, {0xeb6a, 4, 0x20eb56}, {0xeb5e, 1, 2}, {0xeb68, 1, 4}} {
			renderFramePatch(h.Memory.BSS, p)
		}
		profile, pe := h.Memory.BSS.Read16(0xeb42)
		xp, xe := h.Memory.BSS.Read8(commandGod(uint16(side+1)) + 0x53)
		t.Logf("configured side%d profile%d XP%d D=%x errors=%v/%v", side, profile, xp, frames[side].D, pe, xe)
		conn, err := NewNativeSerialConn(connection)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		index := side
		directors[side] = NativeRuntimeDirector{started: true}
		directors[side].Startup.Startup.Entry = 0x10ad8
		waitSite := uint32(0)
		var waitStart time.Time
		transports[side], err = h.NewTransport(conn, NativeTransportFrameCallbacks{WaitCPU: func(site, count uint32) (bool, error) {
			if count != 100000 {
				t.Fatal("native wait iteration count differs")
			}
			if site != waitSite {
				waitSite, waitStart = site, time.Now()
			}
			// Match NativeGame's configured portable delay for the original
			// 100000-iteration spin; do not race peer TCP scheduling.
			return time.Since(waitStart) >= 250*time.Millisecond, nil
		}, CallTransport: func(call NativeFileFrameCall, _ *uint32) (NativeSerialFrameChildResult, error) {
			if call.Routine != 0x10ad8 {
				local, _ := hosts[index].Memory.BSS.Read16(0xeb42)
				peer, _ := hosts[index].Memory.Code.Read8(0x182c1)
				t.Fatalf("unexpected handshake source child%x: side%d PC%x dialog%d message%x localprofile%d received%d failure=%v", call.Routine, index, transports[index].Handshake.PC, transports[index].Handshake.DialogPhase, transports[index].Handshake.DialogMessage, local, peer, transports[index].Handshake.Failure)

			}
			step, err := directors[index].Advance(hosts[index], &rules[index], call.Frame, supplied[index])
			return NativeSerialFrameChildResult{Complete: step.Complete, Zero: step.Zero, Negative: step.Negative}, err
		}}, func(bool, *NativeFrameRegisterContext) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for !transports[0].Handshake.Finished || !transports[1].Handshake.Finished {
		if time.Now().After(deadline) {
			t.Fatalf("actual runtime handshake stalled%x/%x", transports[0].Handshake.PC, transports[1].Handshake.PC)
		}
		for side, h := range hosts {
			p := &h.Session.Presentation.Input
			_, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &frames[side])
			if err != nil {
				t.Fatal(err)
			}
			step, err := transports[side].AdvanceHandshake(&frames[side])
			if err != nil {
				t.Fatal(err)
			}
			if step.Complete && !step.FlagsKnown {
				t.Fatal("handshake omitted true constructor condition flags")
			}
		}
		time.Sleep(100 * time.Microsecond)
	}
	for side, h := range hosts {
		mode, err := h.Memory.BSS.Read16(0xeb44)
		if err != nil || mode != 6 || !directors[side].ready {
			t.Fatal("actual negotiated world constructor did not complete", mode, err)
		}
	}
	for _, span := range [][2]int{{0xf44, 0x4000}, {0x4f44, 0x1000}, {0x5f50, 0x87f0}} {
		for i := 0; i < span[1]; i++ {
			a, err := hosts[0].Memory.BSS.Read8(span[0] + i)
			if err != nil {
				t.Fatal(err)
			}
			b, err := hosts[1].Memory.BSS.Read8(span[0] + i)
			if err != nil {
				t.Fatal(err)
			}
			if a != b {
				t.Fatalf("negotiated world bytes diverged%x", span[0]+i)
			}
		}
	}
	// Two independent menu visits must each exchange the actual resume
	// packet. A completed controller from the first visit cannot stand in
	// for the second call; preserve the real constructor statistics.
	for visit := 0; visit < 2; visit++ {
		var menu [2]NativeInGameHostState
		var done [2]bool
		menuRules, err := DecodeNativeInGameHostRules(hosts[0].Bundle.Executable)
		if err != nil {
			t.Fatal(err)
		}
		var resumeWords [2]uint16
		for side, h := range hosts {
			resumeWords[side], err = h.Memory.BSS.Read16(0xe90c)
			if err != nil {
				t.Fatal(err)
			}
		}
		deadline = time.Now().Add(time.Second)
		for !done[0] || !done[1] {
			if time.Now().After(deadline) {
				t.Fatalf("actual menu resume visit%d stalled", visit)
			}
			for side := range hosts {
				if done[side] {
					continue
				}
				result, err := menu[side].AdvanceChild(&menuRules, NativeInGameHostCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Call: transports[side].ResumeMenuChild}}, NativeFileFrameCall{Routine: 0x181c0, Frame: &frames[side]}, &menu[side].Menu.ChildPhase)
				if err != nil {
					t.Fatal(err)
				}
				if result.Complete && !result.Zero {
					t.Fatal("menu lost the actual resume condition flags")
				}
				done[side] = result.Complete
			}
			time.Sleep(100 * time.Microsecond)
		}
		for side, h := range hosts {
			got, err := h.Memory.BSS.Read16(0xe90c)
			if err != nil || got != resumeWords[1-side] {
				t.Fatal("menu resume did not exchange the new peer state", side, visit, got, err)
			}
		}
	}
	var gameFrames [2]*NativeRuntimeFrame
	var results [2]NativeRuntimeResultHost
	var executed, debited [2]map[uint8]int
	var paintedRoads, createdWalls [2]int
	var progressions [2]NativeRuntimeProgressionState
	var deities [2]NativeRuntimeDeity
	var err error
	for side, h := range hosts {
		side := side
		audio := supplied[side].Audio
		operations := NativeRuntimeAudioOperations{Command: audio.Command, MusicCommand: audio.MusicCommand, DirectCue: devices[side].DirectCue}
		results[side] = NativeRuntimeResultHost{Rules: rules[side], Startup: supplied[side], Callbacks: NativeRuntimeResultCallbacks{Audio: operations, Sound: devices[side].DirectCue, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}}
		progression := NativeRuntimeProgressionCallbacks{Audio: operations, Ownership: supplied[side].Ownership, Child: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
			if call.Routine == 0xb740 {
				return deities[side].AdvanceChild(h, call, phase, supplied[side].Campaign)
			}
			err := RunNativeProgressionChild(call.Routine, h, call.Frame, call.A)
			return NativeCommandFrameResult{Complete: err == nil}, err
		}}
		results[side].Progression = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
			return progressions[side].AdvanceChild(h, call, phase, progression)
		}
		resultAdvance := func(identity uint16, c *NativeFrameRegisterContext) (bool, error) {
			complete, err := results[side].Advance(h, identity, c)
			if complete && err == nil && results[side].RefreshPending {
				err = h.RefreshResultWorldCaches()
				if err == nil {
					results[side].RefreshPending = false
				}
			}
			return complete, err
		}
		gameFrames[side], err = h.NewFrame(NativeRuntimeFrameBindings{Audio: operations, RenderChildren: NativeRuntimeRenderChildrenCallbacks{Beam: func() (uint16, error) { return 0, nil }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: devices[side].DirectCue}, Menu: NativeInGameHostCallbacks{Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}, InputChildren: NativeGameplayHUDHostCallbacks{Campaign: supplied[side].Campaign, Ownership: supplied[side].Ownership, Audio: operations}, Session: NativeFrameSessionCallbacks{Transport: transports[side].PacketCallback, ResultAdvance: resultAdvance}})
		if err != nil {
			t.Fatal(err)
		}
		executed[side], debited[side] = map[uint8]int{}, map[uint8]int{}
		bindings := gameFrames[side].Callbacks.Commands
		bindings.WallRules, bindings.WallPlacement = &h.Session.wallRules, &h.Session.wallPlacement
		actual := h.Session.commandExecutor(gameFrames[side].Callbacks, bindings)
		gameFrames[side].Callbacks.Execute = func(at int, context *NativeCommandRegisterContext, phase *uint32) (bool, error) {
			command, err := h.Memory.BSS.Read8(at + 1)
			if err != nil {
				return false, err
			}
			x, err := h.Memory.BSS.Read8(at + 2)
			if err != nil {
				return false, err
			}
			y, err := h.Memory.BSS.Read8(at + 3)
			if err != nil {
				return false, err
			}
			grid := 0xf44 + int(y)*256 + int(x)*4
			before, err := h.Memory.BSS.Read8(grid + 1)
			if err != nil {
				return false, err
			}
			wallsBefore := nativeRuntimePairedWallCount(t, h)
			done, err := actual(at, context, phase)
			if err != nil || !done {
				return done, err
			}
			step := h.Session.commandStates[(at-0xeb56)/10].Step
			if command != 0 {
				executed[side][command]++
				if step.Debited {
					debited[side][command]++
				}
			}
			if command == 42 {
				after, err := h.Memory.BSS.Read8(grid + 1)
				if err != nil {
					return false, err
				}
				property, err := h.Memory.Code.Read16(0x33312 + int(after)*2)
				if err != nil {
					return false, err
				}
				if after != before && property&0x40 != 0 {
					paintedRoads[side]++
				}
			}
			if command == 34 && nativeRuntimePairedWallCount(t, h) > wallsBefore {
				createdWalls[side]++
			}
			return true, nil
		}
	}
	commandRules, err := DecodeNativeGameplayHUDInputRules(hosts[0].Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	issued := [2]map[uint8]int{{}, {}}
	unavailable := [2]map[uint8]int{{}, {}}
	unaffordable := [2]map[uint8]int{{}, {}}
	var maxMana [2]uint32
	viewPermissionDifferences := 0
	availabilityDifferences := 0
	frameCount := 2048
	if unlocked {
		frameCount = 4096
	}
	for tick := 0; tick < frameCount; tick++ {
		var done [2]bool
		for side, h := range hosts {
			at := 0xeb56 + side*10
			mana, err := h.Memory.BSS.Read32(commandGod(uint16(side + 1)))
			if err != nil {
				t.Fatal(err)
			}
			maxMana[side] = max(maxMana[side], mana)
			command := uint8(0)
			if tick%32 == 0 {
				sequence := [...]uint8{2, 42, 34, 46, 8}
				candidate := sequence[(tick/32+side)%len(sequence)]
				if unlocked && issued[side][42] == 0 {
					candidate = 42
				}
				price, err := commandRules.Commands.word(0x210b0 + int(candidate))
				if err != nil {
					t.Fatal(err)
				}
				power := price / 2
				available, err := h.Memory.BSS.Read8(commandGod(uint16(side+1)) + 0x70 + int(power))
				if err != nil {
					t.Fatal(err)
				}
				if int8(available) > 0 {
					caller := frames[side]
					caller.Word(2, uint16(candidate))
					caller.Word(3, uint16(side+1))
					var a [7]NativeRequesterAddress
					allowed, err := NativeGameplayHUDAdmission(&commandRules, NativeGameplayHUDInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: &caller, Memory: h.Memory.BSS, Code: h.Memory.Code, CodeBase: h.Memory.CodeBase}}, &a)
					if err != nil {
						t.Fatal(err)
					}
					// Retain the real admission outputs in this configured
					// producer's incoming context; never reset D4/D5/D7.
					frames[side] = caller
					if allowed {
						command = candidate
					} else {
						unaffordable[side][candidate]++
					}
				} else {
					unavailable[side][candidate]++
				}
			}
			for _, patch := range []nativeHeroPatch{{at + 1, 1, uint32(command)}, {at + 2, 1, uint32(20 + side*10 + tick%4)}, {at + 3, 1, uint32(25 + side*8 + tick%3)}} {
				renderFramePatch(h.Memory.BSS, patch)
			}
			if command != 0 {
				issued[side][command]++
			}
			if err := h.Session.BeginRaw(h.World, frames[side]); err != nil {
				t.Fatal(err)
			}
			p := &h.Session.Presentation.Input
			if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &frames[side]); err != nil {
				t.Fatal(err)
			}
		}
		deadline = time.Now().Add(time.Second)
		for !done[0] || !done[1] {
			if time.Now().After(deadline) {
				t.Fatalf("paired native gameplay frame%d stalled%d/%d", tick, hosts[0].Session.Phase, hosts[1].Session.Phase)
			}
			for side := range hosts {
				if done[side] {
					continue
				}
				if hosts[side].Session.Phase == NativeFrameSessionRender && gameFrames[side].RenderChildren.actorActive {
					nativeGameplayEnterProtectionAnswer(t, hosts[side], gameFrames[side])
				}
				if hosts[side].Session.Phase == NativeFrameSessionClock {
					p := &hosts[side].Session.Presentation.Input
					if _, err := hosts[side].Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, hosts[side].Memory.BSS, &frames[side]); err != nil {
						t.Fatal(err)
					}
				}
				done[side], err = gameFrames[side].Advance()
				if err != nil {
					t.Fatalf("paired native frame%d side%d:%v", tick, side, err)
				}
				if done[side] {
					frames[side] = hosts[side].Session.Frame
				}
			}
			time.Sleep(100 * time.Microsecond)
		}
		for _, span := range [][2]int{{0xf44, 0x4000}, {0x4f44, 0x1000}, {0x5f50, 0x87f0}, {0xe8a4, 628}, {0xeb28, 4}, {0xf40, 4}} {
			for i := 0; i < span[1]; i++ {
				a, err := hosts[0].Memory.BSS.Read8(span[0] + i)
				if err != nil {
					t.Fatal(err)
				}
				b, err := hosts[1].Memory.BSS.Read8(span[0] + i)
				if err != nil {
					t.Fatal(err)
				}
				if a != b {
					at := span[0] + i
					// Original E532/E624/town rendering sets these low two
					// permission bits from each peer's own visible actors.
					// Compare every gameplay rule bit; retain both raw bytes.
					if (at == 0xe8ef || at == 0xea29) && a&^3 == b&^3 {
						viewPermissionDifferences++
						continue
					}
					if !unlocked {
						wa, knownA := nativeRuntimePairInitialPolicyByte(t, hosts[0], 0, at)
						wb, knownB := nativeRuntimePairInitialPolicyByte(t, hosts[1], 1, at)
						if knownA && knownB {
							if a != wa || b != wb {
								t.Fatalf("native selected-profile availability/policy at%x differs: got%x/%x want%x/%x", at, a, b, wa, wb)
							}
							availabilityDifferences++
							continue
						}
					}
					g0, _ := hosts[0].Memory.BSS.Read16(0xeb2c)
					g1, _ := hosts[1].Memory.BSS.Read16(0xeb2c)
					t.Fatalf("paired native gameplay diverged tick%d at%x values %x/%x EB2C %x/%x", tick, span[0]+i, a, b, g0, g1)
				}
			}
		}
	}
	t.Logf("completed%d native frames; view-permission byte differences=%d; source availability differences=%d; executed=%v debited=%v road-paints=%v wall-creations=%v", frameCount, viewPermissionDifferences, availabilityDifferences, executed, debited, paintedRoads, createdWalls)
	if unlocked && (paintedRoads[0] == 0 || paintedRoads[1] == 0 || createdWalls[0] == 0 || createdWalls[1] == 0) {
		t.Fatal("mixed native commands did not create actual roads and walls")
	}
	for host := range hosts {
		for _, command := range []uint8{2, 8} {
			if executed[host][command] == 0 {
				t.Fatal("native command executor did not consume the source record", host, command)
			}
		}
	}
	for host, h := range hosts {
		if results[host].Result != nil || results[host].Reset != nil || results[host].RefreshPending {
			t.Fatal("healthy paired play retained a result/reset operation", host)
		}
		var living [2]int
		for at := 0x76f4; at < 0xc800; at += 52 {
			owner, err := h.Memory.BSS.Read8(at + 12)
			if err != nil {
				t.Fatal(err)
			}
			population, err := h.Memory.BSS.Read32(at + 26)
			if err != nil {
				t.Fatal(err)
			}
			if owner >= 1 && owner <= 2 && int32(population) > 0 {
				living[owner-1]++
			}
		}
		if living[0] == 0 || living[1] == 0 {
			t.Fatal("paired runtime did not retain both living populations", host, living)
		}
		t.Logf("host%d living follower records=%v", host, living)
	}
	for side, commands := range issued {
		t.Logf("side%d source commands: admitted=%v unavailable=%v unaffordable=%v maximum mana=%d", side+1, commands, unavailable[side], unaffordable[side], maxMana[side])
		required := []uint8{2, 8}
		if unlocked {
			required = []uint8{2, 42, 34}
		}
		for _, command := range required {
			if commands[command] == 0 {
				t.Fatal("paired main did not admit required terrain/road/wall command", side+1, command)
			}
		}
	}
}

func nativeRuntimePairTCP(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	server, err := ListenNativeNetwork("127.0.0.1:0")
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skip("sandbox denied loopback bind; approved local socket run is required")
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	client, err := DialNativeNetwork(context.Background(), server.Address())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		left, a, ae := server.Poll()
		right, b, be := client.Poll()
		if ae != nil || be != nil {
			t.Fatal(ae, be)
		}
		if a && b {
			return left, right
		}
		if time.Now().After(deadline) {
			t.Fatal("native desktop TCP endpoints remained pending")
		}
		time.Sleep(100 * time.Microsecond)
	}
}

// Use source campaign-code entry and the custom constructor's actual 11078
// CONQUEST union. No God power flags, experience, mana or pools are patched.
func nativeRuntimePairUnlockCampaignPowers(t *testing.T, h *NativeRuntimeHost, c *NativeFrameRegisterContext, rules *NativeStartupCampaignHostRules, device *NativeAudioDevice, supplied *NativeRuntimeDirectorCallbacks) {
	t.Helper()
	control := NativeAudioControlDeviceCallbacks(device, h.Memory.BSS, c)
	supplied.Campaign = NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{AudioCommand: device.Command, AudioControl: control, NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}}}
	var director *NativeRuntimeDirector
	advance := func() bool {
		step, err := director.Advance(h, rules, c, *supplied)
		if err != nil {
			t.Fatal(err)
		}
		return step.Complete
	}
	blank := func() {
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, c); err != nil {
			t.Fatal(err)
		}
	}
	fade := func() {
		for range 18 {
			blank()
			if advance() {
				t.Fatal("native configuration requester returned during fade")
			}
		}
	}
	var err error
	director, err = NewNativeRuntimeResetDirector(0x10a8c)
	if err != nil {
		t.Fatal(err)
	}
	if advance() || director.Menu == nil {
		t.Fatal("native configuration menu missing")
	}
	fade()
	nativeRuntimeClickAction(t, h, c, 4)
	for i := 0; i < 40 && director.Selection == nil; i++ {
		if advance() {
			t.Fatal("campaign setup skipped actual chooser")
		}
		blank()
	}
	if director.Selection == nil {
		t.Fatal("campaign configuration chooser missing")
	}
	fade()
	nativeRuntimeClickAction(t, h, c, 2)
	if advance() || director.Selection == nil || !director.Selection.Modal.Active {
		t.Fatal("campaign world-code modal missing")
	}
	// The text comes from the independently proven original 103C6 table.
	worldRules, err := DecodeNativeInGameRequesterRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	text := worldRules.NativeWorldCode(999)
	keys, err := DecodeNativeInputRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range append(text, 13) {
		raw := -1
		for i, v := range keys.Keys[:128] {
			if v == value {
				raw = i
				break
			}
		}
		if raw < 0 {
			t.Fatalf("native world-code keyboard character %d unavailable", value)
		}
		wire, err := NativeKeyWire(uint8(raw), true)
		if err != nil {
			t.Fatal(err)
		}
		if err = h.Session.Presentation.Input.KeyboardInterrupt(wire); err != nil {
			t.Fatal(err)
		}
		blank()
		if advance() {
			t.Fatal("world-code input completed constructor prematurely")
		}
		if err = h.Session.Presentation.Input.KeyboardInterrupt(wire &^ 1); err != nil {
			t.Fatal(err)
		}
		blank()
		if advance() {
			t.Fatal("world-code release completed constructor prematurely")
		}
	}
	fade()
	if world, err := h.Memory.BSS.Read16(0xeb46); err != nil || world != 999 {
		t.Fatal("native world code did not select world999", world, err)
	}
	nativeRuntimeClickAction(t, h, c, 6)
	done := false
	for i := 0; i < 40 && !done; i++ {
		done = advance()
		if !done {
			blank()
		}
	}
	if !done {
		t.Fatal("actual campaign world construction remained pending")
	}
	director, err = NewNativeRuntimeResetDirector(0x10a8c)
	if err != nil {
		t.Fatal(err)
	}
	if advance() || director.Menu == nil {
		t.Fatal("custom configuration menu missing")
	}
	fade()
	nativeRuntimeClickAction(t, h, c, 6)
	done = false
	for i := 0; i < 40 && !done; i++ {
		done = advance()
		if !done {
			blank()
		}
	}
	if !done {
		t.Fatal("actual custom constructor did not unlock campaign powers")
	}
	if err = h.RefreshWorldCaches(); err != nil {
		t.Fatal(err)
	}
}

// This original CPU password fixture contains native category1 experience224. Enter it
// through B740/4BBA so the native decoder, profile writes and price table own
// the resulting discount; no raw experience or mana bytes are substituted.
func nativeRuntimePairConfigureDeity(t *testing.T, h *NativeRuntimeHost, c *NativeFrameRegisterContext, supplied NativeCampaignSelectionChildrenCallbacks) {
	t.Helper()
	var deity NativeRuntimeDeity
	var a [7]NativeRequesterAddress
	var phase uint32
	advance := func() bool {
		step, err := deity.AdvanceChild(h, NativeStartupResetFrameCall{Routine: 0xb740, Frame: c, A: &a}, &phase, supplied)
		if err != nil {
			t.Fatal(err)
		}
		return step.Complete
	}
	blank := func() {
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, c); err != nil {
			t.Fatal(err)
		}
	}
	if advance() {
		t.Fatal("deity configuration skipped the requester")
	}
	for range 40 {
		blank()
		if advance() {
			t.Fatal("deity configuration exited without input")
		}
	}
	nativeRuntimeClickAction(t, h, c, 64)
	if advance() || deity.State == nil || !deity.State.Modal.Active {
		t.Fatal("real deity password modal missing")
	}
	keys, err := DecodeNativeInputRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []byte("KIADMCWLDIDOKOXP\r") {
		raw := -1
		for i, v := range keys.Keys[:128] {
			if v == value {
				raw = i
				break
			}
		}
		if raw < 0 {
			t.Fatal("deity keyboard character missing", value)
		}
		wire, err := NativeKeyWire(uint8(raw), true)
		if err != nil {
			t.Fatal(err)
		}
		if err = h.Session.Presentation.Input.KeyboardInterrupt(wire); err != nil {
			t.Fatal(err)
		}
		blank()
		if advance() {
			t.Fatal("deity password completed editor prematurely")
		}
		if err = h.Session.Presentation.Input.KeyboardInterrupt(wire &^ 1); err != nil {
			t.Fatal(err)
		}
		blank()
		if advance() {
			t.Fatal("deity password key release completed editor prematurely")
		}
	}
	for range 4 {
		blank()
		if advance() {
			t.Fatal("deity decoder exited without proceed")
		}
	}
	if xp, err := h.Memory.BSS.Read8(0xe8a4 + 0x53); err != nil || xp != 224 {
		t.Fatal("actual deity decoder did not install the original category1 experience", xp, err)
	}
	nativeRuntimeClickAction(t, h, c, 66)
	done := false
	for i := 0; i < 40 && !done; i++ {
		done = advance()
		if !done {
			blank()
		}
	}
	if !done {
		t.Fatal("actual deity configuration did not return")
	}
}

func nativeRuntimePairedWallCount(t *testing.T, h *NativeRuntimeHost) int {
	t.Helper()
	count := 0
	for at := 0x5f50; at < 0x6bd0; at += 16 {
		owner, err := h.Memory.BSS.Read8(at + 12)
		if err != nil {
			t.Fatal(err)
		}
		if owner != 0 {
			count++
		}
	}
	return count
}

// The original 11078/10DF2/10E90 CPU corpus proves these selected-profile
// fields. Compute their world0 values directly from immutable source bytes:
// power flags, the two count words and the compacted 32 four-byte choices.
// Simulation state and every other deity field remain strictly compared.
func nativeRuntimePairInitialPolicyByte(t *testing.T, h *NativeRuntimeHost, selected int, at int) (byte, bool) {
	t.Helper()
	god := -1
	offset := 0
	for index := 0; index < 2; index++ {
		base := 0xe8a4 + index*314
		if at >= base && at < base+314 {
			god, offset = index, at-base
		}
	}
	if god < 0 {
		return 0, false
	}
	code := h.Bundle.Executable.Hunks[0].Data
	var flags [36]byte
	for power := range flags {
		flags[power] = code[0x20646+god*58+power] | h.Bundle.Levels[0].Raw[selected*58+22+power]
	}
	if offset >= 0x70 && offset < 0x94 {
		return flags[offset-0x70], true
	}
	if !(offset >= 0x94 && offset < 0x98 || offset >= 0x9c && offset < 0x11c) {
		return 0, false
	}
	var expected [0x11c]byte
	out := 0x9c
	for bank, span := range [][2]int{{0x20768, 0x207c8}, {0x207c8, 0x207e8}} {
		count := uint16(1 - bank)
		for source := span[0]; source < span[1]; source += 4 {
			command := binary.BigEndian.Uint16(code[source:])
			power := binary.BigEndian.Uint16(code[0x210b0+int(command):]) >> 1
			if power >= uint16(len(flags)) {
				t.Fatal("native initial policy source alias outside power flags", power)
			}
			if int8(flags[power]) > 0 {
				count++
				copy(expected[out:out+4], code[source:source+4])
				out += 4
			}
		}
		binary.BigEndian.PutUint16(expected[0x94+bank*2:], count)
	}
	return expected[offset], true
}
