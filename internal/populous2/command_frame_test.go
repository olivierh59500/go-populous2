package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type commandFrameInput struct {
	Name                                        string
	Command, Owner                              uint8
	Caller                                      int
	Pause, Terrain, Code3F90, Code4468, CodeA2A uint16
	Seed, SavedSeed, Target                     uint32
	D                                           [8]uint32
	ReturnSeed                                  uint32
	Zero                                        bool
}
type commandFrameFixture struct {
	Input                       commandFrameInput
	D                           [8]uint32
	Caller                      uint32
	Hash                        string
	ErrorPC                     uint32
	Code3F90, Code4468, CodeA2A uint16
	Calls                       []struct {
		Routine                     uint32
		D, Output                   [8]uint32
		A0, A2, A3                  uint32
		Hash                        string
		Code3F90, Code4468, CodeA2A uint16
		Zero                        bool
	}
}

func commandFrameInitial(c commandFrameInput) []byte {
	b := make([]byte, 0x11280)
	b[c.Caller], b[c.Caller+1], b[c.Caller+8] = c.Owner, c.Command, 2
	binary.BigEndian.PutUint16(b[c.Caller+2:], 0x1234)
	binary.BigEndian.PutUint16(b[0xf3c:], c.Pause)
	binary.BigEndian.PutUint16(b[0xeb22:], c.Terrain)
	binary.BigEndian.PutUint32(b[0xeb28:], c.Seed)
	binary.BigEndian.PutUint32(b[0xeb24:], c.SavedSeed)
	binary.BigEndian.PutUint32(b[0x22:], c.Target)
	return b
}
func commandFrameBacking(b []byte) FollowerCleanupMemory {
	check := func(at, size int) error {
		if at < 0 || at > len(b)-size {
			return fmt.Errorf("test sourceBSS address%x unavailable", at)
		}
		return nil
	}
	return FollowerCleanupMemory{Read8: func(at int) (uint8, error) {
		if e := check(at, 1); e != nil {
			return 0, e
		}
		return b[at], nil
	}, Read16: func(at int) (uint16, error) {
		if e := check(at, 2); e != nil {
			return 0, e
		}
		return binary.BigEndian.Uint16(b[at:]), nil
	}, Read32: func(at int) (uint32, error) {
		if e := check(at, 4); e != nil {
			return 0, e
		}
		return binary.BigEndian.Uint32(b[at:]), nil
	}, Write8: func(at int, v uint8) error {
		if e := check(at, 1); e != nil {
			return e
		}
		b[at] = v
		return nil
	}, Write16: func(at int, v uint16) error {
		if e := check(at, 2); e != nil {
			return e
		}
		binary.BigEndian.PutUint16(b[at:], v)
		return nil
	}, Write32: func(at int, v uint32) error {
		if e := check(at, 4); e != nil {
			return e
		}
		binary.BigEndian.PutUint32(b[at:], v)
		return nil
	}}
}

