package populous2

import (
	"bytes"
	"testing"
)

func TestNativeRuntimeRecreationProtectionPolicyOnlyValidatesSession(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	children, err := h.NewRenderChildren(NativeRuntimeRenderChildrenCallbacks{SkipCopyProtection: true})
	if err != nil {
		t.Fatal(err)
	}
	before, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	c := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase, D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	saved := c.D
	for range 2 {
		done, err := children.AdvanceProtection(0x76f4, &c)
		if err != nil || !done || c.D != saved {
			t.Fatal("recreation policy suspended or changed caller", done, c.D, err)
		}
	}
	after, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	before[0x3b8], before[0x3b9] = 0, 1
	if !bytes.Equal(before, after) || children.Protection.Started || children.actorActive {
		t.Fatal("copy-protection policy changed unrelated game state or entered modal")
	}
}

func TestNativeRuntimeRecreationStartsAndContinuesPastFirstTown(t *testing.T) {
	h, device, c := nativeGameplayIntegrationStartup(t, 0, 4311)
	if err := h.Memory.BSS.Write16(0xeb44, 8); err != nil {
		t.Fatal(err)
	}
	if err := h.RefreshWorldCaches(); err != nil {
		t.Fatal(err)
	}
	frame, err := h.NewFrame(NativeRuntimeFrameBindings{Audio: NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}, RenderChildren: NativeRuntimeRenderChildrenCallbacks{SkipCopyProtection: true, Beam: func() (uint16, error) { return 0, nil }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: device.DirectCue}, InputChildren: NativeGameplayHUDHostCallbacks{Audio: NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}}, Menu: NativeInGameHostCallbacks{Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}})
	if err != nil {
		t.Fatal(err)
	}
	completed := 0
	var cadence NativeHostCadence
	irqBefore := h.Session.Presentation.Input.long(0x16)
	for tick := uint64(0); tick < 1000; tick++ {
		// Input and interrupt clocks continue even when no new gameplay
		// pass is admitted on this host update.
		registers := &h.Session.Frame
		if h.Session.Phase == NativeFrameSessionIdle {
			registers = &c
		}
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, registers); err != nil {
			t.Fatal(err)
		}
		if h.Session.Phase == NativeFrameSessionIdle {
			if !cadence.Ready(tick, NativeHostGameplayPeriod) {
				continue
			}
			if err := h.Session.BeginRaw(h.World, c); err != nil {
				t.Fatal(err)
			}
		}
		done, err := frame.Advance()
		if err != nil {
			t.Fatal(tick, err)
		}
		if done {
			c = h.Session.Frame
			completed++
		}
		if frame.RenderChildren.Protection.Started {
			t.Fatal("ordinary recreation entered the manual statue challenge")
		}
	}
	if h.Session.Phase != NativeFrameSessionIdle {
		h.Session.finish(nil)
	}
	if completed < 100 {
		t.Fatal("paced runtime did not keep advancing", completed)
	}
	if irqs := h.Session.Presentation.Input.long(0x16) - irqBefore; irqs != 1000 {
		t.Fatal("gameplay pacing skipped PAL interrupts", irqs)
	}
	if flag, err := h.Memory.BSS.Read16(0x3b8); err != nil || flag != 1 {
		t.Fatal("first town did not validate the recreation session", flag, err)
	}
}
