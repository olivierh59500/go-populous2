package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type followerContextFixture struct {
	Input struct {
		Name, Mode                 string
		Initial                    []nativeHeroPatch
		D4, D5, D0, D1             uint32
		DefaultHeader, DefaultTile uint8
		OverrideTerrain            bool
	}
	D4, D5         uint32
	Hash, GridHash string
	SlotsVisited   int
	Visits         []struct {
		Reference    uint16
		Owner, State uint8
		D4, D5       uint32
	}
	Writes []struct {
		PC            uint32
		Register      int
		Before, After uint32
	}
}

func followerContextMemory(f followerContextFixture) *entryFixtureMemory {
	m := &entryFixtureMemory{}
	m.putWord(0xeb44, 8)
	m.putLong(0xeb28, 59500)
	m.putLong(0xf40, 600)
	for i := range 4096 {
		h, t := uint8(0xa8), uint8(15)
		if f.Input.OverrideTerrain {
			h, t = f.Input.DefaultHeader, f.Input.DefaultTile
		}
		m.bytes[0xf44+i*4], m.bytes[0xf45+i*4] = h, t
	}
	for side := 1; side <= 2; side++ {
		g, marker := 0xe76a+side*314, 0xe740+side*14
		m.putLong(g, 10000)
		m.putWord(g+0x18, uint16(side))
		m.putWord(g+12, 14)
		m.putWord(g+10, uint16(marker-0x76c0))
		m.bytes[marker], m.bytes[marker+12] = 20, uint8(side)
		m.putWord(marker+6, 0x1080)
		m.putWord(marker+8, 0x1080)
	}
	for _, p := range f.Input.Initial {
		switch p.Width {
		case 1:
			m.bytes[p.Address] = uint8(p.Value)
		case 2:
			m.putWord(p.Address, uint16(p.Value))
		case 4:
			m.putLong(p.Address, p.Value)
		}
	}
	return m
}

func followerContextTownCallbacks(m *entryFixtureMemory, c *NativeFollowerRegisterContext) NativeTownCallbacks {
	cb := townCombatEvaluatorCallbacks(m)
	cb.Context = c
	return cb
}

