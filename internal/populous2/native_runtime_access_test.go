package populous2

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"
)

type nativeRuntimeReaderFunc func([]byte) (int, error)

func (r nativeRuntimeReaderFunc) Read(dst []byte) (int, error) { return r(dst) }

func TestNativeRuntimeAccessSerializesCompleteSharedMemoryOperations(t *testing.T) {
	code, host := nativeSharedCodeTestView(t)
	var access NativeRuntimeAccess
	// A real physical LONG at this scalar debug slot is updated in two WORD
	// writes, as native operations can do. The reader deliberately yields
	// between byte reads; both complete operations still share one owner.
	const at = 0x2e3a
	if err := code.Physical().Write32(at, 0x0000ffff); err != nil {
		t.Fatal(err)
	}
	reads, writes := 0, 0
	reader, err := access.Reader(nativeRuntimeReaderFunc(func(dst []byte) (int, error) {
		for i := 0; i < 4; i++ {
			value, err := host.Memory().Read8(int(code.CodeBase) + at + i)
			if err != nil {
				return i, err
			}
			dst[i] = value
			runtime.Gosched()
		}
		reads++
		return 4, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	const iterations = 256
	var work sync.WaitGroup
	failures := make(chan error, 4)
	start := make(chan struct{})
	for worker := 0; worker < 2; worker++ {
		work.Add(1)
		go func(worker int) {
			defer work.Done()
			<-start
			for i := 0; i < iterations; i++ {
				err := access.Execute(func() error {
					value := uint16(i + worker*iterations)
					if err := code.Physical().Write16(at, value); err != nil {
						return err
					}
					runtime.Gosched()
					if err := code.Physical().Write16(at+2, ^value); err != nil {
						return err
					}
					writes++
					return nil
				})
				if err != nil {
					failures <- err
					return
				}
			}
		}(worker)
	}
	for worker := 0; worker < 2; worker++ {
		work.Add(1)
		go func() {
			defer work.Done()
			<-start
			var bytes [4]byte
			for i := 0; i < iterations; i++ {
				if n, err := reader.Read(bytes[:]); err != nil || n != 4 {
					failures <- fmt.Errorf("owned physical reader failed: %d/%v", n, err)
					return
				}
				if binary.BigEndian.Uint16(bytes[:2]) != ^binary.BigEndian.Uint16(bytes[2:]) {
					failures <- fmt.Errorf("reader observed a torn native multi-write operation")
					return
				}
			}
		}()
	}
	close(start)
	work.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	if reads != iterations*2 || writes != iterations*2 {
		t.Fatal("complete shared operations were dropped")
	}
}

func TestNativeRuntimeAccessPreservesReaderReturnsAndReleasesOwnership(t *testing.T) {
	var access NativeRuntimeAccess
	want := errors.New("source backend failed after a partial read")
	reader, err := access.Reader(nativeRuntimeReaderFunc(func(dst []byte) (int, error) { dst[0] = 0x5a; return 1, want }))
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 4)
	if n, err := reader.Read(data); n != 1 || !errors.Is(err, want) || data[0] != 0x5a {
		t.Fatal("partial source read changed", n, err)
	}
	if err := access.Execute(func() error { return want }); !errors.Is(err, want) {
		t.Fatal("source operation error changed")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("source panic was discarded")
			}
		}()
		_ = access.Execute(func() error { panic("source panic") })
	}()
	done := make(chan error, 1)
	go func() { done <- access.Execute(func() error { return nil }) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("error/panic leaked runtime ownership")
	}
	eof, err := access.Reader(nativeRuntimeReaderFunc(func([]byte) (int, error) { return 0, io.EOF }))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := eof.Read(data); n != 0 || err != io.EOF {
		t.Fatal("source EOF changed", n, err)
	}
	if access.Execute(nil) == nil {
		t.Fatal("missing operation accepted")
	}
	if _, err := access.Reader(nil); err == nil {
		t.Fatal("missing reader accepted")
	}
	if _, err := access.BindPCM(nil); err == nil {
		t.Fatal("missing PCM accepted")
	}
	var missing *NativeRuntimeAccess
	if missing.Execute(func() error { return nil }) == nil {
		t.Fatal("missing access owner accepted")
	}
}

func nativeRuntimeAccessPCMTest(t *testing.T) (*NativeRuntimeHost, *NativeAudioDevice, *NativeRuntimeAccess, *NativeRuntimePCMAccess) {
	t.Helper()
	h := nativeRuntimeHostTest(t)
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	if done, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{}); err != nil || !done {
		t.Fatal("actual FX allocation/load failed", done, err)
	}
	resource, err := h.Memory.BSS.Read32(0x3b4)
	if err != nil {
		t.Fatal(err)
	}
	fx, err := h.Host.Span(resource, NativeStartupAudioBytes)
	if err != nil {
		t.Fatal(err)
	}
	device, err := NewNativeSharedAudioDevice(h.Bundle.Executable, fx, h.Code, resource, 0)
	if err != nil {
		t.Fatal(err)
	}
	device.ReadAbsolute8 = func(at uint32) (byte, error) { return h.Memory.RAM.Read8(int(at)) }
	if _, err := device.InitializeWithFrame(&frame); err != nil {
		t.Fatal(err)
	}
	pcm, err := NewNativeAudioPCMWithDMA(device, NativePALAudioTiming(44100, 0), NativePALPaulaDMAConfig(0))
	if err != nil {
		t.Fatal(err)
	}
	access := &NativeRuntimeAccess{}
	bound, err := access.BindPCM(pcm)
	if err != nil {
		t.Fatal(err)
	}
	return h, device, access, bound
}

