package populous2

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestNativeRuntimeTCPPartialPacketRunsErrorAndResumesFrame(t *testing.T) {
	h, device, incoming := nativeGameplayIntegrationStartup(t, 0, 4311)
	left, right := nativeRuntimePairTCP(t)
	conn, err := NewNativeSerialConn(left)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	transport, err := h.NewTransport(conn, NativeTransportFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}}, func(bool, *NativeFrameRegisterContext) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []nativeHeroPatch{{0xeb44, 2, 6}, {0xeb5e, 1, 8}, {0xeb68, 1, 6}, {0x15e, 2, 380}} {
		renderFramePatch(h.Memory.BSS, p)
	}
	aliasBefore, err := h.Memory.BSS.Read8(0xeb72)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte{1, 8, 22, 23}
	if n, err := right.Write(prefix); err != nil || n != len(prefix) {
		t.Fatal("actual TCP prefix failed", n, err)
	}
	if err = right.Close(); err != nil {
		t.Fatal(err)
	}
	operations := NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}
	frame, err := h.NewFrame(NativeRuntimeFrameBindings{Audio: operations, RenderChildren: NativeRuntimeRenderChildrenCallbacks{Beam: func() (uint16, error) { return 0, nil }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: device.DirectCue}, InputChildren: NativeGameplayHUDHostCallbacks{Ownership: func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error { return nil }, Audio: operations}, Session: NativeFrameSessionCallbacks{Transport: transport.PacketCallback}})
	if err != nil {
		t.Fatal(err)
	}
	renderCalls, inputCalls := 0, 0
	render, input := frame.Callbacks.Render, frame.Callbacks.Input
	frame.Callbacks.Render = func(m FollowerCleanupMemory, c *NativeFrameRegisterContext, image *NativeImageRenderState, bitmap []byte, phase *uint32) (bool, error) {
		renderCalls++
		return render(m, c, image, bitmap, phase)
	}
	frame.Callbacks.Input = func(m FollowerCleanupMemory, c *NativeFrameRegisterContext, phase *uint32) (bool, error) {
		inputCalls++
		return input(m, c, phase)
	}
	if err = h.Session.BeginRaw(h.World, incoming); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if h.Session.Phase != NativeFrameSessionIdle {
			h.Session.finish(nil)
		}
	}()
	blank := func() {
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &h.Session.Frame); err != nil {
			t.Fatal(err)
		}
	}
	for poll := 0; transport.mismatch == nil || transport.mismatch.PC != 0x33ea; poll++ {
		if poll > 80 {
			t.Fatalf("actual TCP failure did not reach the original error click: phase%d", h.Session.Phase)
		}
		blank()
		done, err := frame.Advance()
		if err != nil || done {
			t.Fatal("partial TCP frame completed before error acknowledgement", done, err)
		}
		time.Sleep(100 * time.Microsecond)
	}
	if transport.mismatch.Flag != 0 {
		t.Fatal("failure did not exercise the actual error palette gates", transport.mismatch.Flag)
	}
	if h.World.nativeCallDepth != 1 || h.Session.Pass.Stage != NativeFrameCommands || inputCalls != 0 {
		t.Fatal("error did not retain the source command stage", h.World.nativeCallDepth, h.Session.Pass.Stage, inputCalls)
	}
	packet := transport.packets[0]
	if packet == nil || packet.position != 4 || !errors.Is(packet.Failure, io.EOF) || !packet.Disconnected {
		t.Fatal("TCP EOF lost the transferred native prefix", packet)
	}
	rng, err := h.Memory.BSS.Read32(0xeb28)
	if err != nil {
		t.Fatal(err)
	}
	caller := make([]byte, len(prefix))
	for i := range caller {
		caller[i], err = h.Memory.BSS.Read8(0xeb56 + i)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(caller, prefix) {
		t.Fatal("received source prefix was discarded", caller)
	}
	before, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	rendersBefore := renderCalls
	clockBefore, err := h.Memory.BSS.Read32(0xf40)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		blank()
		done, err := frame.Advance()
		if err != nil || done {
			t.Fatal("source error requester completed without actual click", done, err)
		}
		after, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		clock, err := h.Memory.BSS.Read32(0xf40)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before[0xf44:0xe740], after[0xf44:0xe740]) || renderCalls != rendersBefore || inputCalls != 0 || clock != clockBefore {
			t.Fatal("pending network dialog replayed simulation/render/input")
		}
	}
	nativeRuntimeClickAction(t, h, &h.Session.Frame, 2)
	done := false
	for poll := 0; poll < 80 && !done; poll++ {
		done, err = frame.Advance()
		if err != nil {
			t.Fatal(err)
		}
		if !done {
			blank()
		}
	}
	if !done || transport.mismatch != nil || transport.packets[0] != nil || h.World.nativeCallDepth != 0 || inputCalls != 1 || renderCalls != rendersBefore {
		t.Fatal("actual error acknowledgement did not resume the retained frame", done, inputCalls, renderCalls)
	}
	if got, err := h.Memory.BSS.Read32(0xeb28); err != nil || got != rng {
		t.Fatal("network failure repaired or repeated native RNG", got, rng, err)
	}
	for _, p := range []struct {
		at   int
		want uint8
	}{{0xeb5e, 4}, {0xeb68, 2}, {0xeb72, nativeRuntimeFailureDisconnectedMode(aliasBefore)}} {
		if got, err := h.Memory.BSS.Read8(p.at); err != nil || got != p.want {
			t.Fatal("native1826E fallback/third-record alias missing", p.at, got, p.want, err)
		}
	}
	if mode, err := h.Memory.BSS.Read16(0xeb44); err != nil || mode != 4 {
		t.Fatal("native failure did not return to local game mode", mode, err)
	}
	if step := h.Session.commandStates[0].Step; step.Command != 8 {
		t.Fatal("received command was not passed to the actual17500 body", step)
	}
	// The disconnected modes must run their genuine next complete local frame.
	if err = h.Session.BeginRaw(h.World, h.Session.Frame); err != nil {
		t.Fatal(err)
	}
	done = false
	for poll := 0; poll < 80 && !done; poll++ {
		blank()
		done, err = frame.Advance()
		if err != nil {
			t.Fatal(err)
		}
	}
	if !done || h.World.nativeCallDepth != 0 || inputCalls != 2 {
		t.Fatal("native local frame did not resume after TCP failure", done, inputCalls)
	}
}

