package populous2

import (
	"encoding/binary"
	"fmt"
)

type NativeFrameWorldBindings struct {
	// MapPoint supplies the real overview bitmap writer at $15b62.
	MapPoint func(*NativeFrameRegisterContext) error
	// Audio owns the original mutable software descriptors. A visible
	// Whirlpool must increment its flag before the later audio pass.
	Audio *NativeFrameAudioState
	// FXBody is required for effect families whose full source-register
	// controller has not yet been installed in this adapter.
	FXBody   func(NativeRecordReference, *NativeFrameRegisterContext) (NativeFrameFXStep, error)
	Commands NativeCommandWorldBindings
}

func (w *World) nativeFrameFXCallbacks(bindings NativeFrameWorldBindings) NativeFrameFXCallbacks {
	return NativeFrameFXCallbacks{Memory: w.nativeCleanupMemory(), MapPoint: bindings.MapPoint, Tick: func(ref NativeRecordReference, c *NativeFrameRegisterContext) (NativeFrameFXStep, error) {
		at := cleanupRecordAddress(ref)
		state, e := w.nativeCleanupMemory().Read8(at + 22)
		if e != nil {
			return NativeFrameFXStep{}, e
		}
		switch state {
		case 0:
			return NativeFrameFXStep{}, nil
		case 2, 4, 6:
			cb := w.nativeFireColumnCallbacks()
			cb.Frame = c
			step, e := w.NativeFireColumn.Tick(ref, cb)
			return NativeFrameFXStep{Draw: !step.Removed, Color: 5}, e
		case 8, 10, 12:
			cb := w.nativeWhirlwindCallbacks()
			cb.Frame = c
			cb.SourceD2 = uint16(c.D[2])
			step, e := w.NativeWhirlwind.Tick(ref, cb)
			return NativeFrameFXStep{Draw: !(step.Expired && step.Removed), Color: 5}, e
		case 0x0e, 0x10:
			return w.Whirlpools.TickFrameWhirlpool(ref, c, NativeFrameWhirlpoolCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Lower: func(context *NativeFrameRegisterContext) error {
				command := context.CommandContext()
				_, e := w.commandDirectTerrain(NativeCommandCall{Routine: 0xd7f0, Context: &command}, false)
				context.SetCommandContext(command)
				return e
			}, Sound: func() error {
				if bindings.Audio == nil {
					return fmt.Errorf("native Whirlpool frame audio descriptors missing")
				}
				at := 125 * 10
				binary.BigEndian.PutUint16(bindings.Audio.Entries[at:], binary.BigEndian.Uint16(bindings.Audio.Entries[at:])+1)
				if len(w.effectSoundCues) < NativeEffectCapacity {
					w.effectSoundCues = append(w.effectSoundCues, 125)
				}
				return nil
			}})
		case 0x3c:
			rules := NativeCommandRules{Code: w.NativeAI.Code}
			return rules.TickFrameHurricane(ref, c, NativeFrameHurricaneCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Move: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
				command := context.CommandContext()
				e := w.commandMove(cleanupRecordAddress(ref), &command)
				context.SetCommandContext(command)
				return e
			}, Cleanup: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
				_, e := CleanupFollowerWithFrame(ref, context, FollowerCleanupCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
				return e
			}})
		case 0x30, 0x32:
			rules := NativeCommandRules{Code: w.NativeAI.Code}
			return rules.TickFrameVolcano(ref, c, NativeFrameVolcanoCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Terrain: func(raise bool, context *NativeFrameRegisterContext) error {
				command := context.CommandContext()
				routine := 0xd7f0
				if raise {
					routine = 0xd81e
				}
				_, e := w.commandDirectTerrain(NativeCommandCall{Routine: routine, Context: &command}, raise)
				context.SetCommandContext(command)
				return e
			}, FireColumn: func(context *NativeFrameRegisterContext) error {
				command := context.CommandContext()
				_, e := w.nativeNormalCommandCallbacks(bindings.Commands).Call(NativeCommandCall{Routine: 0x15b7c, Context: &command})
				context.SetCommandContext(command)
				return e
			}, Lava: func(context *NativeFrameRegisterContext) error {
				return rules.CreateFrameLava(context, w.frameLavaCallbacks())
			}})
		case 0x34:
			rules := NativeCommandRules{Code: w.NativeAI.Code}
			return rules.TickFrameLava(ref, c, w.frameLavaCallbacks())
		case 0x22, 0x24, 0x26:
			rules := NativeCommandRules{Code: w.NativeAI.Code}
			return rules.TickFrameQuake(ref, c, NativeFrameQuakeCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Lower: func(context *NativeFrameRegisterContext) error {
				command := context.CommandContext()
				_, e := w.commandDirectTerrain(NativeCommandCall{Routine: 0xd7f0, Context: &command}, false)
				context.SetCommandContext(command)
				return e
			}, Create: w.frameQuakeCreate, Sound: func() error {
				if bindings.Audio == nil {
					return fmt.Errorf("native Earthquake frame audio descriptors missing")
				}
				at := 105 * 10
				binary.BigEndian.PutUint16(bindings.Audio.Entries[at:], binary.BigEndian.Uint16(bindings.Audio.Entries[at:])+1)
				if len(w.effectSoundCues) < NativeEffectCapacity {
					w.effectSoundCues = append(w.effectSoundCues, 105)
				}
				return nil
			}})
		case 0x1c, 0x1e, 0x20:
			cb := w.fireRainCallbacks()
			cb.Frame = c
			step, e := w.FireRainRules.Tick(ref, cb)
			return NativeFrameFXStep{Draw: !step.NextActor, Color: 5}, e
		case 0x16, 0x18, 0x1a:
			rules := NativeCommandRules{Code: w.NativeAI.Code}
			return rules.TickFrameLightning(ref, c, NativeFrameLightningCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Unlink: w.nativeRuntimeUnlink, Dismiss: func(context *NativeFrameRegisterContext) error {
				command := context.CommandContext()
				_, e := w.commandLightning(NativeCommandCall{Routine: 0x15f80, Context: &command})
				context.SetCommandContext(command)
				return e
			}, Scorch: func(ref NativeRecordReference) error { return w.StormRules.Scorch(ref, w.nativeCleanupMemory()) }})
		case 0x36, 0x38, 0x3a:
			rules := NativeCommandRules{Code: w.NativeAI.Code}
			return rules.TickFrameBasalt(ref, c, NativeFrameBasaltCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Create: func(context *NativeFrameRegisterContext) error {
				command := context.CommandContext()
				_, e := w.commandBasaltCreation(NativeCommandCall{Routine: 0x171ea, Context: &command})
				context.SetCommandContext(command)
				return e
			}})
		case 0x28, 0x2a:
			cb := w.tsunamiCallbacks()
			cb.Frame = c
			cb.LowerFrame = func(context *NativeFrameRegisterContext) error {
				command := context.CommandContext()
				_, e := w.commandDirectTerrain(NativeCommandCall{Routine: 0xd7f0, Context: &command}, false)
				context.SetCommandContext(command)
				return e
			}
			step, e := w.TsunamiRules.Tick(ref, cb)
			return NativeFrameFXStep{Draw: !step.Removed, Color: 5}, e
		case 0x2c, 0x2e:
			cb := w.stormCallbacks()
			cb.Frame = c
			step, e := w.StormRules.Tick(ref, cb)
			return NativeFrameFXStep{Draw: !step.Removed, Color: 5}, e
		case 0x12, 0x14:
			cb := w.nativeFungusCallbacks()
			cb.Frame = c
			step, e := w.NativeFungus.Tick(ref, cb)
			return NativeFrameFXStep{Draw: !step.Generated, Color: 5}, e
		default:
			if bindings.FXBody != nil {
				return bindings.FXBody(ref, c)
			}
			return NativeFrameFXStep{}, fmt.Errorf("native FX frame state%02x register controller missing", state)
		}
	}}
}

