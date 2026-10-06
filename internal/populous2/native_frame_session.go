package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type NativeFrameSessionPhase uint8

const (
	NativeFrameSessionIdle NativeFrameSessionPhase = iota
	NativeFrameSessionVBlank
	NativeFrameSessionClock
	NativeFrameSessionRender
	NativeFrameSessionPhysics
	NativeFrameSessionMenu
	NativeFrameSessionInput
)

// NativeFrameSession retains one original frame across clock, render, serial
// and modal waits. It owns the raw World until the entire frame completes;
// typed views are hydrated once at completion, never before a resumed child.
type NativeFrameSession struct {
	Presentation  *NativeFramePresentationState
	Frame         NativeFrameRegisterContext
	Phase         NativeFrameSessionPhase
	Pass          NativeFramePassState
	Deferred      NativeDeferredFrameState
	Followers     NativeFollowerFrameState
	Image         NativeImageRenderState
	Audio         NativeFrameAudioState
	RenderPhase   uint32
	MenuPhase     uint32
	ExitRequested bool
	entryChecked  bool
	exitChecked   bool

	followerRules        *NativeFollowerFrameRules
	wallRules            NativeWallRules
	wallPlacement        NativeWallPlacementState
	world                *World
	outerBorrow          bool
	failed               error
	land                 []byte
	bitmapResolver       func(uint32) ([]byte, error)
	previousTerrainPoint func(*NativeCommandRegisterContext) error
	palette              *NativeFramePaletteState
	previousDirectSound  func(uint16) error
	directSound          func(uint16) error
	commandRules         NativeCommandRules
	commandStates        [2]NativeCommandFrameState
	commandPalettes      [2]*NativeFramePaletteState
	imageAudioCode       *NativeImageAudioCodeAlias
	inputPhase           uint32
	requireInput         bool
	followersCompleted   bool
	resultPending        bool
	resultIdentity       uint16
}

type NativeFrameSessionCallbacks struct {
	// Clock and Render perform genuine source children and retain any inner
	// phase on a wait. Render receives the real HUNK4 drawing target and the
	// shared image descriptor bank. There is no default successful renderer.
	Palette       func(*NativeFrameRegisterContext) (bool, error)
	Render        func(FollowerCleanupMemory, *NativeFrameRegisterContext, *NativeImageRenderState, []byte, *uint32) (bool, error)
	Result        func(uint16, *NativeFrameRegisterContext) error
	ResultAdvance func(uint16, *NativeFrameRegisterContext) (bool, error)
	Audio         NativeFrameAudioCallbacks
	Execute       func(int, *NativeCommandRegisterContext, *uint32) (bool, error)
	Transport     func(int, uint8, *NativeCommandRegisterContext, *uint32) (bool, error)
	Commands      NativeCommandWorldBindings
	// DirectSound performs register-preserving184F6. It is a real device
	// operation and must not increment the software scheduler's flags.
	DirectSound func(uint16) error
	// Bitmap resolves source numeric pointers such as persistent terrain
	// target$22. It must supply that real buffer, not the overview target$1e.
	Bitmap func(uint32) ([]byte, error)
	// CommandChild owns real modal/startup/resource children that remain
	// inside the original17500 command. Completion and CCR.Z stay separate.
	CommandChild func(NativeCommandFrameCall, *uint32) (NativeCommandFrameResult, error)
	// Menu is the real446A call before the source786 VBlank gate. It may
	// wait without replaying the DCE clear or releasing raw World ownership.
	Menu func(FollowerCleanupMemory, *NativeFrameRegisterContext, *NativeImageRenderState, *uint32) (bool, error)
	// Input runs the original110E suffix after commands. It may retain a
	// real child wait; the raw World is borrowed until this suffix returns.
	Input func(FollowerCleanupMemory, *NativeFrameRegisterContext, *uint32) (bool, error)
}

