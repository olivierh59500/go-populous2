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
	deadline = time.Now().Add(time.Second)
	for !transports[0].Resume.Finished || !transports[1].Resume.Finished {
		if time.Now().After(deadline) {
			t.Fatal("negotiated actual runtime resume stalled")
		}
		for side := range transports {
			step, err := transports[side].AdvanceResume(&frames[side])
			if err != nil {
				t.Fatal(err)
			}
			if step.CorruptReturn {
				t.Fatal("healthy peer stream entered corrupt source return")
			}
		}
		time.Sleep(100 * time.Microsecond)
	}
}
