package populous2

import (
	"fmt"
	"io"
	"sync"
)

// NativeRuntimeAccess serializes complete operations on one shared native
// runtime. Its zero value is ready for use; it must not be copied after use.
// Every frame/menu/resource mutation, PCM read and external audio control
// operation sharing that runtime must use this SAME instance.
//
// Execute is not reentrant. Inside it, use the borrowed owners directly or
// WithinExecuteCallbacks, never another Execute, the wrapped Reader or a
// Locked audio method. The lock order is runtime access, then PCM's mutex.
// This is host memory ownership, not a claim about original execution pacing.
type NativeRuntimeAccess struct{ mu sync.Mutex }

func (a *NativeRuntimeAccess) Execute(operation func() error) error {
	if a == nil || operation == nil {
		return fmt.Errorf("native runtime access/operation missing")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return operation()
}

// Reader locks a complete Read using the same runtime owner as Execute. A
// backend must not enter Execute again from Read. An error, partial read or
// EOF keeps the backend's original return values and releases ownership.
func (a *NativeRuntimeAccess) Reader(reader io.Reader) (io.Reader, error) {
	if a == nil || reader == nil {
		return nil, fmt.Errorf("native runtime access/reader missing")
	}
	// Reusing our own locked wrapper must not introduce a second lock of
	// the same nonreentrant owner. A different owner would break the one
	// runtime ownership contract and has no supported lock order here.
	switch r := reader.(type) {
	case *nativeRuntimeReader:
		if r == nil || r.access != a {
			return nil, fmt.Errorf("native reader belongs to another runtime owner")
		}
		return r, nil
	case *NativeRuntimePCMAccess:
		if r == nil || r.access != a {
			return nil, fmt.Errorf("native PCM reader belongs to another runtime owner")
		}
		return r, nil
	}
	return &nativeRuntimeReader{access: a, reader: reader}, nil
}

type nativeRuntimeReader struct {
	access *NativeRuntimeAccess
	reader io.Reader
}

func (r *nativeRuntimeReader) Read(dst []byte) (n int, err error) {
	err = r.access.Execute(func() error { var e error; n, e = r.reader.Read(dst); return e })
	return n, err
}

// NativeRuntimePCMAccess keeps the same outer runtime owner for streaming
// Read and external audio commands. Its raw PCM is intentionally private:
// children executing inside an owned frame receive explicit callbacks below.
type NativeRuntimePCMAccess struct {
	access *NativeRuntimeAccess
	pcm    *NativeAudioPCM
}

func (a *NativeRuntimeAccess) BindPCM(pcm *NativeAudioPCM) (*NativeRuntimePCMAccess, error) {
	if a == nil || pcm == nil {
		return nil, fmt.Errorf("native runtime access/PCM missing")
	}
	return &NativeRuntimePCMAccess{access: a, pcm: pcm}, nil
}

func (p *NativeRuntimePCMAccess) valid() error {
	if p == nil || p.access == nil || p.pcm == nil {
		return fmt.Errorf("native runtime PCM owner missing")
	}
	return nil
}

// Read is the externally locked io.Reader operation. The audio host receives
// this wrapper, rather than the underlying PCM, when running on another thread.
func (p *NativeRuntimePCMAccess) Read(dst []byte) (n int, err error) {
	if err = p.valid(); err != nil {
		return 0, err
	}
	err = p.access.Execute(func() error { var e error; n, e = p.pcm.Read(dst); return e })
	return n, err
}

func (p *NativeRuntimePCMAccess) LockedCommand(control, data uint16, input uint32) (output uint32, err error) {
	if err = p.valid(); err != nil {
		return 0, err
	}
	err = p.access.Execute(func() error { var e error; output, e = p.pcm.Command(control, data, input); return e })
	return output, err
}

func (p *NativeRuntimePCMAccess) LockedMusicCommand(control, data uint16, input uint32) (output uint32, err error) {
	if err = p.valid(); err != nil {
		return 0, err
	}
	err = p.access.Execute(func() error { var e error; output, e = p.pcm.MusicCommand(control, data, input); return e })
	return output, err
}

func (p *NativeRuntimePCMAccess) LockedDirectCue(offset uint16, frame *NativeFrameRegisterContext) error {
	if err := p.valid(); err != nil {
		return err
	}
	return p.access.Execute(func() error { return p.pcm.DirectCue(offset, frame) })
}

type NativeRuntimeAudioOperations struct {
	Command      func(control, data uint16, input uint32) (uint32, error)
	MusicCommand func(control, data uint16, input uint32) (uint32, error)
	DirectCue    func(offset uint16, frame *NativeFrameRegisterContext) error
}

// WithinExecuteCallbacks borrows the PCM's existing operations for children
// called synchronously INSIDE Execute. They take only PCM's inner mutex and
// never reacquire the outer mutex. They must not escape to asynchronous work,
// another goroutine or an unowned call. Existing source APIs remain unchanged.
func (p *NativeRuntimePCMAccess) WithinExecuteCallbacks() (NativeRuntimeAudioOperations, error) {
	if err := p.valid(); err != nil {
		return NativeRuntimeAudioOperations{}, err
	}
	return NativeRuntimeAudioOperations{Command: p.pcm.Command, MusicCommand: p.pcm.MusicCommand, DirectCue: p.pcm.DirectCue}, nil
}

// ExternalCallbacks take the outer mutex themselves. They are suitable for
// controls arriving outside an Execute scope and must not be installed as
// children of an already-owned frame/menu/resource operation.
func (p *NativeRuntimePCMAccess) ExternalCallbacks() (NativeRuntimeAudioOperations, error) {
	if err := p.valid(); err != nil {
		return NativeRuntimeAudioOperations{}, err
	}
	return NativeRuntimeAudioOperations{Command: p.LockedCommand, MusicCommand: p.LockedMusicCommand, DirectCue: p.LockedDirectCue}, nil
}
