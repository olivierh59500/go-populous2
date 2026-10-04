package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func lightningWorld(t *testing.T, seed uint32) *World {
	t.Helper()
	w := oceanWorld(t, seed)
	for index := range w.Core.Alt {
		w.Core.Alt[index] = 1
	}
	for index := range w.Core.MapBlk {
		w.Core.MapBlk[index] = legacy.FlatBlock
	}
	w.initializeActorGraph()
	w.bindFollowerMotion()
	w.bindHeroCombat()
	w.bindFlameDeaths()
	w.bindFollowerHazards()
	w.bindLightningVictims()
	w.bindActorGraphHooks()
	return w
}

func lightningWorldRaw(w *World, slot int) string {
	a := w.NativeEffects[slot]
	graph := w.Occupancy.Effects[slot].Record
	var raw [32]byte
	raw[0] = a.Kind
	for offset, value := range map[int]uint16{2: uint16(graph.Next), 4: uint16(graph.Previous), 6: uint16(a.X), 8: uint16(a.Y), 10: uint16(a.Animation), 14: uint16(a.VX), 16: uint16(a.VY), 20: uint16(a.Timer), 24: uint16(a.Life), 26: uint16(w.LightningState.Word26[slot]), 28: uint16(w.LightningState.Word28[slot]), 30: w.LightningState.RandomWords[slot]} {
		binary.BigEndian.PutUint16(raw[offset:], value)
	}
	if a.Active {
		raw[12] = a.Player + 1
	}
	raw[18], raw[22] = a.Speed, a.State
	return hex.EncodeToString(raw[:])
}

func TestLightningWorldLifecycleAgainstNativeTraces(t *testing.T) {
	data, err := os.ReadFile("testdata/lightning_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			Name    string
			XP      uint8
			X, Y    int
			Initial nativeLightningSnapshot
			Trace   []nativeLightningSnapshot
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Fixtures) != 5 {
		t.Fatalf("invalid native lightning lifecycle catalog: %v", err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			w := lightningWorld(t, 4311)
			w.Experience[0][Air] = fixture.XP
			if fixture.Name == "full-pool" {
				for index := 1; index < len(w.NativeEffects); index++ {
					w.NativeEffects[index].Active, w.NativeEffects[index].Player = true, 2
				}
			}
			before := w.Core.Magnets[0].Mana
			if !w.PlaceLightning(0, fixture.X, fixture.Y) || w.Core.Magnets[0].Mana != before {
				t.Fatal("native marker creation was charged/rejected")
			}
			if !w.ActivateLightning(0) || w.Core.Magnets[0].Mana != before-w.ManaCost(0, Lightning) {
				t.Fatal("activation debit differs from native command30")
			}
			compare := func(want nativeLightningSnapshot) {
				t.Helper()
				for _, actor := range want.Actors {
					if actual := lightningWorldRaw(w, actor.Slot); actual != actor.Raw {
						t.Fatalf("update%d slot%d: world%s native%s", want.Tick, actor.Slot, actual, actor.Raw)
					}
				}
				var tiles [4096]byte
				for pos := range tiles {
					tiles[pos] = w.nativeTileAt(pos%64, pos/64)
				}
				if uint16(w.LightningState.Markers[0]) != want.MarkerReference || w.Core.Snapshot().RNG != want.RNG || fmt.Sprintf("%x", sha256.Sum256(tiles[:])) != want.TilesSHA256 {
					t.Fatalf("update%d marker/RNG/terrain differs from native", want.Tick)
				}
			}
			compare(fixture.Initial)
			for _, update := range fixture.Trace {
				if fixture.Name == "center-xp32" && update.Tick == 30 {
					w.PlaceLightning(0, 35, 34)
				}
				w.tickNativeEffects()
				compare(update)
			}
		})
	}
}

func TestLightningWorldPartialVolleySaveAndDismissal(t *testing.T) {
	w := lightningWorld(t, 4311)
	if !w.PlaceLightning(0, 32, 32) || !w.ActivateLightning(0) {
		t.Fatal("native lightning setup rejected")
	}
	for range 20 {
		w.tickNativeEffects()
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 30 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
		if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
			t.Fatal("loading lightning changed bolt chain, grid or RNG")
		}
	}
	w.DismissLightning(0)
	restored.DismissLightning(0)
	for range 20 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("loading lightning changed dismissal/outro")
	}
}

