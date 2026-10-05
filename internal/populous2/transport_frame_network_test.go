package populous2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestNativeTransportFramePairedConnections(t *testing.T) {
	exe := testBundle(t).Executable
	left, right := net.Pipe()
	var conns [2]*NativeSerialConn
	var states [2]NativeTransportHandshakeState
	var callbacks [2]NativeTransportFrameCallbacks
	var memory [2]FollowerCleanupMemory
	var frames [2]NativeFrameRegisterContext
	var initialized [2]int
	for side, connection := range []net.Conn{left, right} {
		conn, e := NewNativeSerialConn(connection)
		if e != nil {
			t.Fatal(e)
		}
		conns[side] = conn
		defer conn.Close()
		p, e := NewNativeFramePresentationState(exe, 0x500000, 0x400000)
		if e != nil {
			t.Fatal(e)
		}
		p.InterruptChain = false
		if _, e = p.Initialize(exe, NativeMouseSample{}); e != nil {
			t.Fatal(e)
		}
		memory[side] = p.Memory(commandFrameBacking(make([]byte, 0x11280)))
		m := memory[side]
		code := fileFrameRelocatedCode(t)
		cm := commandFrameBacking(code)
		_ = cm.Write16(0x3ea, 0)
		_ = cm.Write32(0x77a, p.CopperSelector)
		_ = cm.Write32(0x77e, p.SpritePatchPointer)
		_ = m.Write16(0x15a, 4800)
		_ = m.Write16(0x15e, 380)
		_ = m.Write16(0xeb42, uint16(side+1))
		_ = m.Write16(0xeb44, 4)
		_ = m.Write8(0xeb5e, 2)
		_ = m.Write8(0xeb68, 4)
		_ = m.Write16(0xeb22, 1)
		_ = m.Write32(0xeb24, 0x1a3b5+uint32(side)*100)
		_ = m.Write16(0xeb2c, 0x155)
		_ = m.Write16(0xeb2e, 0x2aa)
		for i := 0; i < 236; i++ {
			_ = m.Write8(0xe8f2+side*314+i, byte(i*17+side*51))
		}
		frames[side] = NativeFrameRegisterContext{D: [8]uint32{0x12340000 + uint32(side), 0x56780101, 0x9abc0202, 0xdef00303, 0x11110404, 0x22220505, 0x33330606, 0x44440707}, AddressBase: 0x200000}
		waitSite, waitCalls := uint32(0), 0
		callbacks[side] = NativeTransportFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frames[side], Presentation: p, Bitmap: func(address uint32) ([]byte, error) {
			at := int(address - p.ChipBase)
			if at < 0 || at > len(p.Chip)-32000 {
				return nil, fmt.Errorf("screen unavailable")
			}
			return p.Chip[at : at+32000], nil
		}}, Port: conn.Port(), ReceiveImage: conn.NativeTransportReceiveImage, WaitCPU: func(site, count uint32) (bool, error) {
			if count != 100000 {
				t.Fatal("source delay lost")
			}
			if site != waitSite {
				waitSite, waitCalls = site, 0
			}
			waitCalls++
			return waitCalls >= 10, nil
		}, CallTransport: func(call NativeFileFrameCall, phase *uint32) (NativeSerialFrameChildResult, error) {
			if call.Routine != 0x10ad8 {
				return NativeSerialFrameChildResult{}, fmt.Errorf("unexpected real requester child%x", call.Routine)
			}
			if *phase == 0 {
				*phase = 1
				return NativeSerialFrameChildResult{}, nil
			}
			initialized[side]++
			for i := 3; i < 8; i++ {
				call.Frame.D[i] ^= 0xfeed0000 + uint32(i)
			}
			return NativeSerialFrameChildResult{Complete: true, Negative: true}, nil
		}}
	}
	deadline := time.Now().Add(5 * time.Second)
	clock := uint32(0)
	waits := 0
	for !states[0].Finished || !states[1].Finished {
		if time.Now().After(deadline) {
			t.Fatalf("real handshake stalled at%x/%x", states[0].PC, states[1].PC)
		}
		clock++
		for side := range states {
			_ = memory[side].Write32(0x12, clock)
			step, e := states[side].Advance(callbacks[side])
			if e != nil {
				t.Fatal(e)
			}
			if step.Waiting {
				if step.FlagsKnown {
					t.Fatal("pending child fabricated returned CCR")
				}
				waits++
			}
			if step.Complete && (!step.FlagsKnown || !step.Negative || step.Zero) {
				t.Fatal("real reset CCR lost despite outer register restore")
			}
		}
		time.Sleep(100 * time.Microsecond)
	}
	if initialized != [2]int{1, 1} || waits == 0 {
		t.Fatal("real stream skipped or repeated reset continuation")
	}
	for _, span := range [][2]int{{0xeb22, 14}, {0xe8f2, 236}, {0xea2c, 236}} {
		a := make([]byte, span[1])
		b := make([]byte, span[1])
		for i := range a {
			a[i], _ = memory[0].Read8(span[0] + i)
			b[i], _ = memory[1].Read8(span[0] + i)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("actual negotiated bytes differ at%x", span[0])
		}
	}
	for side := range states {
		if frames[side].D[0] != 0x12340000+uint32(side) || frames[side].D[1] != 0x56780101 || frames[side].D[2] != 0x9abc0202 {
			t.Fatal("outer handshake MOVEM lost high words")
		}
		if conns[side].Baud() != 4800 {
			t.Fatal("actual port not configured")
		}
	}
	var resume [2]NativeTransportResumeState
	for side := range callbacks {
		_ = memory[side].Write32(0xeb6a, 0x200000+0xeb56)
		callbacks[side].CallerStackPointer = 0xef0000
	}
	deadline = time.Now().Add(time.Second)
	for !resume[0].Finished || !resume[1].Finished {
		if time.Now().After(deadline) {
			t.Fatalf("real resume stalled at%x/%x", resume[0].PC, resume[1].PC)
		}
		for side := range resume {
			step, e := resume[side].Advance(callbacks[side])
			if e != nil {
				t.Fatal(e)
			}
			if step.CorruptReturn {
				t.Fatal("paired stream unexpectedly disconnected")
			}
			if step.Complete && (!step.FlagsKnown || !step.Zero || step.Negative || frames[side].D[0] != 0) {
				t.Fatal("source resume flags lost")
			}
		}
		time.Sleep(100 * time.Microsecond)
	}
	if initialized != [2]int{1, 1} {
		t.Fatal("resume replayed initialization")
	}

}