func TestNativeCommandFramePrefixesAndReturnsAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/command_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []commandFrameFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 1152 {
		t.Fatal("native UI command frame coverage incomplete")
	}
	rules, e := DecodeNativeCommandRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, pending), func(t *testing.T) {
				b := commandFrameInitial(f.Input)
				memory := commandFrameBacking(b)
				code := map[int]uint16{0x3f90: f.Input.Code3F90, 0x4468: f.Input.Code4468, 0xa2a: f.Input.CodeA2A}
				c := NativeCommandRegisterContext{D: f.Input.D}
				state := NativeCommandFrameState{}
				index, entered := 0, 0
				cb := NativeCommandFrameCallbacks{Immediate: NativeCommandCallbacks{Memory: memory}, ReadCode16: func(at int) (uint16, error) {
					v, ok := code[at]
					if !ok {
						return 0, fmt.Errorf("unavailable testCODEword%x", at)
					}
					return v, nil
				}, WriteCode16: func(at int, v uint16) error {
					if _, ok := code[at]; !ok {
						return fmt.Errorf("unavailable testCODEword%x", at)
					}
					code[at] = v
					return nil
				}, Call: func(call NativeCommandFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if index >= len(f.Calls) {
						return NativeCommandFrameResult{}, fmt.Errorf("unexpected UIchild%x", call.Routine)
					}
					want := f.Calls[index]
					if *phase == 0 {
						entered++
						if uint32(call.Routine) != want.Routine || call.Caller != f.Input.Caller || call.Context.D != want.D {
							return NativeCommandFrameResult{}, fmt.Errorf("actual native child input/routine differs%x/%08x expected%x/%08x", call.Routine, call.Context.D, want.Routine, want.D)
						}
						if call.Routine == 0x102e4 && (call.PaletteA2+0x100000 != want.A2 || call.PaletteA3+0x100000 != want.A3) {
							return NativeCommandFrameResult{}, fmt.Errorf("native palette input pointers differ")
						}
						if call.Routine == 0xd8cc && call.TargetA0 != want.A0 {
							return NativeCommandFrameResult{}, fmt.Errorf("native terrain target pointer differs")
						}
						if fmt.Sprintf("%x", sha256.Sum256(b)) != want.Hash || code[0x3f90] != want.Code3F90 || code[0x4468] != want.Code4468 || code[0xa2a] != want.CodeA2A {
							return NativeCommandFrameResult{}, fmt.Errorf("native child prefix BSS/CODE mutation differs")
						}
						if pending {
							*phase = 1
							return NativeCommandFrameResult{Complete: false, Zero: true}, nil
						}
					}
					call.Context.D = want.Output
					index++
					return NativeCommandFrameResult{Complete: true, Zero: want.Zero}, nil
				}}
				done := false
				var step NativeCommandStep
				for resumes := 0; resumes < 16 && !done; resumes++ {
					step, done, e = state.ExecuteFrame(&rules, f.Input.Caller, &c, cb)
					if e != nil {
						t.Fatal(e)
					}
				}
				if !done || entered != len(f.Calls) || index != len(f.Calls) || f.ErrorPC != 0 || c.D != f.D {
					t.Fatalf("native handler continuation incomplete/changed %v/%d/%08x native%08x", done, index, c.D, f.D)
				}
				calls := []int{}
				for _, call := range f.Calls {
					calls = append(calls, int(call.Routine))
				}
				if !reflect.DeepEqual(step.Calls, calls) || step.Debited {
					t.Fatal("UIcalls repeated or UIdebit invented")
				}
				if fmt.Sprintf("%x", sha256.Sum256(b)) != f.Hash || code[0x3f90] != f.Code3F90 || code[0x4468] != f.Code4468 || code[0xa2a] != f.CodeA2A {
					t.Fatal("native complete UIhandler BSS/CODE differs")
				}
			})
		}
	}
}