func NewNativeFrameSession(bundle *Bundle, landIndex int, chipBase, pointerBase uint32) (*NativeFrameSession, error) {
	followers, err := DecodeNativeFollowerFrameRules(bundle, landIndex)
	if err != nil {
		return nil, err
	}
	presentation, err := NewNativeFramePresentationState(bundle.Executable, chipBase, pointerBase)
	if err != nil {
		return nil, err
	}
	wall, err := DecodeNativeWallRules(bundle.Executable)
	if err != nil {
		return nil, err
	}
	images, err := DecodeNativeEditorCursorRules(bundle.Executable)
	if err != nil {
		return nil, err
	}
	audio, err := DecodeNativeFrameAudioState(bundle.Executable)
	if err != nil {
		return nil, err
	}
	commandRules, err := DecodeNativeCommandRules(bundle.Executable)
	if err != nil {
		return nil, err
	}
	return &NativeFrameSession{Presentation: presentation, followerRules: followers, wallRules: wall,
		Followers: *followers.NewState(), Image: images.NewImageState(), Audio: audio,
		land: append([]byte(nil), bundle.Raw[fmt.Sprintf("land%d.dat", landIndex)]...), commandRules: commandRules}, nil
}

// Begin accepts the actual incoming caller registers. Initialization of the
// real Copper buffers and initial World/presentation is owned by startup.
func (s *NativeFrameSession) Begin(w *World, input NativeFrameRegisterContext) error {
	return s.begin(w, input, false)
}

// BeginRaw accepts authoritative bytes produced by the actual startup or
// loader. It does not flush inherited typed actors back over those records.
// Complete frame ownership and final hydration remain identical to Begin.
func (s *NativeFrameSession) BeginRaw(w *World, input NativeFrameRegisterContext) error {
	return s.begin(w, input, true)
}

func (s *NativeFrameSession) begin(w *World, input NativeFrameRegisterContext, raw bool) error {
	if s == nil || w == nil || s.Presentation == nil || s.followerRules == nil {
		return fmt.Errorf("native frame session backing missing")
	}
	if s.Phase != NativeFrameSessionIdle || s.world != nil {
		return fmt.Errorf("native frame already in progress")
	}
	if s.failed != nil {
		return s.failed
	}
	if w.Landscape != s.followerRules.landscape {
		return fmt.Errorf("native frame session LAND differs from World")
	}
	if len(w.NativeAI.Code) < 0x33886 {
		return fmt.Errorf("native frame CODE resource backing missing")
	}
	if !bytes.Equal(w.NativeAI.Code[0x3365a:0x33886], s.land) {
		if err := w.retainNativeLAND(s.land); err != nil {
			return err
		}
	}
	s.outerBorrow = w.nativeCallDepth == 0
	if s.outerBorrow && !raw {
		w.reconcileActorGraph()
		w.refreshNativeRecordImage()
		w.syncNativeRuntimeBridge()
	}
	w.nativeCallDepth++
	s.world = w
	s.previousTerrainPoint = w.nativeTerrainPoint
	s.previousDirectSound = w.nativeDirectSound
	w.nativeDirectSound = func(raw uint16) error {
		resource, err := s.Presentation.Memory(w.nativeCleanupMemory()).Read32(0x3b4)
		if err != nil {
			return err
		}
		if int32(resource) <= 0 {
			return nil
		} // Original190E4 disabled RTS.
		if s.directSound == nil {
			return fmt.Errorf("initialized native direct audio device callback missing")
		}
		return s.directSound(raw)
	}
	w.nativeTerrainPoint = func(c *NativeCommandRegisterContext) error {
		memory := s.Presentation.Memory(w.nativeCleanupMemory())
		address, err := memory.Read32(0x22)
		if err != nil {
			return err
		}
		bitmap, err := s.bitmapAt(address)
		if err != nil {
			return err
		}
		frame := NativeFrameRegisterContext{D: c.D, AddressBase: s.Frame.AddressBase}
		point, err := PlanNativeMapPoint(&frame)
		if err != nil {
			return err
		}
		c.D = frame.D
		return point.Paint(bitmap)
	}
	s.Frame = input
	s.Pass = NativeFramePassState{}
	s.followersCompleted, s.resultPending = false, false
	s.Deferred = NativeDeferredFrameState{}
	s.commandStates = [2]NativeCommandFrameState{}
	s.commandPalettes = [2]*NativeFramePaletteState{}
	s.RenderPhase = 0
	s.inputPhase = 0
	s.MenuPhase = 0
	s.ExitRequested = false
	s.entryChecked = false
	s.exitChecked = false
	s.palette = nil
	s.Phase = NativeFrameSessionVBlank
	return nil
}

