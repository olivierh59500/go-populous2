package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func TestNativeSerialStartupAgainstCompleteOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/serial_startup_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Profile, World, Land                                                       uint16
			Seed                                                                       uint32
			GridHex, ActorsHex, GodsHex, ControlHex, ViewHex, CommandsHex, TemplateHex string
			GridHash, ActorsHash                                                       string
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeSerialRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range corpus.Cases {
		t.Run(fmt.Sprintf("profile%d-land%d", c.Profile, c.Land), func(t *testing.T) {
			roles := [3]uint8{6, 8, 0}
			if c.Profile == 2 {
				roles = [3]uint8{8, 6, 0}
			}
			m, b := serialFixtureMemory(t, serialNativeInput{Profile: c.Profile, Transports: roles, RNG: c.Seed, Rules: [2]uint16{0x155, 0x2aa}})
			_ = m.Write16(0xeb46, c.World)
			_ = m.Write16(0xeb22, c.Land)
			_ = m.Write32(0xeb24, c.Seed)
			_ = m.Write8(0xeb56, 1)
			_ = m.Write8(0xeb60, 2)
			_ = m.Write8(0xeb57, 104)
			_ = m.Write8(0xeb61, 106)
			_ = m.Write32(0xeb6a, 0xeb56+uint32(c.Profile-1)*10)
			for owner := 0; owner < 2; owner++ {
				for i := 0; i < 236; i++ {
					b[0xe8f2+owner*314+i] = byte(owner*51 + i*17)
				}
			}
			w, template, err := NewNativeSerialWorld(bundle, m, rules, rules.DefaultCustom)
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(template[:]) != c.TemplateHex {
				t.Fatal("native custom template differs")
			}
			actual := w.nativeCleanupMemory()
			for _, span := range []struct {
				Start, End int
				Hash       string
			}{{0xf44, 0x4f44, c.GridHash}, {0x5f50, 0xe76a, c.ActorsHash}} {
				data, err := serialReadBytes(actual, span.Start, span.End-span.Start)
				if err != nil {
					t.Fatal(err)
				}
				if fmt.Sprintf("%x", sha256.Sum256(data)) != span.Hash {
					t.Errorf("native initial full-span%x..%x hash differs", span.Start, span.End)
				}
			}
			for _, span := range []struct {
				Name       string
				Start, End int
				Hex        string
			}{{"grid", 0xf44, 0x4f44, c.GridHex}, {"actors", 0x5f50, 0xe76a, c.ActorsHex}, {"gods", 0xe76a, 0xeb18, c.GodsHex}, {"control", 0xdc2, 0xf44, c.ControlHex}, {"view", 0x5f44, 0x5f50, c.ViewHex}, {"commands", 0xeb18, 0xeb90, c.CommandsHex}} {
				want, err := hex.DecodeString(span.Hex)
				if err != nil {
					t.Fatal(err)
				}
				if span.Name == "commands" {
					at := 0xeb6a - span.Start
					want[at], want[at+1], want[at+2], want[at+3] = byte((0xeb56+uint32(c.Profile-1)*10)>>24), byte((0xeb56+uint32(c.Profile-1)*10)>>16), byte((0xeb56+uint32(c.Profile-1)*10)>>8), byte(0xeb56+uint32(c.Profile-1)*10)
				}
				differences := 0
				first := []string{}
				for i, value := range want {
					got, e := actual.Read8(span.Start + i)
					if e != nil {
						t.Fatal(e)
					}
					if got != value {
						differences++
						if len(first) < 8 {
							first = append(first, fmt.Sprintf("%x:%02x/%02x", span.Start+i, got, value))
						}
					}
				}
				if differences != 0 {
					t.Errorf("native initial%s differs%d bytes: %v", span.Name, differences, first)
				}
			}
		})
	}
}