func TestNativeCommandFrameKeepsOriginalCallerAndPrefixAcrossWait(t *testing.T) {
	rules, e := DecodeNativeCommandRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, command := range []uint8{116, 122} {
		t.Run(fmt.Sprintf("command%d", command), func(t *testing.T) {
			input := commandFrameInput{Caller: 0xeb56, Owner: 1, Command: command, Seed: 0x11223344, SavedSeed: 0xaabbccdd, Target: 0x530000, D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 0xff008877}}
			b := commandFrameInitial(input)
			memory := commandFrameBacking(b)
			c := NativeCommandRegisterContext{D: input.D}
			state := NativeCommandFrameState{}
			calls := []int{}
			targets := []uint32{}
			cb := NativeCommandFrameCallbacks{Immediate: NativeCommandCallbacks{Memory: memory}, Call: func(call NativeCommandFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
				if *phase == 0 {
					calls = append(calls, call.Routine)
					*phase = 1
					call.Context.D[7] ^= 0xabcdef01
					return NativeCommandFrameResult{Complete: false, Zero: true}, nil
				}
				if call.Routine == 0xd8cc {
					targets = append(targets, call.TargetA0)
				}
				return NativeCommandFrameResult{Complete: true, Zero: false}, nil
			}}
			_, complete, e := state.ExecuteFrame(&rules, input.Caller, &c, cb)
			if e != nil || complete {
				t.Fatal("initial child should suspend", e)
			}
			if command == 116 {
				seed, _ := memory.Read32(0xeb28)
				if seed != input.SavedSeed {
					t.Fatal("source seed reset prefix missing")
				}
			}
			terrain, _ := memory.Read16(0xeb22)
			if command == 122 && terrain != 1 {
				t.Fatal("source terrain prefix missing")
			}
			_ = memory.Write8(input.Caller+1, 124)
			_ = memory.Write32(0xeb24, 0xdeadbeef)
			_ = memory.Write32(0xeb28, 0x12345678)
			for resumes := 0; resumes < 12 && !complete; resumes++ {
				if state.ChildActive && state.ChildRoutine == 0xd8cc {
					_ = memory.Write32(0x22, 0x540000)
					c.D = [8]uint32{}
				}
				_, complete, e = state.ExecuteFrame(&rules, input.Caller, &c, cb)
				if e != nil {
					t.Fatal(e)
				}
			}
			if !complete {
				t.Fatal("UIhandler did not complete")
			}
			seed, _ := memory.Read32(0xeb28)
			if seed != 0x12345678 {
				t.Fatal("seed reset prefix repeated")
			}
			terrain, _ = memory.Read16(0xeb22)
			if command == 122 && terrain != 1 {
				t.Fatal("terrain increment repeated")
			}
			want := []int{0x102e4, 0x10ad8}
			if command == 122 {
				want = []int{0x102e4, 0x1a32a, 0xd8cc}
				if !reflect.DeepEqual(targets, []uint32{input.Target}) {
					t.Fatal("pending terrain child target changed")
				}
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("frozen handler changed/replayed%v", calls)
			}
			if _, _, e := state.ExecuteFrame(&rules, input.Caller+10, &c, cb); e == nil {
				t.Fatal("changed source caller accepted")
			}
		})
	}
}

func TestNativeCommandFrameImmediateBodyDelegatesOnce(t *testing.T) {
	rules, e := DecodeNativeCommandRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	input := commandFrameInput{Caller: 0xeb56, Owner: 1, Command: 120, D: [8]uint32{0xaabb1234, 2, 3, 4, 5, 6, 7, 8}}
	b := commandFrameInitial(input)
	memory := commandFrameBacking(b)
	god := 0xe8a4
	_ = memory.Write32(god, 100)
	state := NativeCommandFrameState{}
	c := NativeCommandRegisterContext{D: input.D}
	cb := NativeCommandFrameCallbacks{Immediate: NativeCommandCallbacks{Memory: memory}}
	step, done, e := state.ExecuteFrame(&rules, input.Caller, &c, cb)
	if e != nil || !done || step.Debited {
		t.Fatal("ordinary immediate body failed", e)
	}
	money, _ := memory.Read32(god)
	// Command120 uses owner-derived D0.L, then MOVE.W XY before ADD.L.
	expected := uint32(100) + uint32(uint16(0x1234))
	if money != expected {
		t.Fatalf("actual ordinary body amount%x native%x", money, expected)
	}
	before := append([]byte(nil), b...)
	out := c
	for i := 0; i < 3; i++ {
		if _, done, e = state.ExecuteFrame(&rules, input.Caller, &c, cb); e != nil || !done {
			t.Fatal(e)
		}
	}
	if !reflect.DeepEqual(before, b) || c != out {
		t.Fatal("completed ordinary prefix/body repeated")
	}
}