func (s *NativeFrameSession) finish(err error) {
	if s.world != nil {
		s.world.nativeTerrainPoint = s.previousTerrainPoint
		s.world.nativeDirectSound = s.previousDirectSound
		s.world.nativeCallDepth--
		if s.outerBorrow {
			s.world.hydrateNativeRuntimeRecords()
		}
	}
	s.world = nil
	s.previousTerrainPoint = nil
	s.previousDirectSound = nil
	s.directSound = nil
	s.bitmapResolver = nil
	s.outerBorrow = false
	s.Phase = NativeFrameSessionIdle
	s.failed = err
}

// Advance never repeats completed clock, render, physics, swap or command
// work. A callback failure ends this session; the mutated native prefix is
// retained for diagnosis instead of silently retried or rolled back.
func (s *NativeFrameSession) Advance(cb NativeFrameSessionCallbacks) (bool, error) {
	if s == nil || s.world == nil || s.Phase == NativeFrameSessionIdle {
		return false, fmt.Errorf("native frame session not started")
	}
	w := s.world
	s.bitmapResolver = cb.Bitmap
	s.directSound = cb.DirectSound
	memory := s.Presentation.Memory(w.nativeCleanupMemory())
	fail := func(err error) (bool, error) { s.finish(err); return false, err }
	ready, exit, err := s.advanceFrameEntry(memory, cb)
	if err != nil {
		return fail(err)
	}
	if exit {
		s.ExitRequested = true
		s.finish(nil)
		return true, nil
	}
	if !ready {
		return false, nil
	}

	if s.Phase == NativeFrameSessionVBlank {
		if s.Presentation.Input.word(0xa) == 0 {
			return false, nil
		}
		s.Phase = NativeFrameSessionClock
	}
	if s.Phase == NativeFrameSessionClock {
		palette := cb.Palette
		if palette == nil {
			palette = s.advancePalette
		}
		done, err := s.Presentation.AdvanceClock(&s.Frame, NativeFrameClockCallbacks{Memory: memory, Palette: palette})
		if err != nil {
			return fail(err)
		}
		if !done {
			return false, nil
		}
		s.Phase = NativeFrameSessionRender
	}
	bitmap, err := s.Presentation.BackBuffer()
	if err != nil {
		return fail(err)
	}
	if s.Phase == NativeFrameSessionRender {
		if cb.Render == nil {
			return fail(fmt.Errorf("native main render continuation missing"))
		}
		done, err := cb.Render(memory, &s.Frame, &s.Image, bitmap, &s.RenderPhase)
		if err != nil {
			return fail(err)
		}
		if !done {
			return false, nil
		}
		// EE32 and182CE use the same CODE bank. Transfer authority once at
		// this boundary; copying again during physics would erase queued FX.
		s.Audio.Entries = s.Image.AudioBank
		if s.imageAudioCode != nil {
			if err := s.imageAudioCode.SetOwner(NativeAudioCodeOwner); err != nil {
				return fail(err)
			}
		}
		s.Phase = NativeFrameSessionPhysics
		// A real renderer modal may swap the screens before returning. The
		// physics suffix must use the current native drawing pointer too.
		bitmap, err = s.Presentation.BackBuffer()
		if err != nil {
			return fail(err)
		}
	}
	done := true
	if s.Phase != NativeFrameSessionInput {
		done, err = s.Pass.TickFramePass(&s.Frame, s.physicsCallbacks(cb, bitmap))
	}
	if err != nil {
		return fail(err)
	}
	if !done {
		return false, nil
	}
	if s.requireInput && cb.Input == nil {
		return fail(fmt.Errorf("native runtime gameplay input suffix missing"))
	}
	if cb.Input != nil {
		s.Phase = NativeFrameSessionInput
		done, err = cb.Input(memory, &s.Frame, &s.inputPhase)
		if err != nil {
			return fail(err)
		}
		if !done {
			return false, nil
		}
	}
	s.finish(nil)
	return true, nil
}