func TestLightningManagedVictimUsesNativeDamageAndReservation(t *testing.T) {
	w := lightningWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 1, Population: 100, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}}
	w.initializeNativeFollower(0)
	w.NativeEffects[0] = NativeEffectActor{Active: true, Kind: 0x2a, Player: 0, X: 8320, Y: 8320, State: 24}
	w.linkEffect(0)
	ref := nativeActorReference(NativeFollowerPool, 0)
	v, _ := w.lightningRecord(ref)
	v.State, v.Animation, v.EffectReference = 0x1c, 0x738, nativeActorReference(NativeEffectPool, 0)
	w.setLightningRecord(ref, v)
	w.Core.TickWithComputer([2]bool{})
	if w.Core.Peeps[0].Population != 90 || w.Core.Magnets[1].Population != 0 || !w.Core.FollowerReserved(0) {
		t.Fatalf("native managed damage/hidden totals/reservation differs: population=%d total=%d", w.Core.Peeps[0].Population, w.Core.Magnets[1].Population)
	}
	for range 20 {
		w.Core.TickWithComputer([2]bool{})
	}
	if !w.LightningVictims[0].Active || !w.Core.FollowerReserved(0) {
		t.Fatal("managed zero-population record was reused before bolt cleanup")
	}
	// The native owner byte still allocates the record even when the signed
	// damage result is nonpositive. Preserve it through the Go save format.
	w.NativeEffects[0].Kind = 0x22 // A reused positive-owner slot is also alive.
	w.NativeEffects[0].State, w.NativeEffects[0].Animation, w.NativeEffects[0].Speed, w.NativeEffects[0].Life = 2, 0x1a0, 16, 100
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Core.Peeps[0].Population != w.Core.Peeps[0].Population || !restored.LightningVictims[0].Active {
		t.Fatal("managed signed population or stale positive-owner effect identity changed after loading")
	}
	w.NativeEffects[0].Active = false
	w.unlinkActor(NativeEffectPool, 0)
	w.Core.TickWithComputer([2]bool{})
	if w.LightningVictims[0].Victim.State != 0x20 || !w.Core.FollowerReserved(0) {
		t.Fatal("native dead-bolt transition did not retain its death animation")
	}
	for count := 0; count < 100 && w.LightningVictims[0].Active; count++ {
		w.Core.TickWithComputer([2]bool{})
	}
	if w.LightningVictims[0].Active || w.Core.FollowerReserved(0) {
		t.Fatal("finished lightning death retained its slot")
	}
}

func TestLightningActivationDebitAndNoInstantAreaDamage(t *testing.T) {
	w := lightningWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: pos, Flags: legacy.OnMove}, {Player: 1, Population: 100, AtPos: pos + 1, Flags: legacy.OnMove}}
	w.reconcileActorGraph()
	before := w.Core.Magnets[0].Mana
	if !w.ActivateLightning(0) || w.Core.Magnets[0].Mana != before-w.ManaCost(0, Lightning) {
		t.Fatal("native activation without a marker incorrectly skipped its command debit")
	}
	before = w.Core.Magnets[0].Mana
	if !w.Cast(0, Lightning, Target{X: 32, Y: 32}) || w.Core.Magnets[0].Mana != before || w.Core.Peeps[0].Population != 100 || w.Core.Peeps[1].Population != 100 {
		t.Fatal("placing the marker charged mana or retained generic immediate radius damage")
	}
}

func TestLightningPartialPoolKeepsNativeDebitAndCreatedBolt(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Experience[0][Air] = 255
	if !w.PlaceLightning(0, 32, 32) {
		t.Fatal("marker creation failed")
	}
	for index := 2; index < len(w.NativeEffects); index++ {
		w.NativeEffects[index].Active = true
	}
	before := w.Core.Magnets[0].Mana
	if !w.ActivateLightning(0) || w.Core.Magnets[0].Mana != before-w.ManaCost(0, Lightning) || w.NativeEffects[1].Kind != 0x2a || !w.NativeEffects[1].Active {
		t.Fatal("partial native volley was discarded or refunded")
	}
	marker := w.LightningState.Markers[0]
	slot, _ := lightningSlot(marker)
	if w.LightningState.Word26[slot] != nativeActorReference(NativeEffectPool, 1) {
		t.Fatal("partial volley lost its original marker chain")
	}
	before = w.Core.Magnets[0].Mana
	if !w.ActivateLightning(0) || w.Core.Magnets[0].Mana != before-w.ManaCost(0, Lightning) {
		t.Fatal("busy marker activation skipped the native command debit")
	}
}

