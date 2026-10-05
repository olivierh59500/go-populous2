package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeImageAudioCodeOwner uint8

const (
	NativeImageCodeOwner NativeImageAudioCodeOwner = iota
	NativeAudioCodeOwner
)

// NativeImageAudioCodeAlias exposes the source's shared mutable image/audio
// data through callback views. Queue authority changes only at explicit source
// boundaries; it never infers the owner from a host frame phase or copies CODE.
// LastY always belongs to Image, and channel words always belong to Audio.
// The caller serializes access with every other owner of this physical RAM.
type NativeImageAudioCodeAlias struct {
	Image     *NativeImageRenderState
	Audio     *NativeFrameAudioState
	CodeBase  uint32
	RAM, Code FollowerCleanupMemory
	backing   FollowerCleanupMemory
	owner     NativeImageAudioCodeOwner
}

func NewNativeImageAudioCodeAlias(image *NativeImageRenderState, audio *NativeFrameAudioState, physical FollowerCleanupMemory, codeBase uint32, owner NativeImageAudioCodeOwner) (*NativeImageAudioCodeAlias, error) {
	if image == nil || audio == nil || !winMemoryValid(physical) || uint64(codeBase)+0x18ada > 0x100000000 {
		return nil, fmt.Errorf("native image/audio CODE alias backing missing")
	}
	a := &NativeImageAudioCodeAlias{Image: image, Audio: audio, CodeBase: codeBase, backing: physical}
	if err := a.SetOwner(owner); err != nil {
		return nil, err
	}
	a.RAM = nativeByteAddressMemory(a.readByte, a.writeByte)
	a.Code = nativeOffsetMemory(a.RAM, codeBase)
	return a, nil
}

// SetOwner does not transfer queue bytes. The caller first performs the proven
// render→physics copy and selects Audio, then performs the audio→swap copy and
// selects Image before any retained deferred UI child draws again.
func (a *NativeImageAudioCodeAlias) SetOwner(owner NativeImageAudioCodeOwner) error {
	if a == nil || (owner != NativeImageCodeOwner && owner != NativeAudioCodeOwner) {
		return fmt.Errorf("native image/audio CODE queue owner unavailable")
	}
	a.owner = owner
	return nil
}

func (a *NativeImageAudioCodeAlias) Owner() NativeImageAudioCodeOwner { return a.owner }

func (a *NativeImageAudioCodeAlias) queue() *[0x532]byte {
	if a.owner == NativeAudioCodeOwner {
		return &a.Audio.Entries
	}
	return &a.Image.AudioBank
}

func (a *NativeImageAudioCodeAlias) readByte(at int) (uint8, error) {
	offset := int64(at) - int64(a.CodeBase)
	switch {
	case offset >= 0x185a8 && offset < 0x18ada:
		return a.queue()[offset-0x185a8], nil
	case offset >= 0x18426 && offset < 0x1842e:
		index := int(offset) - 0x18426
		return uint8(a.Audio.Channels[index/2] >> uint(8-(index&1)*8)), nil
	case offset >= 0xeee0 && offset < 0xeee2:
		return uint8(a.Image.LastY >> uint(8-(int(offset)-0xeee0)*8)), nil
	default:
		return a.backing.Read8(at)
	}
}

func (a *NativeImageAudioCodeAlias) writeByte(at int, v uint8) error {
	if err := a.backing.Write8(at, v); err != nil {
		return err
	}
	offset := int64(at) - int64(a.CodeBase)
	switch {
	case offset >= 0x185a8 && offset < 0x18ada:
		a.queue()[offset-0x185a8] = v
	case offset >= 0x18426 && offset < 0x1842e:
		index := int(offset) - 0x18426
		word := &a.Audio.Channels[index/2]
		shift := uint(8 - (index&1)*8)
		*word = *word & ^(uint16(255)<<shift) | uint16(v)<<shift
	case offset >= 0xeee0 && offset < 0xeee2:
		shift := uint(8 - (int(offset)-0xeee0)*8)
		a.Image.LastY = a.Image.LastY & ^(uint16(255)<<shift) | uint16(v)<<shift
	}
	return nil
}

// NewNativeSharedAudioDevice borrows the actual initialized host allocation,
// retaining all prior CODE mutations. Call it before the source Initialize;
// it never imports a private device's already-mutated copy over startup RAM.
// FX must be the caller's actual decoded allocation, not encoded I/O. Its
// resource header bounds validation; the larger original allocation remains
// borrowed so native address reads retain genuine adjacent allocated bytes.
// The caller must serialize device/PCM, frame and physical-memory operations
// sharing this allocation. NativeAudioPCM's own mutex does not protect other
// owners of CODE and cannot substitute for that host ownership boundary.
func NewNativeSharedAudioDevice(exe *amiga.Executable, fx []byte, code *NativeSharedCode, resourceBase uint32, timerLow uint8) (*NativeAudioDevice, error) {
	if exe == nil || len(exe.Hunks) == 0 || code == nil || len(code.RawData()) < 0x3e40c || uint64(len(code.RawData())) != exe.Hunks[0].AllocatedBytes {
		return nil, fmt.Errorf("native shared audio actual CODE allocation missing")
	}
	if len(fx) < 4 || uint64(binary.BigEndian.Uint32(fx[:4]))+4 > uint64(len(fx)) {
		return nil, fmt.Errorf("native shared audio decoded FX exceeds actual allocation")
	}
	resourceBytes := uint64(binary.BigEndian.Uint32(fx[:4])) + 4
	if _, err := DecodeAudioBank(exe, fx[:int(resourceBytes)]); err != nil {
		return nil, err
	}
	return &NativeAudioDevice{Code: code.RawData(), FX: fx, CodeBase: code.CodeBase, ResourceBase: resourceBase, TimerLow: timerLow}, nil
}
