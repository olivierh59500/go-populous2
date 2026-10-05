package populous2

import (
	"reflect"
	"testing"
)

func TestNativeDeferredFrameRetainsPendingSideAndRegisters(t *testing.T) {
	w, e := NewWorld(testBundle(t), 0, false)
	if e != nil {
		t.Fatal(e)
	}
	m := w.nativeCleanupMemory()
	for side, mode := range []uint8{6, 4} {
		at := 0xeb56 + side*10
		_ = m.Write8(at+8, mode)
		_ = m.Write8(at+1, 108)
		_ = m.Write16(at+2, 0x1234)
	}
	initial := [8]uint32{1, 2, 3, 4, 5, 6, 7, 0xaabbff00}
	frame := NativeFrameRegisterContext{D: initial}
	state := NativeDeferredFrameState{}
	transportCalls, bodyCalls, applied := 0, 0, 0
	cb := NativeDeferredFrameCallbacks{Memory: m,
		Transport: func(at int, cMode uint8, c *NativeCommandRegisterContext, phase *uint32) (bool, error) {
			transportCalls++
			if at != 0xeb56 || cMode != 6 {
				t.Fatal("transport side order changed")
			}
			if *phase == 0 {
				*phase = 1
				c.D[7] = 0xcafefeff
				return false, nil
			}
			return true, nil
		},
		Execute: func(at int, c *NativeCommandRegisterContext, phase *uint32) (bool, error) {
			bodyCalls++
			if at == 0xeb56 {
				if c.D[7] != 0xcafefeff {
					t.Fatal("transport registers lost")
				}
				if *phase == 0 {
					*phase = 1
					applied++
					c.D[4] = 0x12345678
					return false, nil
				}
				return true, nil
			}
			if at != 0xeb60 || c.D[4] != 0x12345678 || c.D[7] != 0xcafefeff {
				t.Fatal("side2 did not retain side1 context")
			}
			applied++
			return true, nil
		},
	}
	done, e := state.TickDeferredFrame(&frame, cb)
	if e != nil || done {
		t.Fatal("transport suspension missing", e)
	}
	if state.Phase != NativeDeferredFrameTransport || state.Side != 0 || state.CallbackPhase != 1 {
		t.Fatal("transport continuation lost")
	}
	if command, _ := m.Read8(0xeb57); command != 108 {
		t.Fatal("pending transport cleared command")
	}
	done, e = state.TickDeferredFrame(&frame, cb)
	if e != nil || done {
		t.Fatal("body suspension missing", e)
	}
	if state.Phase != NativeDeferredFrameExecute || state.CallbackPhase != 1 || state.Command.D[4] != 0x12345678 {
		t.Fatal("body continuation lost")
	}
	if command, _ := m.Read8(0xeb57); command != 108 {
		t.Fatal("pending body cleared command")
	}
	done, e = state.TickDeferredFrame(&frame, cb)
	if e != nil || !done {
		t.Fatal("deferred continuation incomplete", e)
	}
	if transportCalls != 2 || bodyCalls != 3 || applied != 2 || frame.D != initial {
		t.Fatal("completed operation replayed or caller MOVEM lost")
	}
	for _, at := range []int{0xeb56, 0xeb60} {
		command, _ := m.Read8(at + 1)
		xy, _ := m.Read16(at + 2)
		if command != 0 || xy != 0 {
			t.Fatal("source completed-command fields not cleared")
		}
	}
}

func TestNativeFramePassDoesNotReplayPhysicsAcrossPendingCommand(t *testing.T) {
	w, e := NewWorld(testBundle(t), 0, false)
	if e != nil {
		t.Fatal(e)
	}
	m := w.nativeCleanupMemory()
	_ = m.Write16(0xf3c, 0)
	_ = m.Write16(0xf3e, 0)
	state := NativeFramePassState{}
	frame := NativeFrameRegisterContext{}
	order := []NativeFrameStage{}
	callback := func(stage NativeFrameStage) NativeFrameStageCallback {
		return func(*NativeFrameRegisterContext) (bool, error) { order = append(order, stage); return true, nil }
	}
	pending := true
	cb := NativeFrameCallbacks{Memory: m, Followers: callback(NativeFrameFollowers), AI: callback(NativeFrameAI), FX: callback(NativeFrameFX), Walls: callback(NativeFrameWalls), Forest: callback(NativeFrameForest), Scenario: callback(NativeFrameScenario), Audio: callback(NativeFrameAudio), Swap: callback(NativeFrameSwap), Commands: func(*NativeFrameRegisterContext) (bool, error) {
		order = append(order, NativeFrameCommands)
		if pending {
			pending = false
			return false, nil
		}
		return true, nil
	}}
	complete, e := state.TickFramePass(&frame, cb)
	if e != nil || complete || state.Stage != NativeFrameCommands {
		t.Fatal("pending command stage lost", e)
	}
	complete, e = state.TickFramePass(&frame, cb)
	if e != nil || !complete {
		t.Fatal("command resume failed", e)
	}
	want := []NativeFrameStage{NativeFrameFollowers, NativeFrameAI, NativeFrameFX, NativeFrameWalls, NativeFrameForest, NativeFrameScenario, NativeFrameAudio, NativeFrameSwap, NativeFrameCommands, NativeFrameCommands}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("source stage replay/order changed%v", order)
	}
}