func TestNativeSerialTwoWorldsExecuteBothPlayersActualCommands(t *testing.T) {
	bundle := testBundle(t)
	rules, err := DecodeNativeSerialRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	commands, err := DecodeNativeCommandRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	ports := [2]*NativeSerialConn{}
	ports[0], err = NewNativeSerialConn(left)
	if err != nil {
		t.Fatal(err)
	}
	ports[1], err = NewNativeSerialConn(right)
	if err != nil {
		t.Fatal(err)
	}
	defer ports[0].Close()
	defer ports[1].Close()
	var worlds [2]*World
	var handshake [2]*NativeSerialHandshake
	var callbacks [2]NativeSerialHandshakeCallbacks
	clock := uint32(0)
	for side := 0; side < 2; side++ {
		roles := [3]uint8{2, 4, 0}
		m, b := serialFixtureMemory(t, serialNativeInput{Profile: uint16(side + 1), Transports: roles})
		_ = m.Write16(0xeb46, 27)
		_ = m.Write16(0xeb22, 1)
		_ = m.Write32(0xeb24, 0x000110d7)
		_ = m.Write32(0xeb28, 0x88776655)
		_ = m.Write8(0xeb56, 1)
		_ = m.Write8(0xeb60, 2)
		_ = m.Write32(0xeb6a, 0xeb56+uint32(side)*10)
		own := 0xe8f2 + side*314
		b[own], b[own+1], b[own+2] = byte(side), byte(side+1), byte(side+2)
		_ = m.Write16(own+10, 5)
		handshake[side], err = NewNativeSerialHandshake(rules, 4800)
		if err != nil {
			t.Fatal(err)
		}
		waitSite, waitCalls := uint32(0), 0
		callbacks[side] = NativeSerialHandshakeCallbacks{Serial: NativeSerialCallbacks{Memory: m, Port: ports[side].Port(), Message: func(kind NativeSerialMessage) (bool, error) {
			t.Fatalf("unexpected gameplay connection dialog%d", kind)
			return false, nil
		}}, FrameCounter: func() uint32 { return clock }, WaitCPU: func(site, count uint32) (bool, error) {
			if site != waitSite {
				waitSite, waitCalls = site, 0
			}
			waitCalls++
			return waitCalls >= 10, nil
		}, Initialize: func() (bool, error) {
			var e error
			worlds[side], _, e = NewNativeSerialWorld(bundle, m, rules, rules.DefaultCustom)
			return e == nil, e
		}}
	}
	deadline := time.Now().Add(5 * time.Second)
	for !handshake[0].Ready || !handshake[1].Ready {
		if time.Now().After(deadline) {
			t.Fatal("real game connection handshake stalled")
		}
		clock++
		for side := 0; side < 2; side++ {
			step, err := handshake[side].Advance(callbacks[side])
			if err != nil {
				t.Fatal(err)
			}
			if step.Complete && !step.Ready {
				t.Fatalf("game connection failed%+v", step)
			}
		}
		time.Sleep(100 * time.Microsecond)
	}
	for tick := 0; tick < 240; tick++ {
		var schedulers [2]*NativeSerialDeferred
		var driver [2]NativeSerialDeferredCallbacks
		for side := 0; side < 2; side++ {
			w := worlds[side]
			m := w.nativeCleanupMemory()
			at := 0xeb56 + side*10
			// Actual papal-magnet command8 moves each owner's target and
			// selects native homing mode16. Both inputs travel through the
			// source17500 bodies, never World.Cast or synthetic mutations.
			_ = m.Write8(at+1, 8)
			_ = m.Write8(at+2, uint8(10+side*20+tick%10))
			_ = m.Write8(at+3, uint8(12+side*15+tick%7))
			schedulers[side] = NewNativeSerialDeferred(NativeCommandRegisterContext{})
			driver[side] = NativeSerialDeferredCallbacks{Serial: NativeSerialCallbacks{Memory: m, Port: ports[side].Port(), Message: func(kind NativeSerialMessage) (bool, error) {
				t.Fatalf("real game RNG mismatch at tick%d", tick)
				return false, nil
			}}, Execute: func(at int, c *NativeCommandRegisterContext) error {
				_, err := w.executeNativeNormalCommand(&commands, at, c, NativeCommandWorldBindings{})
				return err
			}}
		}
		deadline := time.Now().Add(time.Second)
		for !schedulers[0].Complete || !schedulers[1].Complete {
			if time.Now().After(deadline) {
				t.Fatalf("real game packet stage stalled%d", tick)
			}
			for side := 0; side < 2; side++ {
				if _, err := schedulers[side].Advance(driver[side]); err != nil {
					t.Fatal(err)
				}
			}
			time.Sleep(50 * time.Microsecond)
		}
		for side := 0; side < 2; side++ {
			worlds[side].hydrateNativeRuntimeRecords()
		}
		for owner := 0; owner < 2; owner++ {
			want := 10 + owner*20 + tick%10 + (12+owner*15+tick%7)*64
			for side := 0; side < 2; side++ {
				if worlds[side].Core.Magnets[owner].GoTo != want {
					t.Fatalf("player%d input did not move its target on peer%d: got%d want%d tick%d", owner+1, side+1, worlds[side].Core.Magnets[owner].GoTo, want, tick)
				}
			}
		}
		if worlds[0].Core.RandomState() != worlds[1].Core.RandomState() {
			t.Fatal("actual player commands diverged after native startup")
		}
	}
}
