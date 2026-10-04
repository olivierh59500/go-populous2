package populous2

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestFungusWorldPrepassAgainstNativeMortalityFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/fungus_hazard_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeFungusHazardCase }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 17 {
		t.Fatalf("invalid native mortality fixture catalog: %v", err)
	}
	for _, fixture := range catalog.Cases {
		if fixture.Owner == 3 {
			// The inherited two-side pool cannot represent native neutral
			// actors yet. The independent rules test covers their bypass.
			continue
		}
		t.Run(fixture.Name, func(t *testing.T) {
			w := flatGroundWorld(t)
			pos, player := 2000, fixture.Owner-1
			w.Marks[pos] = Mark{Spell: Fungus, Life: 1, Persistent: true, NativeTile: fixture.Tile}
			w.Core.Peeps = []legacy.Peep{{Player: player, Population: 100, AtPos: pos, Flags: legacy.OnMove}}
			w.Core.MapWho[pos] = 1
			if fixture.Hero >= 0 {
				w.Heroes[0] = Hero{Active: true, Player: int(player), Spell: heroIDs[fixture.Hero], Population: 100}
			}
			if fixture.InitialSubstate == 0x3a {
				w.Core.Magnets[player].Carried = 1
				w.Core.Magnets[player].Flags = legacy.MagnetMode
			}
			w.Core.BeforeFollower(0)
			if w.Core.Peeps[0].Population != fixture.Population || w.Core.MapWho[pos]*52 != fixture.Occupancy {
				t.Fatal("world mortality/retained occupancy differs from native prepass")
			}
			if fixture.Kind == 2 {
				if len(w.FlameDeaths) != 0 || w.HazardSerial != 0 {
					t.Fatal("native survivor entered a death animation")
				}
				return
			}
			if len(w.FlameDeaths) != 1 {
				t.Fatal("native death lacks its retained world record")
			}
			death := w.FlameDeaths[0]
			if death.Kind != fixture.Kind || death.State != fixture.State || death.Animation != fixture.Animation || w.LastHazardCue*10 != int(fixture.SoundArguments[0]) {
				t.Fatal("world death metadata or sound differs from native prepass")
			}
		})
	}
}

func TestMatureFungusRetainsNativeFollowerDeathAndCue(t *testing.T) {
	w := flatGroundWorld(t)
	pos := 2000
	w.Marks[pos] = Mark{Spell: Fungus, Life: 1, Persistent: true, NativeTile: 146}
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: pos, Flags: legacy.OnMove}}
	w.Core.TickWithComputer([2]bool{})
	if w.Core.Peeps[0].Population != 0 || len(w.FlameDeaths) != 1 || !w.Core.FollowerReserved(0) || w.Core.MapWho[pos] != 1 {
		t.Fatal("fungus skipped its retained death record/occupancy")
	}
	death := w.FlameDeaths[0]
	if death.Animation != 0x7dc || death.Kind != 0x10 || death.State != 0x38 || w.LastHazardCue != 26 || w.HazardSerial != 1 {
		t.Fatalf("native fungus death state or cue differs: %+v", death)
	}
	if _, ok := testBundle(t).FollowerDeathFrame(death.Animation); !ok {
		t.Fatal("native fungus death frame unavailable to rendering/audio")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for len(w.FlameDeaths) > 0 {
		w.tickFlameDeaths()
		restored.tickFlameDeaths()
		if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
			t.Fatal("fungus death load changed animation, cue or release")
		}
	}
	if w.Core.FollowerReserved(0) || w.Core.MapWho[pos] != 0 {
		t.Fatal("fungus death completion retained occupancy")
	}
}

func TestFreshFungusAndAdonisRetainNativeImmunity(t *testing.T) {
	for _, tile := range []uint8{145, 146, 150, 151} {
		for _, spell := range heroIDs {
			w := flatGroundWorld(t)
			pos := 2000
			w.Marks[pos] = Mark{Spell: Fungus, Life: 1, Persistent: true, NativeTile: tile}
			w.Core.Peeps = []legacy.Peep{{Player: 1, Population: 100, AtPos: pos, Flags: legacy.OnMove, Status: legacy.KnightStatus}}
			w.Heroes[0] = Hero{Active: true, Spell: spell, Player: 1, Population: 100}
			w.Core.BeforeFollower(0)
			property := w.GroundRules.Properties[tile]
			fatal := property&0x10 != 0 && property&1 == 0 && spell != Adonis
			if (len(w.FlameDeaths) != 0) != fatal || (w.Core.Peeps[0].Population == 0) != fatal {
				t.Fatalf("tile %d hero %d: native fungus immunity differs", tile, spell)
			}
			if fatal && w.FlameDeaths[0].Animation != w.FungusHazards.HeroDeath[heroIndex(spell)] {
				t.Fatal("fungus substituted ordinary hero death art")
			}
		}
	}
}

func TestFungusPrepassRunsBeforeOrdinaryFollowerDispatch(t *testing.T) {
	w := flatGroundWorld(t)
	pos := 2000
	w.Marks[pos] = Mark{Spell: Fungus, Life: 1, Persistent: true, NativeTile: 145}
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: pos, Flags: legacy.OnMove, Frame: 6}}
	// Keep this group on its current tile while it completes the inherited
	// animation decision; the native movement adapter will replace that path.
	w.Core.SkipFollower = func(int) bool { return true }
	w.Core.TickWithComputer([2]bool{})
	if w.Core.Peeps[0].Population == 0 || w.HazardSerial != 0 {
		t.Fatal("fresh fungus was immediately fatal")
	}
	w.Marks[pos].NativeTile = 146
	w.Core.TickWithComputer([2]bool{})
	if w.Core.Peeps[0].Population != 0 || w.HazardSerial != 1 {
		t.Fatal("mature fungus prepass was bypassed by follower state dispatch")
	}
}
