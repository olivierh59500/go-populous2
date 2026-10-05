package populous2

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type serialNativeInput struct {
	Name                  string
	Mode                  uint8
	Caller                int
	RNG                   uint32
	PacketHex             string
	AbortRead, AbortWrite int
	Transports            [3]uint8
	Profile               uint16
	Speeds, Rules         [2]uint16
	AvailableError        bool
}
type serialNativeCase struct {
	Input                         serialNativeInput
	RecordHex, SentHex            string
	GameMode, Profile             uint16
	Transports                    [3]uint8
	Speeds, Rules                 [2]uint16
	RNG                           uint32
	D                             [8]uint32
	Flushes, Messages, StackBytes int
	Switches                      []uint16
	NativeReturn                  uint32
}
type serialNativeCorpus struct{ Packets, Resumes, Disconnects []serialNativeCase }

func serialFixtureMemory(t *testing.T, c serialNativeInput) (FollowerCleanupMemory, []byte) {
	t.Helper()
	b := make([]byte, 0x11280)
	guard := func(at, n int) error {
		if at < 0 || at+n > len(b) {
			return fmt.Errorf("serial fixture memory%x+%d outside BSS", at, n)
		}
		return nil
	}
	m := FollowerCleanupMemory{
		Read8: func(at int) (uint8, error) {
			if e := guard(at, 1); e != nil {
				return 0, e
			}
			return b[at], nil
		},
		Read16: func(at int) (uint16, error) {
			if e := guard(at, 2); e != nil {
				return 0, e
			}
			return binary.BigEndian.Uint16(b[at:]), nil
		},
		Read32: func(at int) (uint32, error) {
			if e := guard(at, 4); e != nil {
				return 0, e
			}
			return binary.BigEndian.Uint32(b[at:]), nil
		},
		Write8: func(at int, v uint8) error {
			if e := guard(at, 1); e != nil {
				return e
			}
			b[at] = v
			return nil
		},
		Write16: func(at int, v uint16) error {
			if e := guard(at, 2); e != nil {
				return e
			}
			binary.BigEndian.PutUint16(b[at:], v)
			return nil
		},
		Write32: func(at int, v uint32) error {
			if e := guard(at, 4); e != nil {
				return e
			}
			binary.BigEndian.PutUint32(b[at:], v)
			return nil
		},
	}
	_ = m.Write16(0xeb44, 6)
	_ = m.Write32(0xeb28, c.RNG)
	_ = m.Write16(0xeb42, c.Profile)
	_ = m.Write32(0xeb6a, 0xeb56)
	for i, v := range c.Transports {
		_ = m.Write8(0xeb56+i*10+8, v)
	}
	for i, v := range c.Speeds {
		_ = m.Write16(0xe90c+i*314, v)
	}
	for i, v := range c.Rules {
		_ = m.Write16(0xeb2c+i*2, v)
	}
	return m, b
}
func loadSerialNativeCorpus(t *testing.T) serialNativeCorpus {
	t.Helper()
	data, err := os.ReadFile("testdata/serial_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var c serialNativeCorpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	return c
}
func checkSerialNativeState(t *testing.T, m FollowerCleanupMemory, b []byte, c serialNativeCase) {
	t.Helper()
	if hex.EncodeToString(b[c.Input.Caller:c.Input.Caller+10]) != c.RecordHex || binary.BigEndian.Uint16(b[0xeb44:]) != c.GameMode || binary.BigEndian.Uint16(b[0xeb42:]) != c.Profile || binary.BigEndian.Uint32(b[0xeb28:]) != c.RNG {
		t.Fatal("native serial record/session/RNG differs")
	}
	for i, mode := range c.Transports {
		if b[0xeb56+i*10+8] != mode {
			t.Fatal("native three-record disconnect differs")
		}
	}
	for i, v := range c.Speeds {
		got, _ := m.Read16(0xe90c + i*314)
		if got != v {
			t.Fatal("native menu peer speed overwrite differs")
		}
	}
	for i, v := range c.Rules {
		got, _ := m.Read16(0xeb2c + i*2)
		if got != v {
			t.Fatal("native menu rule OR differs")
		}
	}
}

func TestNativeSerialPacketPreludeAgainstOriginalCPU(t *testing.T) {
	corpus := loadSerialNativeCorpus(t)
	if len(corpus.Packets) != 160 {
		t.Fatal("native serial packet corpus incomplete")
	}
	for index, c := range corpus.Packets {
		t.Run(fmt.Sprintf("packet%d", index), func(t *testing.T) {
			input := c.Input
			m, b := serialFixtureMemory(t, input)
			copy(b[input.Caller:], []byte{1, 6, 9, 12, 0x11, 0x22, 0x33, 0x44, input.Mode, 0xab})
			incoming, err := hex.DecodeString(input.PacketHex)
			if err != nil {
				t.Fatal(err)
			}
			sent := []byte{}
			flushes, messages := 0, 0
			cb := NativeSerialCallbacks{Memory: m, Port: NativeSerialPort{
				Available: func() (int, error) { return len(incoming), nil }, Flush: func() error { flushes++; return nil },
				Write: func(src []byte) (int, error) {
					n := len(src)
					var err error
					if input.AbortWrite >= 0 {
						n = input.AbortWrite
						err = errors.New("native transmit interrupted")
					}
					sent = append(sent, src[:n]...)
					return n, err
				},
				Read: func(dst []byte) (int, error) {
					n := len(dst)
					var err error
					if input.AbortRead >= 0 {
						n = input.AbortRead
						err = errors.New("native receive interrupted")
					}
					copy(dst, incoming[:n])
					return n, err
				},
			}, Message: func(kind NativeSerialMessage) (bool, error) {
				if kind != NativeSerialLandscapeMismatch {
					t.Fatal("wrong native packet dialog")
				}
				messages++
				return true, nil
			}}
			packet, err := NewNativeSerialPacket(input.Caller, input.Mode)
			if err != nil {
				t.Fatal(err)
			}
			var context NativeCommandRegisterContext
			for i := range context.D {
				context.D[i] = 0x98760000 + uint32(i)*0x103
			}
			step, err := packet.Advance(&context, cb)
			if err != nil {
				t.Fatal(err)
			}
			if !step.Complete || step.Waiting || context.D != c.D || hex.EncodeToString(sent) != c.SentHex || flushes != c.Flushes || messages != c.Messages {
				t.Fatalf("native serial prelude differs: step%+v D%x native%x flush%d/%d messages%d/%d", step, context.D, c.D, flushes, c.Flushes, messages, c.Messages)
			}
			checkSerialNativeState(t, m, b, c)
		})
	}
}

func TestNativeSerialMenuResumeAgainstOriginalCPU(t *testing.T) {
	corpus := loadSerialNativeCorpus(t)
	if len(corpus.Resumes) != 72 {
		t.Fatal("native serial menu corpus incomplete")
	}
	for index, c := range corpus.Resumes {
		t.Run(fmt.Sprintf("resume%d", index), func(t *testing.T) {
			input := c.Input
			m, b := serialFixtureMemory(t, input)
			incoming, err := hex.DecodeString(input.PacketHex)
			if err != nil {
				t.Fatal(err)
			}
			sent := []byte{}
			flushes := 0
			switches := []uint16{}
			cb := NativeSerialCallbacks{Memory: m, Port: NativeSerialPort{
				Available: func() (int, error) {
					if input.AvailableError {
						return 0, errors.New("native availability interrupted")
					}
					return len(incoming), nil
				},
				Flush: func() error { flushes++; return nil },
				Write: func(src []byte) (int, error) {
					if input.AvailableError {
						return 0, errors.New("native transmit interrupted")
					}
					sent = append(sent, src...)
					return len(src), nil
				},
				Read: func(dst []byte) (int, error) {
					n := len(dst)
					var err error
					if input.AbortRead > 0 {
						n = input.AbortRead
						err = errors.New("native partial menu read interrupted")
					}
					copy(dst, incoming[:n])
					return n, err
				},
			}, SwitchProfile: func(profile uint16) error {
				switches = append(switches, profile)
				return m.Write32(0xeb6a, 0xeb4c+uint32(profile)*10)
			}}
			resume := NativeSerialResume{}
			step, err := resume.Advance(cb)
			if err != nil {
				t.Fatal(err)
			}
			if step.Waiting || step.Complete != (c.StackBytes == 0) || step.NativeStackReturn != 0 && step.NativeStackReturn != c.NativeReturn || hex.EncodeToString(sent) != c.SentHex || flushes != c.Flushes || !reflect.DeepEqual(switches, c.Switches) {
				t.Fatalf("native serial menu differs: step%+v native stack%d return%x flush%d/%d switch%v/%v", step, c.StackBytes, c.NativeReturn, flushes, c.Flushes, switches, c.Switches)
			}
			if c.StackBytes == 10 && (step.Failure == nil || step.NativeStackReturn != c.NativeReturn) {
				t.Fatal("native unpopped error stack was hidden")
			}
			checkSerialNativeState(t, m, b, c)
		})
	}
}

func TestNativeSerialDisconnectAliasesAgainstOriginalCPU(t *testing.T) {
	corpus := loadSerialNativeCorpus(t)
	if len(corpus.Disconnects) != 200 {
		t.Fatal("native disconnect corpus incomplete")
	}
	for index, c := range corpus.Disconnects {
		t.Run(fmt.Sprintf("disconnect%d", index), func(t *testing.T) {
			m, b := serialFixtureMemory(t, c.Input)
			flushes := 0
			if err := DisconnectNativeSerial(NativeSerialCallbacks{Memory: m, Port: NativeSerialPort{Flush: func() error { flushes++; return nil }}}); err != nil {
				t.Fatal(err)
			}
			if flushes != 1 {
				t.Fatal("native ring flush missing")
			}
			checkSerialNativeState(t, m, b, c)
		})
	}
}

func TestNativeSerialPartialWaitPreservesCommandProgress(t *testing.T) {
	c := serialNativeInput{Caller: 0xeb56, Profile: 1, RNG: 4311, Transports: [3]uint8{6, 8, 0}}
	m, b := serialFixtureMemory(t, c)
	copy(b[0xeb56:], []byte{1, 6, 9, 12, 0, 0, 0, 0, 6, 0})
	copy(b[0xeb60:], []byte{2, 8, 10, 13, 0, 0, 0, 0, 8, 0})
	queue := []byte{2, 8, 10, 13, 0, 0, 0x10, 0xda}
	sent := []byte{}
	executed := []int{}
	port := NativeSerialPort{Available: func() (int, error) { return len(queue), nil }, Flush: func() error { return nil }, Write: func(p []byte) (int, error) { sent = append(sent, p...); return len(p), nil }, Read: func(dst []byte) (int, error) {
		n := min(3, len(dst))
		copy(dst, queue[:n])
		queue = queue[n:]
		return n, ErrNativeSerialWait
	}}
	scheduler := NewNativeSerialDeferred(NativeCommandRegisterContext{})
	callbacks := NativeSerialDeferredCallbacks{Serial: NativeSerialCallbacks{Memory: m, Port: port}, Execute: func(at int, context *NativeCommandRegisterContext) error {
		executed = append(executed, at)
		old, _ := m.Read32(0xeb28)
		return m.Write32(0xeb28, old+3)
	}}
	for attempt := 0; attempt < 4; attempt++ {
		step, err := scheduler.Advance(callbacks)
		if err != nil {
			t.Fatal(err)
		}
		if step.Complete {
			break
		}
	}
	if !scheduler.Complete || !reflect.DeepEqual(executed, []int{0xeb56, 0xeb60}) || len(sent) != 8 || len(queue) != 0 || !bytes.Equal(b[0xeb57:0xeb5a], []byte{0, 0, 0}) || !bytes.Equal(b[0xeb61:0xeb64], []byte{0, 0, 0}) {
		t.Fatal("waiting serial stage replayed or lost a command")
	}
	if got := binary.BigEndian.Uint32(b[0xeb28:]); got != 4317 {
		t.Fatal("waiting transport invented an RNG resynchronization")
	}
}

func TestNativeSerialSinglePlayerMenuNeedsNoTransport(t *testing.T) {
	m, _ := serialFixtureMemory(t, serialNativeInput{Profile: 1, Transports: [3]uint8{2, 4, 0}})
	resume := NativeSerialResume{}
	step, err := resume.Advance(NativeSerialCallbacks{Memory: m})
	if err != nil || !step.Complete || step.Waiting {
		t.Fatal("nonserial181c0 branch requested fabricated I/O")
	}
}
