package populous2

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeRuntimePairedGAMContinuesFullMainOverTCP(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(fmt.Sprintf("pending-%v", async), func(t *testing.T) { nativeRuntimePairedGAM(t, async) })
	}
}

func nativeRuntimePairedGAM(t *testing.T, async bool) {
	p := nativeRuntimeGAMPairStartup(t)
	var frames [2]*NativeRuntimeFrame
	var browsers [2]NativeRuntimeFileBrowserState
	var stores [2]*NativeRuntimeFileStore
	var roots [2]string
	var fileRules [2]NativeRuntimeFileBrowserRules
	var refreshes [2]int
	for side, h := range p.Hosts {
		side := side
		device := p.Devices[side]
		operations := NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}
		var err error
		roots[side] = t.TempDir()
		stores[side], err = NewNativeRuntimeFileStore(roots[side], []string{"SAVES", "RAM", "DF0", "DF1"}, async)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stores[side].Close() })
		fileRules[side], err = DecodeNativeRuntimeFileBrowserRules(h.Bundle.Executable)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []struct {
			at   int
			text string
		}{{0x4440, "SAVES:"}, {0x4416, "PAIR"}} {
			for i, v := range append([]byte(field.text), 0) {
				if err = h.Memory.Code.Write8(field.at+i, v); err != nil {
					t.Fatal(err)
				}
			}
		}
		frames[side], err = h.NewFrame(NativeRuntimeFrameBindings{
			Audio:          operations,
			RenderChildren: NativeRuntimeRenderChildrenCallbacks{Beam: func() (uint16, error) { return 0, nil }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: device.DirectCue},
			InputChildren:  NativeGameplayHUDHostCallbacks{Campaign: p.Startup[side].Campaign, Ownership: p.Startup[side].Ownership, Audio: operations},
			Menu:           NativeInGameHostCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue, Call: p.Transports[side].ResumeMenuChild}, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }},
			Session: NativeFrameSessionCallbacks{Transport: p.Transports[side].PacketCallback, CommandChild: func(call NativeCommandFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
				if call.Routine != 0x3f92 {
					return h.Session.AdvanceBuiltInCommandChild(call, phase)
				}
				c := NativeFrameRegisterContext{D: call.Context.D, AddressBase: h.Memory.BSSBase}
				var a [7]NativeRequesterAddress
				result, err := browsers[side].AdvanceChild(h, stores[side], &fileRules[side], NativeStartupResetFrameCall{Routine: 0x3f92, Frame: &c, A: &a}, phase, NativeRuntimeFileBrowserCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}, Ownership: p.Startup[side].Ownership})
				call.Context.D = c.D
				return result, err
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	run := func(label string, menu [2]int) {
		t.Helper()
		var done, menuClicked, fileClicked [2]bool
		for side, h := range p.Hosts {
			if err := h.Session.BeginRaw(h.World, p.Registers[side]); err != nil {
				t.Fatal(err)
			}
		}
		deadline := time.Now().Add(5 * time.Second)
		for !done[0] || !done[1] {
			if time.Now().After(deadline) {
				t.Fatalf("%s paired source wait stalled: phases%d/%d menu%x/%x", label, p.Hosts[0].Session.Phase, p.Hosts[1].Session.Phase, frames[0].MenuState.Menu.PC, frames[1].MenuState.Menu.PC)
			}
			for side, h := range p.Hosts {
				if done[side] {
					continue
				}
				if h.Session.Phase == NativeFrameSessionMenu && frames[side].MenuState.Menu.PC == 0x45a4 && menu[side] != 0 && !menuClicked[side] {
					nativeRuntimeClickAction(t, h, &h.Session.Frame, menu[side])
					menuClicked[side] = true
				}
				if b := browsers[side].Browser; b != nil && (b.PC == 0x4082 || b.PC == 0x4088) && !fileClicked[side] {
					runtimeFilesClick(t, h, &h.Session.Frame, 34)
					fileClicked[side] = true
				}
				if h.Session.Phase == NativeFrameSessionRender && frames[side].RenderChildren.actorActive {
					nativeGameplayEnterProtectionAnswer(t, h, frames[side])
				}
				input := &h.Session.Presentation.Input
				if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(input.Mouse.CounterX), CounterY: uint8(input.Mouse.CounterY)}, h.Memory.BSS, &h.Session.Frame); err != nil {
					t.Fatal(err)
				}
				var err error
				done[side], err = frames[side].Advance()
				if err != nil {
					t.Fatalf("%s peer%d: %v", label, side, err)
				}
				if done[side] {
					p.Registers[side] = h.Session.Frame
					if browsers[side].Files.RefreshPending {
						before, err := h.Memory.SnapshotBSS()
						if err != nil {
							t.Fatal(err)
						}
						if err = browsers[side].Files.RefreshLoadedViews(h); err != nil {
							t.Fatal(err)
						}
						after, err := h.Memory.SnapshotBSS()
						if err != nil {
							t.Fatal(err)
						}
						if !bytes.Equal(before, after) {
							t.Fatal("loaded cache refresh mutated source BSS")
						}
						refreshes[side]++
					}
				}
			}
			time.Sleep(100 * time.Microsecond)
		}
	}
	queueMenu := func() {
		if err := p.Hosts[0].Memory.BSS.Write8(0xeb57, 118); err != nil {
			t.Fatal(err)
		}
		run("menu-command", [2]int{})
	}
	for tick := 0; tick < 4; tick++ {
		run("before-save", [2]int{})
	}
	queueMenu()
	run("actual-save-menu-and-browser", [2]int{10, 24})
	var saved [2][]byte
	for side, h := range p.Hosts {
		var err error
		saved[side], err = os.ReadFile(filepath.Join(roots[side], "PAIR.GAM"))
		if err != nil || len(saved[side]) != NativeGAMSize {
			t.Fatal("actual paired browser did not save native GAM", side, len(saved[side]), err)
		}
		raw, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(saved[side], raw[NativeGAMStart:NativeGAMEnd]) {
			t.Fatal("paired source save regenerated its native GAM bytes", side)
		}
	}
	for tick := 0; tick < 3; tick++ {
		run("after-save", [2]int{})
	}
	for side, h := range p.Hosts {
		raw, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(saved[side], raw[NativeGAMStart:NativeGAMEnd]) {
			t.Fatal("native main did not progress before loading", side)
		}
	}
	var liveConnections [2]*NativeSerialConn
	var liveControls [2][26]byte
	for side, h := range p.Hosts {
		liveConnections[side] = p.Transports[side].Conn
		for i := range liveControls[side] {
			v, err := h.Memory.BSS.Read8(0xeb56 + i)
			if err != nil {
				t.Fatal(err)
			}
			liveControls[side][i] = v
		}
	}
	queueMenu()
	run("actual-load-menu-and-browser", [2]int{8, 24})
	if refreshes != [2]int{1, 1} {
		t.Fatal("paired native load was not refreshed once after full-frame return", refreshes)
	}
	for side, h := range p.Hosts {
		profile, err := h.Memory.BSS.Read16(0xeb42)
		if err != nil || profile != uint16(side+1) {
			t.Fatal("native paired load changed its saved selected profile", side, profile, err)
		}
		mode, err := h.Memory.BSS.Read16(0xeb44)
		if err != nil || mode != 6 {
			t.Fatal("paired load did not retain source mode6", side, mode, err)
		}
		raw, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(saved[side], raw[NativeGAMStart:NativeGAMEnd]) {
			for i, value := range saved[side] {
				if value != raw[NativeGAMStart+i] {
					t.Fatalf("paired source load did not restore native GAM byte%x: got%x want%x", NativeGAMStart+i, raw[NativeGAMStart+i], value)
				}
			}
		}
		if p.Transports[side].Conn != liveConnections[side] {
			t.Fatal("native load replaced its unsaved live serial owner", side)
		}
		for _, offset := range []int{0, 8, 10, 18, 20, 21, 22, 23} {
			value, err := h.Memory.BSS.Read8(0xeb56 + offset)
			if err != nil || value != liveControls[side][offset] {
				t.Fatal("native load rewrote unsaved source identity/control/pointer bytes", side, offset, value, err)
			}
		}
		if p.Transports[side].Conn.TerminalError() != nil || browsers[side].Browser != nil || len(stores[side].DOS.handles) != 0 {
			t.Fatal("paired load retained an open operation or lost its actual TCP stream", side)
		}
	}
	var loadedClock, loadedRNG [2]uint32
	var loadedActors [2][]byte
	for side, h := range p.Hosts {
		var err error
		loadedClock[side], err = h.Memory.BSS.Read32(0xf40)
		if err != nil {
			t.Fatal(err)
		}
		loadedRNG[side], err = h.Memory.BSS.Read32(0xeb28)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		loadedActors[side] = append([]byte(nil), raw[0x5f50:0xe740]...)
	}
	for tick := 0; tick < 20; tick++ {
		for side, h := range p.Hosts {
			if pause, err := h.Memory.BSS.Read16(0xf3c); err != nil || pause != 0 {
				t.Fatal("loaded main was paused", side, pause, err)
			}
		}
		run("loaded-full-main", [2]int{})
		for side, h := range p.Hosts {
			clock, err := h.Memory.BSS.Read32(0xf40)
			if err != nil || clock != loadedClock[side]+uint32(tick)+1 {
				t.Fatal("native loaded main clock did not advance once", side, tick, clock, loadedClock[side], err)
			}
		}
		for _, span := range [][2]int{{0xf44, 0x4000}, {0x4f44, 0x1000}, {0x5f50, 0x87f0}, {0xeb28, 4}, {0xf40, 4}} {
			for i := 0; i < span[1]; i++ {
				a, err := p.Hosts[0].Memory.BSS.Read8(span[0] + i)
				if err != nil {
					t.Fatal(err)
				}
				b, err := p.Hosts[1].Memory.BSS.Read8(span[0] + i)
				if err != nil {
					t.Fatal(err)
				}
				if a != b {
					t.Fatalf("paired loaded main diverged at frame%d BSS%x: %x/%x", tick, span[0]+i, a, b)
				}
			}
		}
	}
	for side, h := range p.Hosts {
		raw, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		rng, err := h.Memory.BSS.Read32(0xeb28)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(loadedActors[side], raw[0x5f50:0xe740]) {
			t.Fatal("twenty loaded full mains did not advance native actors", side)
		}
		t.Logf("peer%d:20 unpaused full mains moved native actors; RNG %08x→%08x", side, loadedRNG[side], rng)
	}

}
