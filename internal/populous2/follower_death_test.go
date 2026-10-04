package populous2

import (
	"bytes"
	"reflect"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestFollowerDeathSaveRetainsAnimationAndReleasesReservation(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove}}
	w.burnFireCell(2000%64, 2000/64)
	for range 3 {
		w.tickFlameDeaths()
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for len(w.FlameDeaths) > 0 {
		w.tickFlameDeaths()
		restored.tickFlameDeaths()
		if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
			left, right := reflect.ValueOf(w.Snapshot()), reflect.ValueOf(restored.Snapshot())
			for field := 0; field < left.NumField(); field++ {
				if !reflect.DeepEqual(left.Field(field).Interface(), right.Field(field).Interface()) {
					t.Logf("saved death continuation differs in %s", left.Type().Field(field).Name)
				}
			}
			for index := range w.NativeFollowers {
				if w.NativeFollowers[index] != restored.NativeFollowers[index] {
					t.Logf("follower %d: live=%+v restored=%+v", index, w.NativeFollowers[index], restored.NativeFollowers[index])
					break
				}
			}
			t.Fatal("saved death animation changed slot release or occupancy")
		}
	}
	if restored.Core.FollowerReserved(0) || restored.Core.MapWho[2000] != 0 {
		t.Fatal("completed saved death kept its follower reservation")
	}
}

func TestFollowerDeathSaveRejectsUnboundedAndDuplicateReservations(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove}}
	w.burnFireCell(2000%64, 2000/64)
	for _, mutate := range []func(*Snapshot){
		func(s *Snapshot) { s.FlameDeaths[0].End += 4 },
		func(s *Snapshot) { s.FlameDeaths[0].End = 1 << 30 },
		func(s *Snapshot) { s.FlameDeaths[0].Animation++ },
		func(s *Snapshot) {
			s.FlameDeaths[0].Animation = 0x1a0
			s.FlameDeaths[0].End = 0x1a0 + w.FireColumns.SequenceLengths[0x1a0]*4
		},
		func(s *Snapshot) { s.FlameDeaths = append(s.FlameDeaths, s.FlameDeaths[0]) },
	} {
		s := w.Snapshot()
		mutate(&s)
		if _, err := Restore(testBundle(t), s); err == nil {
			t.Fatal("invalid death lifetime/duplicate reservation was accepted")
		}
	}
}