func TestNativeRuntimeAccessActualSharedPCMAndOwnedFrameCallbacks(t *testing.T) {
	h, device, access, pcm := nativeRuntimeAccessPCMTest(t)
	inside, err := pcm.WithinExecuteCallbacks()
	if err != nil {
		t.Fatal(err)
	}
	outside, err := pcm.ExternalCallbacks()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outside.MusicCommand(0x8f, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := pcm.LockedMusicCommand(0x200f, 32, 0); err != nil {
		t.Fatal(err)
	}
	const iterations = 128
	var work sync.WaitGroup
	failures := make(chan error, 3)
	start := make(chan struct{})
	var audio []byte
	work.Add(3)
	go func() {
		defer work.Done()
		<-start
		block := make([]byte, 257) // Includes genuine PCM partial-frame retention.
		for i := 0; i < iterations; i++ {
			if n, err := io.ReadFull(pcm, block); err != nil || n != len(block) {
				failures <- fmt.Errorf("actual owned PCM read failed: %d/%v", n, err)
				return
			}
			audio = append(audio, block...)
		}
	}()
	go func() {
		defer work.Done()
		<-start
		for i := 0; i < iterations; i++ {
			err := access.Execute(func() error {
				// These are real source control bodies called synchronously
				// inside a complete native host operation, without relocking.
				if _, err := inside.Command(0x2004, uint16(32+i%16), 0); err != nil {
					return err
				}
				if i%8 == 0 {
					frame := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
					before := frame.D
					if err := inside.DirectCue(0, &frame); err != nil {
						return err
					}
					if frame.D != before {
						return fmt.Errorf("source direct-cue MOVEM changed through owned child")
					}
				}
				if err := h.Memory.Code.Write32(0x2e3a, uint32(i)); err != nil {
					return err
				}
				// CIA/PCM writes this same driver byte in its other thread.
				value, err := h.Memory.Code.Read8(0x18b30)
				if err != nil {
					return err
				}
				if value != device.Code[0x18b30] {
					return fmt.Errorf("owned frame and shared driver CODE detached")
				}
				return nil
			})
			if err != nil {
				failures <- err
				return
			}
		}
	}()
	go func() {
		defer work.Done()
		<-start
		for i := 0; i < iterations; i++ {
			if _, err := outside.Command(0x2001, uint16(i%64), uint32(i)); err != nil {
				failures <- err
				return
			}
			if i%16 == 0 {
				if _, err := outside.MusicCommand(0x2006, 32, 0); err != nil {
					failures <- err
					return
				}
			}
		}
	}()
	close(start)
	done := make(chan struct{})
	go func() { work.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("owned frame callback reacquired the outer lock or PCM lock order deadlocked")
	}
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	if len(audio) != iterations*257 {
		t.Fatal("actual PCM bytes lost")
	}
	if err := access.Execute(func() error {
		// A short native excerpt can contain silence, depending on source
		// rests and command order. Prove actual PCM/CIA advancement instead
		// of assigning an invented audible result to concurrent ordering.
		if pcm.pcm.samples != uint64((len(audio)+3)/4) || pcm.pcm.nextCIA <= pcm.pcm.ciaPeriod {
			return fmt.Errorf("actual PCM/CIA producer did not advance")
		}
		value, err := h.Memory.Code.Read32(0x2e3a)
		if err != nil || value != iterations-1 {
			return fmt.Errorf("owned native frame writes dropped: %d/%v", value, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeRuntimeAccessAvoidsDoubleWrappingOwnedReaders(t *testing.T) {
	var owner, other NativeRuntimeAccess
	reader, err := owner.Reader(nativeRuntimeReaderFunc(func(dst []byte) (int, error) { dst[0] = 0x5a; return 1, nil }))
	if err != nil {
		t.Fatal(err)
	}
	again, err := owner.Reader(reader)
	if err != nil || again != reader {
		t.Fatal("same-owner reader added a nested mutex", err)
	}
	if _, err := other.Reader(reader); err == nil {
		t.Fatal("reader from another runtime owner accepted")
	}
	_, _, access, pcm := nativeRuntimeAccessPCMTest(t)
	wrapped, err := access.Reader(pcm)
	if err != nil || wrapped != pcm {
		t.Fatal("same-owner PCM reader added a nested mutex", err)
	}
	if _, err := other.Reader(pcm); err == nil {
		t.Fatal("PCM reader from another runtime owner accepted")
	}
	if _, err := io.ReadFull(wrapped, make([]byte, 17)); err != nil {
		t.Fatal(err)
	}
}

func TestNativeRuntimeAccessAudioCallbackFailuresReleaseOwnership(t *testing.T) {
	_, _, access, pcm := nativeRuntimeAccessPCMTest(t)
	// An odd native cue is rejected by the real source descriptor boundary.
	if err := pcm.LockedDirectCue(1, &NativeFrameRegisterContext{}); err == nil {
		t.Fatal("real invalid cue was acknowledged")
	}
	if err := access.Execute(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := pcm.LockedCommand(0x2004, 32, 0); err != nil {
		t.Fatal("failed cue leaked owned callback lock", err)
	}
	var missing *NativeRuntimePCMAccess
	if _, err := missing.Read(nil); err == nil {
		t.Fatal("missing PCM reader accepted")
	}
	if _, err := missing.ExternalCallbacks(); err == nil {
		t.Fatal("missing external callbacks accepted")
	}
	if _, err := missing.WithinExecuteCallbacks(); err == nil {
		t.Fatal("missing owned callbacks accepted")
	}
}