func TestNativeRuntimeTCPMenuDropRetainsOriginalCorruptReturn(t *testing.T) {
	h, device, incoming := nativeGameplayIntegrationStartup(t, 0, 4311)
	left, right := nativeRuntimePairTCP(t)
	conn, err := NewNativeSerialConn(left)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	transport, err := h.NewTransport(conn, NativeTransportFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Sound: device.DirectCue}, CallerStackPointer: 0xef0000}, func(bool, *NativeFrameRegisterContext) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []nativeHeroPatch{{0xeb6a, 4, h.Memory.BSSBase + 0xeb56}, {0xeb42, 2, 1}, {0xe90c, 2, 0x1234}, {0xeb44, 2, 6}, {0xeb5e, 1, 6}, {0xeb68, 1, 8}, {0x15e, 2, 380}} {
		renderFramePatch(h.Memory.BSS, p)
	}
	var received [10]byte
	if err = right.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	peer := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(right, received[:])
		closeErr := right.Close()
		if err == nil {
			err = closeErr
		}
		peer <- err
	}()
	phase := uint32(0)
	deadline := time.Now().Add(5 * time.Second)
	var result NativeCommandFrameResult
	for {
		result, err = transport.ResumeMenuChild(NativeFileFrameCall{Routine: 0x181c0, Frame: &incoming}, &phase)
		if err != nil {
			break
		}
		if result.Complete {
			t.Fatal("actual dropped peer fabricated a normal menu return")
		}
		if time.Now().After(deadline) {
			t.Fatal("actual TCP menu drop remained pending")
		}
		time.Sleep(100 * time.Microsecond)
	}
	if peerErr := <-peer; peerErr != nil {
		t.Fatal("peer did not read the complete native stack packet", peerErr)
	}
	if result.Complete || !strings.Contains(err.Error(), "00011234") || transport.Resume.PacketBase != 0xeefff6 || !errors.Is(transport.Resume.Failure, io.EOF) || incoming.D[0] != 1 {
		t.Fatal("actual TCP drop repaired the native181C0 stack outcome", result, incoming.D, transport.Resume.PacketBase, err)
	}
	if !bytes.Equal(received[:], transport.Resume.Packet[:]) {
		t.Fatal("menu stream did not carry the exact original stack bytes", received, transport.Resume.Packet)
	}
	again, againErr := transport.ResumeMenuChild(NativeFileFrameCall{Routine: 0x181c0, Frame: &incoming}, &phase)
	if again.Complete || againErr == nil || againErr.Error() != err.Error() {
		t.Fatal("corrupt return became a completed menu on retry", again, againErr)
	}
}

func nativeRuntimeFailureDisconnectedMode(mode uint8) uint8 {
	if mode == 0 {
		return 0
	}
	if mode == 2 || mode == 6 {
		return 2
	}
	return 4
}
