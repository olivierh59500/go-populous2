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
