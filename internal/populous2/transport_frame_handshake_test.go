package populous2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type transportHandshakeInput struct {
	Name                         string
	D                            [8]uint32
	Profile, Peer                uint16
	WrongTokens, WrongSignatures int
	Cancel                       bool
	AbortAt                      int
	ChildSR, DialogFlag          uint16
	Counter                      uint32
}
type transportHandshakeSnapshot struct {
	PC                          int
	SR                          uint16
	PointerHash                 string
	D                           [8]uint32
	BSSHash, CodeHash, ChipHash string
	Sent                        []byte
}
type transportHandshakeFixture struct {
	Input       transportHandshakeInput
	Frames      []transportHandshakeSnapshot
	Initialized int
	EntryD      [8]uint32
}

func TestNativeTransportHandshakeAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/transport_frame_handshake_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []transportHandshakeFixture }
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 87 {
		t.Fatalf("handshake corpus count %d", len(corpus.Cases))
	}
	exe := testBundle(t).Executable
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			p, e := NewNativeFramePresentationState(exe, 0x500000, 0x400000)
			if e != nil {
				t.Fatal(e)
			}
			for i := 0x408; i < len(p.Chip); i++ {
				p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			p.InterruptChain = false
			if _, e = p.Initialize(exe, NativeMouseSample{}); e != nil {
				t.Fatal(e)
			}
			raw := make([]byte, 0x11280)
			m := p.Memory(commandFrameBacking(raw))
			code := fileFrameRelocatedCode(t)
			cm := commandFrameBacking(code)
			_ = cm.Write16(0x3ea, 0)
			_ = cm.Write32(0x77a, p.CopperSelector)
			_ = cm.Write32(0x77e, p.SpritePatchPointer)
			_ = m.Write32(0x12, f.Input.Counter)
			_ = m.Write16(0x3b0, f.Input.DialogFlag)
			_ = m.Write16(0x15a, 9600)
			_ = m.Write16(0x15e, 380)
			_ = m.Write16(0xeb42, f.Input.Profile)
			_ = m.Write16(0xeb44, 4)
			_ = m.Write8(0xeb5e, 2)
			_ = m.Write8(0xeb68, 4)
			header := []byte{0, 1, 0, 1, 0xa3, 0xb5, 0x77, 0x88, 0x99, 0xaa, 1, 0x55, 2, 0xaa}
			for i, v := range header {
				_ = m.Write8(0xeb22+i, v)
			}
			god := make([]byte, 236)
			for i := range god {
				_ = m.Write8(0xe8f2+i, byte(i*37+11))
				_ = m.Write8(0xea2c+i, byte(i*29+83))
				god[i] = byte(i*19 + 101)
			}
			var ring [381]byte
			var rd, wr uint16
			receive := func(data []byte) {
				for _, v := range data {
					ring[wr] = v
					wr++
					if wr > 380 {
						wr = 0
					}
				}
			}
			sent := []byte{}
			round := 0
			sigRound := 0
			stage := 0
			remaining := 0
			token := true
			state := NativeTransportHandshakeState{}
			port := NativeSerialPort{Configure: func(baud uint16) error {
				if baud != 9600 {
					return fmt.Errorf("baud differs")
				}
				return nil
			}, Flush: func() error {
				rd = 0
				wr = 0
				if state.PC == 0x17f18 && sigRound > 0 && sigRound <= f.Input.WrongSignatures {
					token = true
				}
				return nil
			}, Available: func() (int, error) {
				n := int(wr) - int(rd)
				if n < 0 {
					n += 381
				}
				return n, nil
			}, Read: func(dst []byte) (int, error) {
				n := 0
				for n < len(dst) && rd != wr {
					dst[n] = ring[rd]
					rd++
					if rd > 380 {
						rd = 0
					}
					n++
				}
				if n < len(dst) {
					return n, ErrNativeSerialWait
				}
				return n, nil
			}, Write: func(src []byte) (int, error) {
				sent = append(sent, src...)
				// The peer responds only after a complete source transfer.
				if token {
					round++
					v := byte(0x3f)
					if round <= f.Input.WrongTokens {
						v = 0x7e
					} else {
						token = false
						stage = 1
					}
					receive([]byte{v})
					return len(src), nil
				}
				switch stage {
				case 1:
					sigRound++
					if sigRound <= f.Input.WrongSignatures {
						receive([]byte("FAIL"))
					} else {
						receive([]byte("ABCD"))
					}
					stage = 2
				case 2:
					receive([]byte{byte(f.Input.Peer)})
					if byte(f.Input.Profile) == 2 {
						receive(header)
					}
					stage = 3
					if byte(f.Input.Profile) == 2 {
						remaining = 236
					} else {
						remaining = 14
					}
				case 3:
					remaining -= len(src)
					if remaining == 0 {
						if byte(f.Input.Profile) == 2 {
							receive(god)
							stage = 5
						} else {
							stage = 4
							remaining = 236
						}
					}
				case 4:
					remaining -= len(src)
					if remaining == 0 {
						receive(god)
						stage = 5
					}
				}
				return len(src), nil
			}}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			waitReady := false
			initialized := 0
			resolver := func(address uint32) ([]byte, error) {

				at := int(address - p.ChipBase)
				if at < 0 || at > len(p.Chip)-32000 {
					return nil, fmt.Errorf("chip unavailable")
				}
				return p.Chip[at : at+32000], nil
			}
			cb := NativeTransportFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, Sound: func(uint16, *NativeFrameRegisterContext) error { return nil }, Call: func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
				if call.Routine != 0x10ad8 || frame.D != f.EntryD {
					return NativeCommandFrameResult{}, fmt.Errorf("reset source inputs differ %08x vs %08x", frame.D, f.EntryD)
				}
				initialized++
				for i := range frame.D {
					frame.D[i] ^= 0x765400 + uint32(i)*0x10101
				}
				return NativeCommandFrameResult{Complete: true}, nil
			}}, Port: port, ReceiveImage: func() ([381]byte, uint16, uint16) { return ring, rd, wr }, WaitCPU: func(site, iterations uint32) (bool, error) {
				if iterations != 100000 {
					return false, fmt.Errorf("source delay differs")
				}
				ready := waitReady
				waitReady = false
				return ready, nil
			}}
			cancelled := false
			clickReads := 0
			oldRead16 := cb.Memory.Read16
			cb.Memory.Read16 = func(at int) (uint16, error) {
				if at == 0x140 {
					clickReads++
				}
				if at == 0x140 && clickReads > 0 && f.Input.Cancel && !cancelled {
					transportHandshakeClick(t, cm, m)
					cancelled = true
				}
				return oldRead16(at)
			}
			oldCall := cb.Call
			cb.CallTransport = func(call NativeFileFrameCall, phase *uint32) (NativeSerialFrameChildResult, error) {
				if call.Routine == 0x102e4 {
					return NativeSerialFrameChildResult{Complete: true, Zero: true}, nil
				}
				r, e := oldCall(call, phase)
				return NativeSerialFrameChildResult{Complete: r.Complete, Zero: f.Input.ChildSR&4 != 0, Negative: f.Input.ChildSR&8 != 0}, e
			}
			oldWrite := cb.Port.Write
			cb.Port.Write = func(src []byte) (int, error) {
				n, e := oldWrite(src)
				next := map[int]int{0x17f7e: 0x17f84, 0x17fdc: 0x17fee, 0x18036: 0x18048, 0x1809c: 0x1811c, 0x180e8: 0x1811c}[state.PC]
				if next == f.Input.AbortAt && next != 0 {
					_ = m.Write8(0xa7, 1)
					_ = m.Write8(0x6f, 1)
				}
				return n, e
			}
			for i, want := range f.Frames {
				step, e := state.Advance(cb)
				if e != nil {
					t.Fatal(e)
				}
				pc := step.PC
				if step.Complete {
					pc = 0
				}
				if pc == 0x33b2 {
					pc = 0x33ea
				}
				if pc != want.PC || frame.D != want.D {
					t.Fatalf("stage %d PC%x/%x D%08x/%08x", i, pc, want.PC, frame.D, want.D)
				}
				if fileFrameHash(fileFrameMemoryBytes(t, m)) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(p.Chip) != want.ChipHash || !bytes.Equal(sent, want.Sent) {
					t.Fatalf("stage %d backing BSS%v CODE%v chip%v sent%v", i, fileFrameHash(fileFrameMemoryBytes(t, m)) == want.BSSHash, fileFrameHash(code) == want.CodeHash, fileFrameHash(p.Chip) == want.ChipHash, bytes.Equal(sent, want.Sent))
				}
				if want.PC == 0 {
					if !step.FlagsKnown || step.Zero != (want.SR&4 != 0) || step.Negative != (want.SR&8 != 0) {
						t.Fatalf("CCR got %+v SR%04x", step, want.SR)
					}
				}
				if want.PC == 0x33ea {
					clicked := false
					reads := 0
					original := cb.Memory.Read16
					cb.Memory.Read16 = func(at int) (uint16, error) {
						if at == 0x140 {
							reads++
						}
						if at == 0x140 && reads > 0 && !clicked {
							transportHandshakeClick(t, cm, m)
							clicked = true
						}
						return original(at)
					}
				} else if want.PC == 0x17f74 {
					_ = m.Write32(0x12, frame.D[0]+1)
				} else {
					waitReady = true
				}
			}
			if initialized != f.Initialized {
				t.Fatal("reset count changed")
			}
		})
	}
}

func transportHandshakeClick(t *testing.T, code, m FollowerCleanupMemory) {
	t.Helper()
	start, _ := code.Read16(0xab4e)
	width, _ := code.Read16(0xab54)
	col, _ := code.Read16(0xab50)
	row, _ := code.Read16(0xab52)
	for i := 0; ; i++ {
		v, e := code.Read8(0xab4e + int(start) + i)
		if e != nil || v == 0 {
			t.Fatal("requester action missing")
		}
		if int8(v) > 0x5a {
			mark, _ := code.Read8(0x4e92 + int(v-0x5b))
			if int8(mark) > 0 {
				_ = m.Write16(0x134, uint16((int(col)+i%(int(width)+1))*8))
				_ = m.Write16(0x136, uint16(int(row)+i/(int(width)+1)*8))
				_ = m.Write16(0x140, 1)
				return
			}
		}
	}
}