func followerContextEntryCallbacks(t *testing.T, m *entryFixtureMemory, c *NativeFollowerRegisterContext, town *NativeTownEvaluator) FollowerEntryCallbacks {
	t.Helper()
	cb := FollowerEntryCallbacks{Context: c, Read: m.read, Write: m.write,
		Head: func(p NativePackedTile) (NativeRecordReference, error) {
			return NativeRecordReference(m.word(0xf46 + (int(uint8(p))+int(uint8(p>>8))*64)*4)), nil
		},
		Node: func(ref NativeRecordReference) (FollowerEntryNode, error) {
			a := cleanupRecordAddress(ref)
			return FollowerEntryNode{Kind: m.bytes[a], Owner: m.bytes[a+12], Next: NativeRecordReference(m.word(a + 2))}, nil
		},
		ReadWord: func(ref NativeRecordReference, off uint16) (uint16, error) {
			return m.word(cleanupRecordAddress(ref) + int(off)), nil
		},
		ReadLong: func(ref NativeRecordReference, off uint16) (uint32, error) {
			return m.long(cleanupRecordAddress(ref) + int(off)), nil
		},
		WriteWord: func(ref NativeRecordReference, off, value uint16) error {
			m.putWord(cleanupRecordAddress(ref)+int(off), value)
			return nil
		},
		Tile: func(p NativePackedTile) (uint8, error) {
			return m.bytes[0xf45+(int(uint8(p))+int(uint8(p>>8))*64)*4], nil
		},
		GodMode: func(owner uint8) (uint16, error) { return m.word(heroGodAddress(owner) + 12), nil }, Tick: func() uint16 { return m.word(0xf42) },
		SetLeader: func(owner uint8, ref NativeRecordReference) error {
			m.putWord(heroGodAddress(owner)+8, uint16(ref))
			return nil
		},
		Selected: func() NativeRecordReference { return 0 }, Select: func(NativeRecordReference) error { return nil },
		Founded: func(owner uint8) error { g := heroGodAddress(owner); m.putWord(g+0x44, m.word(g+0x44)+1); return nil },
		EvaluateTown: func(ref NativeRecordReference) (int, error) {
			v, err := town.Evaluate(ref, m.word(0xf42), followerContextTownCallbacks(m, c))
			return int(v), err
		},
		ClearFarms: func(ref NativeRecordReference, tile uint8) error {
			return town.ClearFarms(ref, tile, followerContextTownCallbacks(m, c))
		},
		ClearHeroLinks: func(ref NativeRecordReference) error {
			var image NativeRecordImage
			copy(image.Bytes[:], m.bytes[NativeRecordImageStart:NativeRecordImageEnd])
			_, err := ClearEntryHeroLinks(&image, ref)
			copy(m.bytes[NativeRecordImageStart:NativeRecordImageEnd], image.Bytes[:])
			return err
		},
		Sound: func(uint16) error { return fmt.Errorf("unexpected sound in context fixture") },
	}
	cb.Unlink = func(ref NativeRecordReference) error {
		var grid NativeOccupancyState
		for i := range grid.Cells {
			a := 0xf44 + i*4
			grid.Cells[i] = NativeOccupancyCell{Header: m.bytes[a], Tile: m.bytes[a+1], Head: NativeRecordReference(m.word(a + 2))}
		}
		access := NativeRecordAccess{Record: func(ref NativeRecordReference) (NativeOccupancyRecord, bool) {
			a := cleanupRecordAddress(ref)
			return NativeOccupancyRecord{X: m.word(a + 6), Y: m.word(a + 8), Next: NativeRecordReference(m.word(a + 2)), Previous: NativeRecordReference(m.word(a + 4))}, true
		}, SetLinks: func(ref, next, prev NativeRecordReference) {
			a := cleanupRecordAddress(ref)
			m.putWord(a+2, uint16(next))
			m.putWord(a+4, uint16(prev))
		}}
		if err := grid.Remove(ref, access); err != nil {
			return err
		}
		for i, v := range grid.Cells {
			m.putWord(0xf46+i*4, uint16(v.Head))
		}
		return nil
	}
	return cb
}