func (w *World) tickNativeFrameFX(context *NativeFrameRegisterContext, bindings NativeFrameWorldBindings) error {
	rules := NativeCommandRules{Code: w.NativeAI.Code}
	return rules.TickFrameFX(context, w.nativeFrameFXCallbacks(bindings))
}

// frameQuakeCreate retains the returned A1 address from$165da in addition to
// its already proven data-register and memory result.
func (w *World) frameQuakeCreate(parent int, context *NativeFrameRegisterContext) (int, error) {
	address := parent
	x, y := int(int8(uint8(context.D[0]))), int(int8(uint8(context.D[1])))
	if inside(x, y) {
		var e error
		address, e = primitiveFreeRecord(w.nativeCleanupMemory(), 0xc800, 0xe740, 32)
		if e != nil {
			return address, e
		}
		if address == 0 {
			address = 0xe740
		}
	}
	command := context.CommandContext()
	_, e := w.commandEarthquakeCreation(NativeCommandCall{Routine: 0x165da, Caller: parent, Context: &command})
	context.SetCommandContext(command)
	return address, e
}

func (w *World) frameLavaCallbacks() NativeFrameLavaCallbacks {
	return NativeFrameLavaCallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Link: w.nativeRuntimeInsert, Unlink: w.nativeRuntimeUnlink, Move: func(ref NativeRecordReference, context *NativeFrameRegisterContext) error {
		command := context.CommandContext()
		e := w.commandMove(cleanupRecordAddress(ref), &command)
		context.SetCommandContext(command)
		return e
	}, Basalt: func(context *NativeFrameRegisterContext) error {
		command := context.CommandContext()
		_, e := w.commandBasaltCreation(NativeCommandCall{Routine: 0x171ea, Context: &command})
		context.SetCommandContext(command)
		return e
	}, Scorch: func(ref NativeRecordReference) error { return w.StormRules.Scorch(ref, w.nativeCleanupMemory()) }, DestroyTown: w.nativeDestroyTown}
}