// advancePalette supplies $110cc's actual $3361a→$33844 fade. The latter
// words are read from the currently loaded LAND bank, not the bundle's LAND0.
func (s *NativeFrameSession) advancePalette(c *NativeFrameRegisterContext) (bool, error) {
	if s.palette == nil {
		source, target := NativeFramePaletteBank{Address: 0x3361a}, NativeFramePaletteBank{Address: 0x33844}
		for i := range source.Words {
			source.Words[i] = binary.BigEndian.Uint16(s.world.NativeAI.Code[0x3361a+i*2:])
			target.Words[i] = binary.BigEndian.Uint16(s.world.NativeAI.Code[0x33844+i*2:])
		}
		s.palette = NewNativeFramePaletteState(source, target, 0)
	}
	return s.palette.Advance(s.Presentation, c, s.Presentation.Memory(s.world.nativeCleanupMemory()))
}

func (s *NativeFrameSession) physicsCallbacks(cb NativeFrameSessionCallbacks, bitmap []byte) NativeFrameCallbacks {
	w := s.world
	memory := s.Presentation.Memory(w.nativeCleanupMemory())

	commands := cb.Commands
	commands.WallRules = &s.wallRules
	commands.WallPlacement = &s.wallPlacement
	paint := func(c *NativeFrameRegisterContext) error {
		point, err := PlanNativeMapPoint(c)
		if err != nil {
			return err
		}
		return point.Paint(bitmap)
	}
	device := NativeFrameAudioCallbacks{Command: func(control, data uint16, input uint32) (uint32, error) {
		resource, err := memory.Read32(0x3b4)
		if err != nil {
			return 0, err
		}
		if int32(resource) <= 0 {
			return input, nil
		}
		if cb.Audio.Command == nil {
			return 0, fmt.Errorf("initialized native scheduled audio device callback missing")
		}
		return cb.Audio.Command(control, data, input)
	}}
	bindings := NativeFrameContinuationBindings{World: NativeFrameWorldBindings{Audio: &s.Audio, MapPoint: paint, Commands: commands}, Audio: device,
		Followers: func(c *NativeFrameRegisterContext) (bool, error) {
			if cb.ResultAdvance == nil || !s.followersCompleted {
				result := cb.Result
				if cb.ResultAdvance != nil {
					result = func(identity uint16, _ *NativeFrameRegisterContext) error {
						s.resultPending = true
						s.resultIdentity = identity
						return nil
					}
				}
				err := s.followerRules.Tick(w, c, &s.Followers, NativeFollowerFrameOutputs{MapPoint: func(_ uint16, c *NativeFrameRegisterContext) error { return paint(c) }, Result: result, Commands: commands})
				if err != nil {
					return false, err
				}
				s.followersCompleted = cb.ResultAdvance != nil
			}
			if s.resultPending {
				if cb.ResultAdvance == nil {
					return false, fmt.Errorf("native retained result callback missing")
				}
				done, err := cb.ResultAdvance(s.resultIdentity, c)
				if err != nil || !done {
					return false, err
				}
				s.resultPending = false
			}
			return true, nil
		},
		Swap: func(c *NativeFrameRegisterContext) (bool, error) {
			// Audio has completed (or the pause gate skipped it). Transfer
			// authority before a deferred UI child can draw through EE32.
			s.Image.AudioBank = s.Audio.Entries
			if s.imageAudioCode != nil {
				if err := s.imageAudioCode.SetOwner(NativeImageCodeOwner); err != nil {
					return false, err
				}
			}
			_, err := s.Presentation.Swap(c)
			return err == nil, err
		},
		Commands: func(c *NativeFrameRegisterContext) (bool, error) {
			execute := cb.Execute
			if execute == nil {
				execute = s.commandExecutor(cb, commands)
			}
			return s.Deferred.TickDeferredFrame(c, NativeDeferredFrameCallbacks{Memory: memory, Execute: execute, Transport: cb.Transport})
		},
	}
	return w.nativeFrameCallbacks(bindings)
}

func (s *NativeFrameSession) bitmapAt(address uint32) ([]byte, error) {
	if at, err := s.Presentation.chipAt(address, 32000); err == nil {
		return s.Presentation.Chip[at : at+32000], nil
	}
	if s.bitmapResolver != nil {
		return s.bitmapResolver(address)
	}
	return nil, fmt.Errorf("native bitmap target%x backing missing", address)
}