func TestNativeCommandFrameComposesPendingDeferredSide(t *testing.T) {
	rules, e := DecodeNativeCommandRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	input := commandFrameInput{Caller: 0xeb56, Owner: 1, Command: 116, Seed: 1, SavedSeed: 2, Target: 0x530000}
	b := commandFrameInitial(input)
	b[0xeb60], b[0xeb61], b[0xeb68] = 2, 122, 4
	memory := commandFrameBacking(b)
	initial := [8]uint32{1, 2, 3, 4, 5, 6, 7, 0xaabbff00}
	frame := NativeFrameRegisterContext{D: initial}
	deferred := NativeDeferredFrameState{}
	commands := [2]NativeCommandFrameState{}
	entered := []string{}
	cb := NativeDeferredFrameCallbacks{Memory: memory, Execute: func(at int, c *NativeCommandRegisterContext, _ *uint32) (bool, error) {
		side := (at - 0xeb56) / 10
		_, done, e := commands[side].ExecuteFrame(&rules, at, c, NativeCommandFrameCallbacks{Immediate: NativeCommandCallbacks{Memory: memory}, Call: func(call NativeCommandFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
			if *phase == 0 {
				entered = append(entered, fmt.Sprintf("%x/%x", call.Caller, call.Routine))
				*phase = 1
				call.Context.D[7]++
				return NativeCommandFrameResult{Complete: false, Zero: true}, nil
			}
			return NativeCommandFrameResult{Complete: true, Zero: false}, nil
		}})
		return done, e
	}}
	done := false
	for i := 0; i < 12 && !done; i++ {
		done, e = deferred.TickDeferredFrame(&frame, cb)
		if e != nil {
			t.Fatal(e)
		}
		if !done {
			side := int(deferred.Side)
			command, _ := memory.Read8(0xeb56 + side*10 + 1)
			if command == 0 {
				t.Fatal("pending deferred command cleared early")
			}
		}
	}
	if !done || frame.D != initial {
		t.Fatal("deferred MOVEM/output incomplete")
	}
	want := []string{"eb56/102e4", "eb56/10ad8", "eb60/102e4", "eb60/1a32a", "eb60/d8cc"}
	if !reflect.DeepEqual(entered, want) {
		t.Fatalf("original side/child order replayed%v", entered)
	}
}

func TestNativeCommandFrameOrdinaryErrorKeepsOriginalPrefix(t *testing.T) {
	rules, e := DecodeNativeCommandRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	input := commandFrameInput{Caller: 0xeb56, Owner: 1, Command: 255, D: [8]uint32{0x12345678, 2, 3, 4, 5, 6, 7, 8}}
	first := commandFrameInitial(input)
	second := append([]byte(nil), first...)
	direct := NativeCommandRegisterContext{D: input.D}
	framed := direct
	_, directError := rules.Execute(input.Caller, &direct, NativeCommandCallbacks{Memory: commandFrameBacking(first)})
	state := NativeCommandFrameState{}
	_, done, frameError := state.ExecuteFrame(&rules, input.Caller, &framed, NativeCommandFrameCallbacks{Immediate: NativeCommandCallbacks{Memory: commandFrameBacking(second)}})
	if done || directError == nil || frameError == nil || direct != framed || !reflect.DeepEqual(first, second) {
		t.Fatal("ordinary native error prefix was changed")
	}
	before := framed
	if _, _, e := state.ExecuteFrame(&rules, input.Caller, &framed, NativeCommandFrameCallbacks{Immediate: NativeCommandCallbacks{Memory: commandFrameBacking(second)}}); e == nil || framed != before {
		t.Fatal("failed ordinary operation repeated")
	}
}

func TestNativeCommandFrameMissingChildrenAreExplicit(t *testing.T) {
	rules, e := DecodeNativeCommandRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, command := range []uint8{106, 108, 110, 112, 116, 122} {
		input := commandFrameInput{Caller: 0xeb56, Owner: 1, Command: command}
		memory := commandFrameBacking(commandFrameInitial(input))
		state := NativeCommandFrameState{}
		c := NativeCommandRegisterContext{}
		_, done, e := state.ExecuteFrame(&rules, input.Caller, &c, NativeCommandFrameCallbacks{Immediate: NativeCommandCallbacks{Memory: memory}})
		if done || e == nil {
			t.Fatalf("unbound command%d was fabricated as complete", command)
		}
	}
}