func TestFollowerRegisterContextAgainstOriginalControllers(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_context_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []followerContextFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 351 {
		t.Fatal("native follower register catalog incomplete")
	}
	b := testBundle(t)
	decision, err := DecodeFollowerDecisionRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	hero, err := DecodeFollowerHeroRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := DecodeFollowerEntryRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range catalog.Cases {
		if f.Input.Mode == "" {
			continue
		}
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := followerContextMemory(f)
			c := NativeFollowerRegisterContext{D4: f.Input.D4, D5: f.Input.D5, AddressBase: 0x200000}
			memory := FollowerCleanupMemory{Read8: func(a int) (uint8, error) { return m.bytes[a], nil }, Read16: func(a int) (uint16, error) { return m.word(a), nil }, Read32: func(a int) (uint32, error) { return m.long(a), nil }, Write8: func(a int, v uint8) error { m.bytes[a] = v; return nil }, Write16: func(a int, v uint16) error { m.putWord(a, v); return nil }, Write32: func(a int, v uint32) error { m.putLong(a, v); return nil }}
			actor, err := m.read(52)
			if err != nil {
				t.Fatal(err)
			}
			switch f.Input.Mode {
			case "search":
				var grid NativeOccupancyState
				for i := range grid.Cells {
					a := 0xf44 + i*4
					grid.Cells[i] = NativeOccupancyCell{Header: m.bytes[a], Tile: m.bytes[a+1], Head: NativeRecordReference(m.word(a + 2))}
				}
				_, err = decision.Select(&actor.Motion, m.bytes[0x76f4+24], NativeFollowerMode(m.word(heroGodAddress(actor.Owner)+12)), &grid, FollowerDecisionCallbacks{Context: &c, Record: func(ref NativeRecordReference) (FollowerDecisionRecord, bool) {
					a := cleanupRecordAddress(ref)
					return FollowerDecisionRecord{Kind: m.bytes[a], Owner: m.bytes[a+12], Next: NativeRecordReference(m.word(a + 2))}, true
				}, Random: func() int {
					rng := m.long(0xeb28)
					if rng == 0 {
						rng = 0xbc614e
					}
					rng *= 0xbb40e62d
					m.putLong(0xeb28, rng)
					return int(rng >> 8 & 0x7fff)
				}})
				if err == nil {
					err = m.write(52, actor)
				}
			case "town":
				_, err = town.Evaluate(52, m.word(0xf42), followerContextTownCallbacks(m, &c))
			case "hero-target":
				_, err = hero.SelectTargetWithContext(52, memory, &c)
			case "hero-plan":
				_, err = hero.Plan(52, uint8(f.Input.D0), uint8(f.Input.D1), FollowerHeroCallbacks{Context: &c, Memory: memory, RaiseEnabled: func() bool { return false }, Raise: func(uint8, uint8) error { return fmt.Errorf("unexpected direct raise") }})
			case "leg":
				err = decision.Motion.BeginLegWithContext(&actor.Motion, int16(f.Input.D0), int16(f.Input.D1), &c)
				if err == nil {
					err = m.write(52, actor)
				}
			case "entry":
				_, err = entry.Enter(52, followerContextEntryCallbacks(t, m, &c, &town))
			case "lower":
				var grid NativeOccupancyState
				for i := range grid.Cells {
					a := 0xf44 + i*4
					grid.Cells[i] = NativeOccupancyCell{Header: m.bytes[a], Tile: m.bytes[a+1], Head: NativeRecordReference(m.word(a + 2))}
				}
				ai, e := DecodeNativeAIRules(b.Executable)
				if e != nil {
					t.Fatal(e)
				}
				var result NativeFollowerLowerContext
				result, err = ObserveNativeFollowerLower(&c, grid, ai.Raster, int16(f.Input.D0), int16(f.Input.D1))
				if err == nil {
					for i, cell := range result.Cells.Cells {
						a := 0xf44 + i*4
						m.bytes[a], m.bytes[a+1] = cell.Header, cell.Tile
						m.putWord(a+2, uint16(cell.Head))
					}
					if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[0xf44:0x4f44])); got != f.GridHash {
						t.Fatalf("original recursive lower map differs: got%s want%s", got, f.GridHash)
					}
				}
			default:
				t.Fatal("unknown register controller fixture")
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.D4 != f.D4 || c.D5 != f.D5 {
				t.Fatalf("native register continuation differs: got%08x/%08x want%08x/%08x", c.D4, c.D5, f.D4, f.D5)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[:])); f.Input.Mode != "lower" && got != f.Hash {
				t.Fatalf("native controller full BSS differs: got%s want%s", got, f.Hash)
			}
		})
	}
	if len(counts) != 7 {
		t.Fatal("native register controller scopes missing")
	}
}

func TestFollowerRegisterFullPassOriginalEvidence(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_context_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []followerContextFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	passes, states := 0, map[uint8]bool{}
	for _, f := range catalog.Cases {
		if f.Input.Mode != "" {
			continue
		}
		passes++
		if f.SlotsVisited != 400 || len(f.Visits) < 3 || f.Visits[0].Reference != 0 || f.Visits[len(f.Visits)-1].Reference != 399*52 {
			t.Fatal("original pass did not visit all physical follower slots")
		}
		for _, v := range f.Visits {
			if v.Owner != 0 && v.Reference != 0 {
				states[v.State] = true
			}
		}
		last := f.Visits[len(f.Visits)-1]
		if last.D4 != f.D4 || last.D5 != f.D5 {
			t.Fatal("inactive final slot or pass epilogue unexpectedly changed registers")
		}
		if f.Hash == "" {
			t.Fatal("full native pass memory evidence missing")
		}
	}
	if passes != 159 || len(states) != 35 {
		t.Fatalf("original full-pass scope differs: %d passes/%d states", passes, len(states))
	}
}

