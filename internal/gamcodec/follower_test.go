package gamcodec

import (
	"encoding/binary"
	"testing"

	"go-populous2/internal/engine"
)

func TestGAMAdvancedFollowerLifecyclesUseSemanticArtworkRoles(t *testing.T) {
	catalog := Catalog{AnimationRoles: map[uint16][]AnimationRole{100: {{Name: "airborne/follower", Frame: 3}}, 104: {{Name: "airborne/follower", Frame: 11}}, 200: {{Name: "lightning/recovery", Frame: 2}}, 204: {{Name: "lightning/recovery", Frame: 9}}, 300: {{Name: "conversion/blue", Frame: 4}}, 400: {{Name: "death/burning", Frame: 7}}, 404: {{Name: "death/burning", Frame: 20}}, 500: {{Name: "neutral/road-maker", Frame: 1}}}}
	for _, name := range []string{"carried", "lightning-recovery", "conversion", "burning", "neutral"} {
		t.Run(name, func(t *testing.T) {
			w := &engine.World{}
			id := 1
			w.Followers[id] = engine.Follower{Owner: 0, X: 32, Y: 33, State: engine.Walking, Population: 555, Weapons: 1}
			switch name {
			case "carried":
				w.Followers[id].State = engine.Airborne
				w.Air.Carry[id] = engine.AirCarryState{Phase: engine.AirCarryFlying, Effect: 3, Frame: 3, Frames: 12}
			case "lightning-recovery":
				w.AirVictims[id] = engine.LightningVictimState{Phase: engine.LightningVictimRecovery, Sequence: engine.LightningRecoverySequence, Bolt: 3, Frame: 2, Frames: 10}
			case "conversion":
				w.Followers[id].State = engine.Converting
				w.Followers[id].Conversion = engine.ConversionState{Active: true, Frame: 4, SourceOwner: 0}
			case "burning":
				w.Followers[id].State = engine.Ruin
				w.FireDamage.Deaths[id] = engine.FireVictimDeath{Mode: engine.FireVictimBurning, Frame: 7, Frames: 21}
			case "neutral":
				w.Followers[id].Owner = 2
				w.Followers[id].Population = 0
				w.Followers[id].Neutral.Kind = engine.NeutralRoadMaker
				w.Followers[id].Frame = 1
			}
			record := make([]byte, 52)
			record[0], record[12], record[22] = 2, w.Followers[id].Owner+1, 2
			record[6], record[8] = 32, 33
			record[25] = 1
			binary.BigEndian.PutUint32(record[26:], uint32(w.Followers[id].Population))
			if err := encodeFollowerLifecycle(record, w, id, catalog); err != nil {
				t.Fatal(err)
			}
			var snapshot engine.Snapshot
			got, _, err := decodeFollower(record, id, &snapshot, catalog)
			if err != nil {
				t.Fatal(err)
			}
			if got.Owner != w.Followers[id].Owner || got.State != w.Followers[id].State || got.Population != w.Followers[id].Population {
				t.Fatalf("GAM follower identity changed: %+v", got)
			}
			switch name {
			case "carried":
				if snapshot.World.Air.Carry[id] != w.Air.Carry[id] {
					t.Fatal("carry controller changed")
				}
			case "lightning-recovery":
				if snapshot.World.AirVictims[id] != w.AirVictims[id] {
					t.Fatal("lightning recovery changed")
				}
			case "conversion":
				if got.Conversion != w.Followers[id].Conversion {
					t.Fatal("conversion changed")
				}
			case "burning":
				if snapshot.World.FireDamage.Deaths[id] != w.FireDamage.Deaths[id] {
					t.Fatal("burning controller changed")
				}
			case "neutral":
				if got.Neutral.Kind != engine.NeutralRoadMaker || got.Frame != 1 {
					t.Fatal("neutral invention changed")
				}
			}
		})
	}
}

