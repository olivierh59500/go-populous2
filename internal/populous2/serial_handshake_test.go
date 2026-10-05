package populous2

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeHandshakeCase struct {
	Input struct {
		Profile, Peer                          uint16
		WrongSignatures, WrongTokens, CancelAt int
		Seed                                   uint32
	}
	HeaderHex, God1Hex, God2Hex    string
	Connected, Mode                uint16
	RNG                            uint32
	Roles                          [2]uint8
	Sent                           []string
	Flushes, Initialized, Messages int
	CPUWaits                       []uint32
	Attempts                       []uint16
	SignatureBudget                int16
	Attempt                        uint16
}

func TestNativeSerialHandshakeAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/serial_handshake_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []nativeHandshakeCase }
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 72 {
		t.Fatalf("native handshake corpus incomplete: %v", err)
	}
	rules, err := DecodeNativeSerialRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for index, c := range corpus.Cases {
		t.Run(fmt.Sprintf("handshake%d", index), func(t *testing.T) {
			input := c.Input
			m, b := serialFixtureMemory(t, serialNativeInput{Profile: input.Profile, Transports: [3]uint8{2, 4, 0}})
			_ = m.Write16(0xeb44, 4)
			header := []byte{0, 1, 0, 1, 0xa3, 0xb5, 0x77, 0x88, 0x99, 0xaa, 0x01, 0x55, 0x02, 0xaa}
			copy(b[0xeb22:], header)
			for i := 0; i < 236; i++ {
				b[0xe8f2+i] = byte(i*37 + 11)
				b[0xea2c+i] = byte(i*29 + 83)
			}
			peerGod := make([]byte, 236)
			for i := range peerGod {
				peerGod[i] = byte(i*19 + 101)
			}
			rx := []byte{}
			sent := []string{}
			waits := []uint32{}
			attempts := []uint16{}
			flushes, polls, signatureRound, tokenRound, initialized, messages := 0, 0, 0, 0, 0, 0
			clock := uint32(0)
			handshake, err := NewNativeSerialHandshake(rules, 300)
			if err != nil {
				t.Fatal(err)
			}
			callbacks := NativeSerialHandshakeCallbacks{Serial: NativeSerialCallbacks{Memory: m, Port: NativeSerialPort{
				Configure: func(uint16) error { return nil }, Flush: func() error { flushes++; rx = nil; return nil }, Available: func() (int, error) { return len(rx), nil },
				Read: func(dst []byte) (int, error) {
					n := min(len(dst), len(rx))
					copy(dst, rx[:n])
					rx = rx[n:]
					if n < len(dst) {
						return n, ErrNativeSerialWait
					}
					return n, nil
				},
				Write: func(src []byte) (int, error) {
					sent = append(sent, hex.EncodeToString(src))
					switch {
					case len(src) == 1 && src[0] == 0x3f:
						attempts = append(attempts, handshake.Attempt)
						tokenRound++
						token := byte(0x3f)
						if tokenRound <= input.WrongTokens {
							token = 0x7e
						}
						rx = append(rx, token)
					case string(src) == "ABCD":
						signatureRound++
						if signatureRound <= input.WrongSignatures {
							rx = append(rx, []byte("FAIL")...)
						} else {
							rx = append(rx, []byte("ABCD")...)
						}
					case len(src) == 1:
						rx = append(rx, byte(input.Peer))
						if input.Profile == 2 {
							rx = append(rx, header...)
						}
					case len(src) == 236:
						rx = append(rx, peerGod...)
					}
					return len(src), nil
				},
			}, Message: func(NativeSerialMessage) (bool, error) { messages++; return true, nil }}, FrameCounter: func() uint32 { return clock }, WaitCPU: func(site, count uint32) (bool, error) {
				if count != 100000 {
					t.Fatal("native CPU delay count changed")
				}
				waits = append(waits, site)
				return true, nil
			}, Cancel: func() bool { polls++; return input.CancelAt > 0 && polls == input.CancelAt }, Initialize: func() (bool, error) { initialized++; return true, nil }}
			for advances := 0; advances < 5000; advances++ {
				clock++
				step, err := handshake.Advance(callbacks)
				if err != nil {
					t.Fatal(err)
				}
				if step.Complete {
					break
				}
			}
			if handshake.phase != serialHandshakeFinished || handshake.Connected != (c.Connected != 0) || handshake.Ready != (c.Initialized != 0) || handshake.Attempt != c.Attempt || handshake.SignatureBudget != c.SignatureBudget || flushes != c.Flushes || messages != c.Messages || initialized != c.Initialized || !reflect.DeepEqual(sent, c.Sent) || !reflect.DeepEqual(waits, c.CPUWaits) || !reflect.DeepEqual(attempts, c.Attempts) {
				t.Fatalf("native handshake flow differs: phase%d connected%v ready%v attempt%d/%d budget%d/%d flush%d/%d polls%d messages%d/%d init%d/%d sent%v/%v waits%v/%v", handshake.phase, handshake.Connected, handshake.Ready, handshake.Attempt, c.Attempt, handshake.SignatureBudget, c.SignatureBudget, flushes, c.Flushes, polls, messages, c.Messages, initialized, c.Initialized, sent, c.Sent, waits, c.CPUWaits)
			}
			if hex.EncodeToString(b[0xeb22:0xeb30]) != c.HeaderHex || hex.EncodeToString(b[0xe8f2:0xe9de]) != c.God1Hex || hex.EncodeToString(b[0xea2c:0xeb18]) != c.God2Hex {
				t.Fatal("native received header/God suffix bytes differ")
			}
			mode, _ := m.Read16(0xeb44)
			rng, _ := m.Read32(0xeb28)
			if mode != c.Mode || rng != c.RNG || [2]byte{b[0xeb5e], b[0xeb68]} != c.Roles {
				t.Fatal("native negotiated roles/mode/actual seed differs")
			}
		})
	}
}
