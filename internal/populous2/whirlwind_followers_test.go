package populous2

import (
	"encoding/binary"
	"testing"
)

func TestWhirlwindFollowerTransportAgainstOriginal68000(t *testing.T) {
	rules, err := DecodeWhirlwindFollowerRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, fixture := range nativeInteractionFixtures(t) {
		if fixture.Input.Mode != "lifted" && fixture.Input.Mode != "landing" {
			continue
		}
		counts[fixture.Input.Mode]++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := newNativeInteractionMemory(fixture.Input)
			at := interactionGroupBase + 52
			m.bytes[at], m.bytes[at+22] = 8, 0x14
			m.setWord(at+32, 20800)
			if fixture.Input.Mode == "lifted" {
				m.setWord(at+10, fixture.Input.Animation)
				m.setWord(interactionEffect+6, uint16((fixture.Input.X+2)*256+211))
				m.setWord(interactionEffect+8, uint16((fixture.Input.Y-1)*256+17))
			} else {
				// The release transition itself is independently verified by the33
				// existing release cases. Start landing from its centered one-victim
				// result and preserve all unrelated raw52-byte fields.
				m.bytes[at+25], m.bytes[at+22] = 1, 0x1a
				m.setWord(at+10, 0x68c)
				m.bytes[interactionEffect+12] = 0
				m.unlink(interactionEffect)
				m.move(52, uint16((fixture.Input.X+1)*256+128), uint16((fixture.Input.Y+1)*256+128))
				m.RNG *= 0xbb40e62d
			}
			f := WhirlwindFollower{Kind: m.bytes[at], Owner: m.bytes[at+12], Flags: m.bytes[at+13], State: m.bytes[at+22], Weapon: m.bytes[at+25], Next: NativeRecordReference(m.word(at + 2)), Previous: NativeRecordReference(m.word(at + 4)), X: m.word(at + 6), Y: m.word(at + 8), Animation: int(m.word(at + 10)), Population: int32(binary.BigEndian.Uint32(m.bytes[at+26:])), EffectReference: NativeRecordReference(m.word(at + 32))}
			cb := WhirlwindFollowerCallbacks{
				ReadSource: func(ref NativeRecordReference) (uint16, uint16, error) {
					source := m.address(ref)
					return m.word(source + 6), m.word(source + 8), nil
				},
				Move: func(f *WhirlwindFollower, x, y uint16) error {
					m.move(52, x, y)
					f.Next, f.Previous = NativeRecordReference(m.word(at+2)), NativeRecordReference(m.word(at+4))
					return nil
				},
			}
			ticks := 1
			if fixture.Input.Mode == "landing" {
				ticks = fixture.Input.Ticks
			}
			for i := 0; i < ticks; i++ {
				beforeState := f.State
				step, err := rules.Tick(&f, cb)
				if err != nil {
					t.Fatal(err)
				}
				if step.ReadyNextUpdate && beforeState != 0x1a {
					t.Fatal("ready event appeared outside landing terminal")
				}
				if f.State == 2 {
					break
				}
			}
			m.bytes[at], m.bytes[at+12], m.bytes[at+13], m.bytes[at+22], m.bytes[at+25] = f.Kind, f.Owner, f.Flags, f.State, f.Weapon
			m.setWord(at+2, uint16(f.Next))
			m.setWord(at+4, uint16(f.Previous))
			m.setWord(at+6, f.X)
			m.setWord(at+8, f.Y)
			m.setWord(at+10, uint16(f.Animation))
			m.setLong(at+26, uint32(f.Population))
			m.setWord(at+32, uint16(f.EffectReference))
			assertInteractionRecords(t, m, fixture)
			if m.RNG != fixture.RNG {
				t.Fatal("transport or landing consumed unexpected RNG")
			}
		})
	}
	if counts["lifted"] != 7 || counts["landing"] != 3 {
		t.Fatalf("native transport fixture coverage %+v", counts)
	}
}

func TestWhirlwindFollowerLandingDefersDecision(t *testing.T) {
	rules, err := DecodeWhirlwindFollowerRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	f := WhirlwindFollower{Kind: 8, Owner: 2, State: 0x1a, Weapon: 1, Animation: 0x68c, Population: 0, EffectReference: 20800, X: 8576, Y: 8576}
	for tick := 1; tick <= 7; tick++ {
		step, err := rules.Tick(&f, WhirlwindFollowerCallbacks{})
		if err != nil {
			t.Fatal(err)
		}
		if step.ReadyNextUpdate != (tick == 7) {
			t.Fatal("landing completion does not match native seven dispatches")
		}
	}
	if f.Kind != 2 || f.State != 2 || f.Animation != 0 || f.Population != 0 || f.Owner != 2 || f.Weapon != 1 || f.EffectReference != 20800 {
		t.Fatal("landing altered retained native identity fields")
	}
}

func TestWhirlwindFollowerReadsInactiveSourceAndSameTile(t *testing.T) {
	rules, err := DecodeWhirlwindFollowerRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	f := WhirlwindFollower{Kind: 8, Owner: 1, State: 0x14, Animation: 0x4d4, Population: -1, EffectReference: 20800, X: 8320, Y: 8320}
	calls := 0
	_, err = rules.Tick(&f, WhirlwindFollowerCallbacks{ReadSource: func(ref NativeRecordReference) (uint16, uint16, error) {
		if ref != 20800 {
			t.Fatal("raw source reference changed")
		}
		return 8340, 8300, nil
	}, Move: func(_ *WhirlwindFollower, x, y uint16) error { calls++; return nil }})
	if err != nil || calls != 1 || f.X != 8340 || f.Y != 8300 || f.Population != -1 {
		t.Fatal("native owner-based transport or same-cell write was omitted")
	}
}