func TestGAMCombatAndTerrainDeathsRoundTripNamedContinuations(t *testing.T) {
	catalog := Catalog{AnimationRoles: map[uint16][]AnimationRole{100: {{Name: "combat/attack", Frame: 1}}, 200: {{Name: "combat/death", Frame: 3}}, 204: {{Name: "combat/death", Frame: 11}}, 300: {{Name: "combat/victory-blue", Frame: 4}}, 304: {{Name: "combat/victory-blue", Frame: 13}}, 400: {{Name: "death/water", Frame: 1}}, 404: {{Name: "death/water", Frame: 2}}}}
	for _, name := range []string{"combat", "defeated", "victorious", "water-death"} {
		t.Run(name, func(t *testing.T) {
			w := &engine.World{}
			id := 1
			w.Followers[id] = engine.Follower{Owner: 0, X: 32, Y: 33, State: engine.Walking, Population: 1000, Frame: 1}
			switch name {
			case "combat":
				w.Followers[id].State = engine.Fighting
				w.Followers[id].BattleWith = 2
				w.Followers[id].BattleAggressor = true
			case "defeated":
				w.Followers[id].State = engine.Ruin
				w.Followers[id].CombatAftermath = engine.CombatAftermathState{Kind: engine.CombatDefeated, Frame: 3, Frames: 12}
			case "victorious":
				w.Followers[id].CombatAftermath = engine.CombatAftermathState{Kind: engine.CombatVictorious, Frame: 4, Frames: 14}
			case "water-death":
				w.Followers[id].State = engine.Ruin
				w.Followers[id].TerrainDeath = engine.TerrainDeathState{Active: true, Frame: 1, Frames: 3}
			}
			record := make([]byte, 52)
			record[0], record[12], record[22], record[6], record[8] = 2, 1, 2, 32, 33
			binary.BigEndian.PutUint32(record[26:], 1000)
			if err := encodeFollowerLifecycle(record, w, id, catalog); err != nil {
				t.Fatal(err)
			}
			got, _, err := decodeFollower(record, id, &engine.Snapshot{}, catalog)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != w.Followers[id].State || got.CombatAftermath != w.Followers[id].CombatAftermath || got.TerrainDeath != w.Followers[id].TerrainDeath || got.BattleWith != w.Followers[id].BattleWith || got.BattleAggressor != w.Followers[id].BattleAggressor {
				t.Fatalf("combat/death continuation changed: %+v", got)
			}
		})
	}
}

func TestGAMArrivingContactWithoutRepresentedTargetRejects(t *testing.T) {
	data := make([]byte, FileSize)
	at := 0x76c0 + 52 - fileStart
	data[at], data[at+12], data[at+22], data[at+23] = 2, 1, 4, 12
	var snapshot engine.Snapshot
	snapshot.World.Followers[1] = engine.Follower{Owner: 0, X: 20, Y: 20, State: engine.Walking}
	snapshot.World.Occupants[20+20*64] = 1
	if err := decodeContacts(fileReader{data}, &snapshot); err == nil {
		t.Fatal("unrepresented arriving contact was silently reset")
	}
}

func TestGAMHeroContactWaitingPreservesSilentReplyFrame(t *testing.T) {
	catalog := Catalog{AnimationRoles: map[uint16][]AnimationRole{900: {{Name: "contact/perseus", Frame: 1}}, 904: {{Name: "contact/perseus", Frame: 3}}}}
	w := &engine.World{}
	w.Followers[1] = engine.Follower{Owner: 0, X: 20, Y: 20, State: engine.Walking, Population: 500, Frame: 1, ContactWaiting: true, ContactWait: 7, Hero: engine.HeroState{Kind: engine.HeroPerseus}}
	record := make([]byte, 52)
	record[0], record[12], record[22], record[6], record[8], record[13] = 2, 1, 2, 20, 20, 2
	binary.BigEndian.PutUint32(record[26:], 500)
	if err := encodeFollowerLifecycle(record, w, 1, catalog); err != nil {
		t.Fatal(err)
	}
	got, _, err := decodeFollower(record, 1, &engine.Snapshot{}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ContactWaiting || got.ContactWait != 7 || got.Frame != 1 || got.Hero.Kind != engine.HeroPerseus {
		t.Fatal("hero waiting reply changed during GAM conversion")
	}
}

func TestGAMFollowerAppearanceSelectorPreservesEveryVariant(t *testing.T) {
	for variant := 0; variant < 8; variant++ {
		record := make([]byte, 52)
		record[0], record[12], record[22], record[6], record[8] = 2, 1, 2, 20, 20
		binary.BigEndian.PutUint16(record[50:], uint16(variant*2))
		f, _, err := decodeFollower(record, 1, &engine.Snapshot{}, Catalog{})
		if err != nil {
			t.Fatal(err)
		}
		if int(f.AppearanceVariant) != variant {
			t.Fatal("file variant was derived from the follower index")
		}
	}
	record := make([]byte, 52)
	record[0], record[12], record[22] = 2, 1, 2
	binary.BigEndian.PutUint16(record[50:], 3)
	if _, _, err := decodeFollower(record, 1, &engine.Snapshot{}, Catalog{}); err == nil {
		t.Fatal("odd appearance selector was accepted")
	}
}
