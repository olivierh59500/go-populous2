package populous2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
)

func TestNativeRuntimeHandshakeFailurePaletteAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/native_runtime_handshake_palette_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []transportHandshakeFixture }
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 4 {
		t.Fatalf("handshake corpus count %d", len(corpus.Cases))
	}
	exe := testBundle(t).Executable
	snapshots := 0
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if len(f.Frames) != 39 || f.Frames[len(f.Frames)-1].PC != 0 || f.Input.Profile != f.Input.Peer {
				t.Fatal("same-profile failure source trace truncated")
			}
			h := nativeRuntimeHostTest(t)
			p := h.Session.Presentation
			var e error
			for i := 0x408; i < len(p.Chip); i++ {
				p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			p.InterruptChain = false
			if _, e = p.Initialize(exe, NativeMouseSample{}); e != nil {
				t.Fatal(e)
			}
			m := h.Memory.BSS
			code := h.Code.RawData()[:0x3fa2c]
			cm := h.Memory.Code
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
			resolver := func(address uint32) ([]byte, error) {

				at := int(address - p.ChipBase)
				if at < 0 || at > len(p.Chip)-32000 {
					return nil, fmt.Errorf("chip unavailable")
				}
				return p.Chip[at : at+32000], nil
			}
			cb := NativeTransportFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, Sound: func(uint16, *NativeFrameRegisterContext) error { return nil }, Call: func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
				return NativeCommandFrameResult{}, fmt.Errorf("same-profile failure unexpectedly requested child%x", call.Routine)
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
			left, right := net.Pipe()
			defer right.Close()
			conn, e := NewNativeSerialConn(left)
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			transport, e := h.NewTransport(conn, cb, func(bool, *NativeFrameRegisterContext) error { return nil })
			if e != nil {
				t.Fatal(e)
			}
			transport.Callbacks = cb
			control, e := transport.controlCallbacks(&frame)
			if e != nil {
				t.Fatal(e)
			}
			cb.CallTransport = control.CallTransport
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
				snapshots++
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
					if transport.palette != nil {
						pc = 0x786
					}
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
				} else if want.PC == 0x786 {
					if _, e = p.VBlank(NativeMouseSample{}, m, &frame); e != nil {
						t.Fatal(e)
					}
				} else if want.PC == 0x17f74 {
					_ = m.Write32(0x12, frame.D[0]+1)
				} else {
					waitReady = true
				}
			}
			if f.Initialized != 0 {
				t.Fatal("reset count changed")
			}
		})
	}
	if snapshots != 156 {
		t.Fatal("native failure palette coverage changed", snapshots)
	}
}
