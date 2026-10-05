package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeFrameEntryAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_frame_session_entry_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name                  string
				Menu, Exit, AfterExit uint16
				D                     [8]uint32
			}
			PC          uint32
			D           [8]uint32
			BSSHash     string
			MenuEntered bool
			MenuD       [8]uint32
			MenuHash    string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 30 {
		t.Fatal("native entry corpus incomplete", err)
	}
	for _, f := range catalog.Cases {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, pending), func(t *testing.T) {
				raw := make([]byte, 0x11280)
				memory := commandFrameBacking(raw)
				_ = memory.Write16(0xdce, f.Input.Menu)
				_ = memory.Write16(0x3aa, f.Input.Exit)
				session := NativeFrameSession{Phase: NativeFrameSessionVBlank, Frame: NativeFrameRegisterContext{D: f.Input.D}}
				calls := 0
				cb := NativeFrameSessionCallbacks{Menu: func(m FollowerCleanupMemory, c *NativeFrameRegisterContext, _ *NativeImageRenderState, phase *uint32) (bool, error) {
					calls++
					if calls == 1 {
						if c.D != f.MenuD || fileFrameHash(raw) != f.MenuHash {
							t.Fatal("original menu entry prefix differs")
						}
						for i := range c.D {
							c.D[i] ^= 0x13570000 + uint32(i)*0x01010101
						}
						_ = m.Write16(0x3aa, f.Input.AfterExit)
						if pending {
							*phase = 1
							return false, nil
						}
					}
					if calls > 2 {
						t.Fatal("native menu repeated")
					}
					return true, nil
				}}
				ready, exit, err := session.advanceFrameEntry(memory, cb)
				if err != nil {
					t.Fatal(err)
				}
				if pending && f.MenuEntered {
					if ready || exit || session.Phase != NativeFrameSessionMenu {
						t.Fatal("menu wait skipped")
					}
					ready, exit, err = session.advanceFrameEntry(memory, cb)
					if err != nil {
						t.Fatal(err)
					}
				}
				if exit != (f.PC == 0x1bda) || ready == exit || session.Frame.D != f.D || fileFrameHash(raw) != f.BSSHash || ((calls != 0) != f.MenuEntered) {
					t.Fatal("original frame entry output differs", ready, exit, session.Frame.D, f.D)
				}
			})
		}
	}
}

func TestNativeFrameSessionMenuAndExitOwnTheRawWorld(t *testing.T) {
	w, session := nativeSessionTestSetup(t)
	if err := session.Begin(w, NativeFrameRegisterContext{}); err != nil {
		t.Fatal(err)
	}
	memory := session.Presentation.Memory(w.nativeCleanupMemory())
	_ = memory.Write16(0xdce, 1)
	calls := 0
	cb := NativeFrameSessionCallbacks{Menu: func(m FollowerCleanupMemory, _ *NativeFrameRegisterContext, _ *NativeImageRenderState, phase *uint32) (bool, error) {
		calls++
		if value, _ := m.Read16(0xdce); value != 0 {
			t.Fatal("menu flag was not cleared before446A")
		}
		if w.nativeCallDepth != 1 {
			t.Fatal("raw World released during menu")
		}
		if *phase == 0 {
			*phase = 1
			return false, nil
		}
		_ = m.Write16(0x3aa, 1)
		return true, nil
	}}
	if done, err := session.Advance(cb); err != nil || done || session.Phase != NativeFrameSessionMenu || w.nativeCallDepth != 1 {
		t.Fatal("menu did not retain source frame", done, err)
	}
	if done, err := session.Advance(cb); err != nil || !done || !session.ExitRequested || w.nativeCallDepth != 0 || calls != 2 || session.Phase != NativeFrameSessionIdle {
		t.Fatal("source exit did not release frame before rendering", done, err, calls)
	}
}

func TestNativeFrameEntryDoesNotPollFlagsInsideVBlankWait(t *testing.T) {
	raw := make([]byte, 0x11280)
	memory := commandFrameBacking(raw)
	session := NativeFrameSession{Phase: NativeFrameSessionVBlank}
	if ready, exit, err := session.advanceFrameEntry(memory, NativeFrameSessionCallbacks{}); err != nil || !ready || exit {
		t.Fatal(ready, exit, err)
	}
	_ = memory.Write16(0xdce, 1)
	_ = memory.Write16(0x3aa, 1)
	if ready, exit, err := session.advanceFrameEntry(memory, NativeFrameSessionCallbacks{}); err != nil || !ready || exit {
		t.Fatal("entry flags were repolled after source786 admission", ready, exit, err)
	}
}