func TestNativeTransportResumeHostFailureRetainsNativeStackOutcome(t *testing.T) {
	exe := testBundle(t).Executable
	p, e := NewNativeFramePresentationState(exe, 0x500000, 0x400000)
	if e != nil {
		t.Fatal(e)
	}
	p.InterruptChain = false
	if _, e = p.Initialize(exe, NativeMouseSample{}); e != nil {
		t.Fatal(e)
	}
	m := p.Memory(commandFrameBacking(make([]byte, 0x11280)))
	code := fileFrameRelocatedCode(t)
	cm := commandFrameBacking(code)
	_ = cm.Write16(0x3ea, 0)
	_ = cm.Write32(0x77a, p.CopperSelector)
	_ = cm.Write32(0x77e, p.SpritePatchPointer)
	_ = m.Write32(0xeb6a, 0x200000+0xeb56)
	_ = m.Write16(0xeb42, 1)
	_ = m.Write16(0xe90c, 0x1234)
	_ = m.Write16(0xeb44, 6)
	for i := 0; i < 3; i++ {
		_ = m.Write8(0xeb5e+i*10, 6)
	}
	frame := NativeFrameRegisterContext{D: [8]uint32{0xffcc0000, 0x12340001, 0x56780002, 0x98760003}, AddressBase: 0x200000}
	flushed := 0
	written := 0
	cb := NativeTransportFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: func(address uint32) ([]byte, error) {
		at := int(address - p.ChipBase)
		return p.Chip[at : at+32000], nil
	}}, Port: NativeSerialPort{Available: func() (int, error) { return 0, io.EOF }, Read: func([]byte) (int, error) { t.Fatal("source read after negative availability"); return 0, nil }, Write: func(src []byte) (int, error) { written += len(src); return len(src), nil }, Flush: func() error { flushed++; return nil }}, CallerStackPointer: 0xef0000}
	state := NativeTransportResumeState{}
	step, e := state.Advance(cb)
	if e != nil {
		t.Fatal(e)
	}
	if step.Complete || !step.CorruptReturn || !step.FlagsKnown || step.Zero || step.Negative || step.NativeStackReturn != 0x00011234 || state.PacketBase != 0xeefff6 {
		t.Fatalf("source abort stack repaired %+v", step)
	}
	if !errors.Is(step.Failure, io.EOF) || flushed != 1 || written != 10 || frame.D[0] != 1 {
		t.Fatal("host failure or actual output prefix lost")
	}
	for i := 0; i < 3; i++ {
		mode, _ := m.Read8(0xeb5e + i*10)
		if mode != 2 {
			t.Fatal("actual three-record disconnect alias missing")
		}
	}
	mode, _ := m.Read16(0xeb44)
	if mode != 4 {
		t.Fatal("actual disconnected status missing")
	}
	again, e := state.Advance(cb)
	if e != nil || !again.CorruptReturn || !errors.Is(again.Failure, io.EOF) || written != 10 || flushed != 1 {
		t.Fatal("abort continuation replayed transport")
	}
}
