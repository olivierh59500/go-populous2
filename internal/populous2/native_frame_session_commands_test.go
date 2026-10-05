package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// The reference supplies controlled child returns at real native modal and
// resource boundaries. These compare the session's wrapper and continuation;
// they do not claim implementations of the unsupplied child bodies.
func TestNativeSessionCommandEnvelopesAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/command_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []commandFrameFixture }
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 1152 {
		t.Fatal("native session command corpus incomplete")
	}
	accepted, external := 0, 0
	for _, f := range corpus.Cases {
		// $dc4 is the independent scenario-script record, not either of
		// $1744c's two player records; its command body has its own proof.
		if f.Input.Caller != 0xeb56 && f.Input.Caller != 0xeb60 {
			external++
			continue
		}
		accepted++
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%t", f.Input.Name, pending), func(t *testing.T) {
				initial := commandFrameInitial(f.Input)
				w := aiFixtureWorld(t, initial)
				s, err := NewNativeFrameSession(testBundle(t), 0, 0x500000, 0x400000)
				if err != nil {
					t.Fatal(err)
				}
				m := s.Presentation.Memory(w.nativeCleanupMemory())
				for at := 0; at < 0xdc2; at++ {
					if err := m.Write8(at, initial[at]); err != nil {
						t.Fatal(err)
					}
				}
				binary.BigEndian.PutUint16(w.NativeAI.Code[0x3f90:], f.Input.Code3F90)
				binary.BigEndian.PutUint16(w.NativeAI.Code[0x4468:], f.Input.Code4468)
				s.Presentation.Input.Mouse.Image = f.Input.CodeA2A
				w.nativeCallDepth++
				if err := s.Begin(w, NativeFrameRegisterContext{D: f.Input.D}); err != nil {
					t.Fatal(err)
				}
				defer func() { s.finish(nil); w.nativeCallDepth-- }()
				image := func() []byte {
					all := aiFixtureWorldBytes(w, initial)
					copy(all[0xeb90:], w.NativeRedrawBytes[:])
					for at := 0; at < 0xdc2; at++ {
						v, err := m.Read8(at)
						if err != nil {
							t.Fatal(err)
						}
						all[at] = v
					}
					return all
				}
				index, entered := 0, 0
				cb := NativeFrameSessionCallbacks{CommandChild: func(call NativeCommandFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if index >= len(f.Calls) {
						t.Fatal("extra native child call")
					}
					want := f.Calls[index]
					if *phase == 0 {
						entered++
						if uint32(call.Routine) != want.Routine || call.Context.D != want.D || call.Caller != f.Input.Caller {
							t.Fatal("session child input differs from source")
						}
						if call.Routine == 0x102e4 && (call.PaletteA2+0x100000 != want.A2 || call.PaletteA3+0x100000 != want.A3) {
							t.Fatal("native palette pointers differ")
						}
						if call.Routine == 0xd8cc && call.TargetA0 != want.A0 {
							t.Fatal("native frozen target differs")
						}
						if got := fmt.Sprintf("%x", sha256.Sum256(image())); got != want.Hash {
							t.Fatal("native session child BSS prefix differs", got, want.Hash)
						}
						if pending {
							*phase = 1
							return NativeCommandFrameResult{Zero: true}, nil
						}
					}
					// Same declared return transform as the CPU oracle, independently
					// applied to the live caller; no expected register array replay.
					for reg := range call.Context.D {
						call.Context.D[reg] ^= f.Input.ReturnSeed + uint32(index)*0x10101 + uint32(reg)*0x111111
					}
					index++
					return NativeCommandFrameResult{Complete: true, Zero: f.Input.Zero}, nil
				}}
				c := NativeCommandRegisterContext{D: f.Input.D}
				phase := uint32(0)
				execute := s.commandExecutor(cb, NativeCommandWorldBindings{})
				done := false
				for resume := 0; resume < 16 && !done; resume++ {
					done, err = execute(f.Input.Caller, &c, &phase)
					if err != nil {
						t.Fatal(err)
					}
				}
				if !done || c.D != f.D || index != len(f.Calls) || entered != len(f.Calls) {
					t.Fatal("native session command did not preserve complete continuation")
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(image())); got != f.Hash {
					t.Fatal("native session final BSS differs", got, f.Hash)
				}
				for at, want := range map[int]uint16{0x3f90: f.Code3F90, 0x4468: f.Code4468, 0xa2a: f.CodeA2A} {
					got, err := s.commandCodeWord(at)
					if err != nil || got != want {
						t.Fatal("native session mutable CODE differs", at, got, want, err)
					}
				}
			})
		}
	}
	if accepted != 768 || external != 384 {
		t.Fatal("native player/script caller coverage differs", accepted, external)
	}
}

func TestNativeSessionResumesActualFileCommandBeforeSecondSide(t *testing.T) {
	w, s := nativeSessionTestSetup(t)
	m := s.Presentation.Memory(w.nativeCleanupMemory())
	_ = m.Write8(0xeb56, 1)
	_ = m.Write8(0xeb57, 108)
	_ = m.Write8(0xeb5e, 2)
	_ = m.Write8(0xeb60, 2)
	_ = m.Write8(0xeb61, 120)
	_ = m.Write16(0xeb62, 0x1234)
	_ = m.Write8(0xeb68, 2)
	god := 0xe76a + 2*314
	_ = m.Write32(god, 1000)
	s.Presentation.Input.setWord(0xa, 1)
	input := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	if err := s.Begin(w, input); err != nil {
		t.Fatal(err)
	}
	// The outer bridge has completed; native callers now own live raw mana.
	_ = m.Write32(god, 1000)
	renders, children := 0, 0
	cb := NativeFrameSessionCallbacks{Render: func(_ FollowerCleanupMemory, _ *NativeFrameRegisterContext, _ *NativeImageRenderState, _ []byte, _ *uint32) (bool, error) {
		renders++
		return true, nil
	},
		CommandChild: func(call NativeCommandFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
			children++
			if call.Routine != 0x3f92 || call.Caller != 0xeb56 {
				t.Fatal("wrong native file requester child", call.Routine, call.Caller)
			}
			if *phase == 0 {
				*phase = 1
				call.Context.D[7] = 0xcafebabe
				return NativeCommandFrameResult{Zero: true}, nil
			}
			if call.Context.D[7] != 0xcafebabe {
				t.Fatal("pending file requester lost full child context")
			}
			return NativeCommandFrameResult{Complete: true}, nil
		},
	}
	done, err := s.Advance(cb)
	if err != nil || done || s.Deferred.Side != 0 || s.Pass.Stage != NativeFrameCommands {
		t.Fatal("native file requester did not suspend first side", done, err)
	}
	if value, _ := m.Read32(god); value != 1000 {
		t.Fatal("second side ran before first menu completed")
	}
	if command, _ := m.Read8(0xeb57); command != 108 {
		t.Fatal("pending source file requester command cleared")
	}
	done, err = s.Advance(cb)
	if err != nil || !done || renders != 1 || children != 2 || s.Presentation.CopperSelector != 4 || w.nativeCallDepth != 0 {
		t.Fatal("native file requester resumed incorrectly", done, err)
	}
	if value, _ := m.Read32(god); value != 1000+0x1234 {
		t.Fatal("second side real command repeated or omitted", value)
	}
	if s.Frame.D[7] != input.D[7] {
		t.Fatal("outer1744C saved register lost")
	}
}