func TestFollowerNeutralContextAgainstOriginalFullPass(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_context_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []followerContextFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	b := testBundle(t)
	rules, err := DecodeNativeNeutralRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	primitive, err := DecodeNativePrimitiveCreatorRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range catalog.Cases {
		if !strings.HasPrefix(f.Input.Name, "neutral-pass-") {
			continue
		}
		checked++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := followerContextMemory(f)
			c := NativeFollowerRegisterContext{D4: f.Input.D4, D5: f.Input.D5, AddressBase: 0x200000}
			raw := FollowerCleanupMemory{Read8: func(a int) (uint8, error) { return m.bytes[a], nil }, Read16: func(a int) (uint16, error) { return m.word(a), nil }, Read32: func(a int) (uint32, error) { return m.long(a), nil }, Write8: func(a int, v uint8) error { m.bytes[a] = v; return nil }, Write16: func(a int, v uint16) error { m.putWord(a, v); return nil }, Write32: func(a int, v uint32) error { m.putLong(a, v); return nil }}
			random := func() int {
				rng := m.long(0xeb28)
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				m.putLong(0xeb28, rng)
				return int(rng >> 8 & 0x7fff)
			}
			pc := NativePrimitiveCreatorCallbacks{Memory: raw, Random: func() uint16 { return uint16(random()) }, Link: func(ref NativeRecordReference) error {
				tmp := scenarioScriptMemory(m.bytes)
				err := tmp.link(ref)
				m.bytes = [65536]byte(tmp)
				return err
			}}
			cb := NativeNeutralActorCallbacks{Memory: raw,
				Move: func(ref NativeRecordReference, x, y uint16) error {
					a := cleanupRecordAddress(ref)
					if m.word(a+6)>>8 != x>>8 || m.word(a+8)>>8 != y>>8 {
						return fmt.Errorf("context fixture unexpectedly crossed a map cell")
					}
					m.putWord(a+6, x)
					m.putWord(a+8, y)
					return nil
				},
				Unlink: func(NativeRecordReference) error {
					return fmt.Errorf("context fixture unexpectedly removed neutral actor")
				},
				Tile: func(p NativePackedTile) (uint8, error) {
					return m.bytes[0xf45+(int(uint8(p))+int(uint8(p>>8))*64)*4], nil
				},
				Head: func(p NativePackedTile) (NativeRecordReference, error) {
					return NativeRecordReference(m.word(0xf46 + (int(uint8(p))+int(uint8(p>>8))*64)*4)), nil
				},
				WriteTile: func(p NativePackedTile, v uint8) error {
					m.bytes[0xf45+(int(uint8(p))+int(uint8(p>>8))*64)*4] = v
					return nil
				}, Random: random,
				Lower: func(x, y uint8) error {
					var grid NativeOccupancyState
					for i := range grid.Cells {
						a := 0xf44 + i*4
						grid.Cells[i] = NativeOccupancyCell{Header: m.bytes[a], Tile: m.bytes[a+1], Head: NativeRecordReference(m.word(a + 2))}
					}
					result, err := ObserveNativeFollowerLower(&c, grid, rules.Raster, int16(x), int16(y))
					if err != nil {
						return err
					}
					for i, v := range result.Cells.Cells {
						a := 0xf44 + i*4
						m.bytes[a], m.bytes[a+1] = v.Header, v.Tile
						m.putWord(a+2, uint16(v.Head))
					}
					return nil
				},
				CreateWhirlwind:  func(x, y uint8, o uint16) error { _, err := primitive.CreateWhirlwind(o, x, y, pc); return err },
				PlantTree:        func(x, y uint8, _ uint16) error { _, err := primitive.PlantTree(x, y, pc); return err },
				CreateFireColumn: func(x, y uint8, o uint16) error { _, err := primitive.CreateFireColumn(o, x, y, pc); return err },
				Cleanup: func(NativeRecordReference, uint16) error {
					return fmt.Errorf("unexpected neighbor victim in context fixture")
				},
			}
			if _, err := rules.Tick(52, cb); err != nil {
				t.Fatal(err)
			}
			if c.D4 != f.D4 || c.D5 != f.D5 {
				t.Fatalf("neutral full-pass continuation differs: got%x/%x want%x/%x", c.D4, c.D5, f.D4, f.D5)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[0xf44:0x4f44])); got != f.GridHash {
				t.Fatalf("neutral full-pass terrain/heads differ: got%s want%s", got, f.GridHash)
			}
		})
	}
	if checked != 14 {
		t.Fatal("neutral context full-pass cases missing")
	}
}
