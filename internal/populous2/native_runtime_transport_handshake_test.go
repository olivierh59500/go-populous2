package populous2

import (
	"net"
	"testing"
	"time"
)

func TestNativeRuntimeHandshakeConstructsActualNegotiatedWorlds(t *testing.T) {
	left, right := net.Pipe()
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
		for _, p := range []nativeHeroPatch{{0x15a, 2, 4800}, {0x15e, 2, 380}, {0xeb42, 2, uint32(side + 1)}, {0xeb44, 2, 4}, {0xeb22, 2, 0}, {0xeb24, 4, 0x058028af}, {0xeb6a, 4, 0x20eb56}, {0xeb5e, 1, 2}, {0xeb68, 1, 4}} {
			renderFramePatch(h.Memory.BSS, p)
		}
		conn, err := NewNativeSerialConn(connection)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		index := side
		directors[side] = NativeRuntimeDirector{started: true}
		directors[side].Startup.Startup.Entry = 0x10ad8
		waitSite, waitCount := uint32(0), 0
		transports[side], err = h.NewTransport(conn, NativeTransportFrameCallbacks{WaitCPU: func(site, count uint32) (bool, error) {
			if count != 100000 {
				t.Fatal("native wait iteration count differs")
			}
			if site != waitSite {
				waitSite, waitCount = site, 0
			}
			waitCount++
			return waitCount >= 4, nil
		}, CallTransport: func(call NativeFileFrameCall, _ *uint32) (NativeSerialFrameChildResult, error) {
			if call.Routine != 0x10ad8 {
				t.Fatalf("unexpected handshake source child%x", call.Routine)
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
	// for the second call; the peer's changed state must arrive again.
	for visit := 0; visit < 2; visit++ {
		var menu [2]NativeInGameHostState
		var done [2]bool
		menuRules, err := DecodeNativeInGameHostRules(hosts[0].Bundle.Executable)
		if err != nil {
			t.Fatal(err)
		}
		for side, h := range hosts {
			if err := h.Memory.BSS.Write16(0xe90c, uint16(70+visit*10+side)); err != nil {
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
			if err != nil || got != uint16(70+visit*10+1-side) {
				t.Fatal("menu resume did not exchange the new peer state", side, visit, got, err)
			}
		}
	}
	var gameFrames [2]*NativeRuntimeFrame
	var err error
	for side, h := range hosts {
		side := side
		audio := supplied[side].Audio
		operations := NativeRuntimeAudioOperations{Command: audio.Command, MusicCommand: audio.MusicCommand, DirectCue: devices[side].DirectCue}
		gameFrames[side], err = h.NewFrame(NativeRuntimeFrameBindings{Audio: operations, RenderChildren: NativeRuntimeRenderChildrenCallbacks{Beam: func() (uint16, error) { return 0, nil }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: devices[side].DirectCue}, Menu: NativeInGameHostCallbacks{Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}, Session: NativeFrameSessionCallbacks{Transport: transports[side].PacketCallback}})
		if err != nil {
			t.Fatal(err)
		}
	}
	for tick := 0; tick < 120; tick++ {
		var done [2]bool
		for side, h := range hosts {
			at := 0xeb56 + side*10
			command := uint32(8)
			for _, patch := range []nativeHeroPatch{{at + 1, 1, command}, {at + 2, 1, uint32(20 + side*10 + tick%4)}, {at + 3, 1, uint32(25 + side*8 + tick%3)}} {
				renderFramePatch(h.Memory.BSS, patch)
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
		for _, span := range [][2]int{{0xf44, 0x4000}, {0x5f50, 0x87f0}, {0xeb28, 4}} {
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
					t.Fatalf("paired native gameplay diverged tick%d at%x", tick, span[0]+i)
				}
			}
		}
	}
}