func TestLightningWaterPrepassYieldsWithoutTakingDamage(t *testing.T) {
	w := lightningWorld(t, 4311)
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}}
	w.initializeNativeFollower(0)
	v, _ := w.lightningRecord(nativeActorReference(NativeFollowerPool, 0))
	v.State, v.Animation = 0x1c, 0x738
	w.setLightningRecord(nativeActorReference(NativeFollowerPool, 0), v)
	for _, point := range []int{32 + 32*65, 33 + 32*65, 32 + 33*65, 33 + 33*65} {
		w.Core.Alt[point] = 0
	}
	w.Core.MapBlk[pos] = 0
	w.Core.TickWithComputer([2]bool{})
	if w.LightningVictims[0].Active || w.Core.Peeps[0].Population != 1000 || w.Core.Peeps[0].Flags&legacy.InWater == 0 {
		t.Fatalf("lightning ownership overrode native first water entry: managed=%t population=%d flags=%02x", w.LightningVictims[0].Active, w.Core.Peeps[0].Population, w.Core.Peeps[0].Flags)
	}
}

func TestEarlierLightningRadiusEffectMigratesToNativeMarker(t *testing.T) {
	w := lightningWorld(t, 4311)
	s := w.Snapshot()
	s.Version = 13
	s.Effects = []Effect{{Spell: Lightning, Player: 0, X: 32, Y: 32, Life: 4}}
	restored, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	ref := restored.LightningState.Markers[0]
	if ref == 0 || len(restored.Effects) != 0 || len(s.Effects) != 1 {
		t.Fatal("legacy lightning radius effect was replayed or caller data changed")
	}
	if linked, _ := restored.Occupancy.Linked(ref); !linked {
		t.Fatal("migrated native marker did not enter the retained actor graph")
	}
	if _, err := Restore(testBundle(t), restored.Snapshot()); err != nil {
		t.Fatalf("migrated lightning could not be saved again: %v", err)
	}
}

func TestLightningWorldManagedDamageAgainstNativeVictimStates(t *testing.T) {
	data, err := os.ReadFile("testdata/lightning_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		VictimFixtures []nativeLightningVictimFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.VictimFixtures) != 205 {
		t.Fatalf("invalid native victim fixture catalog: %v", err)
	}
	checked := 0
	for _, fixture := range catalog.VictimFixtures {
		if fixture.Input.Mode != "victim" || fixture.Input.Kind != 2 || fixture.Input.Hero >= 0 || fixture.Input.State != 0x1c {
			continue
		}
		checked++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			w := lightningWorld(t, 4311)
			pos := 32 + 32*64
			w.Core.Peeps = []legacy.Peep{{Player: fixture.Input.Owner - 1, Population: int(fixture.Initial.Population), AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}}
			w.initializeNativeFollower(0)
			w.NativeEffects[0] = NativeEffectActor{Active: fixture.Input.BoltOwner != 0, Kind: fixture.Input.BoltKind, Player: fixture.Input.BoltOwner - 1, State: 24}
			v := LightningVictim{Kind: fixture.Initial.Kind, Owner: fixture.Initial.Owner, Flags: fixture.Initial.Flags, State: fixture.Initial.State, Animation: int(fixture.Initial.Animation), Population: fixture.Initial.Population, EffectReference: NativeRecordReference(fixture.Initial.Reference)}
			w.LightningVictims[0] = NativeLightningFollower{Active: true, Victim: v}
			w.NativeFollowers[0].Active = false
			for _, want := range fixture.Trace {
				w.updateLightningVictim(0)
				got := w.LightningVictims[0].Victim
				if got.State != want.State || got.Animation != int(want.Animation) || got.Population != want.Population || int32(w.Core.Peeps[0].Population) != want.Population || got.Owner != want.Owner {
					t.Fatalf("managed world victim differs: got%+v native%+v", got, want)
				}
			}
		})
	}
	if checked != 66 {
		t.Fatalf("world replay covered %d native ordinary-victim cases; expected66", checked)
	}
}

func TestLightningSaveRejectsBrokenMarkerAndVolleyReferences(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.PlaceLightning(0, 32, 32)
	w.ActivateLightning(0)
	for _, mutate := range []func(*Snapshot){
		func(s *Snapshot) { s.LightningState.Markers[0] = 52 },
		func(s *Snapshot) { s.LightningState.Word28[1] = 52 },
		func(s *Snapshot) { s.LightningState.Word26[0] = nativeActorReference(NativeEffectPool, 0) },
		func(s *Snapshot) { s.LightningState.Word26[1] = nativeActorReference(NativeEffectPool, 1) },
	} {
		s := w.Snapshot()
		mutate(&s)
		if _, err := Restore(testBundle(t), s); err == nil {
			t.Fatal("broken native marker/bolt relation accepted")
		}
	}
}
