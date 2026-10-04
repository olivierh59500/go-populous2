package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

// The original traces stop before the next target decision. Replay their
// complete fixed-leg portion through the real Core hook, admission, position,
// occupancy and rendering adapters, without translating search AI here.
func TestFollowerWorldFixedLegsAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_motion_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Fixtures []nativeFollowerMotionFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fmt.Sprintf("%s-%d-%v", fixture.Terrain, fixture.Speed, fixture.Direction), func(t *testing.T) {
			w := flatGroundWorld(t)
			w.Scenery = [SceneryCapacity]SceneryActor{}
			w.rebuildSceneryIndex()
			w.Marks = [4096]Mark{}
			if fixture.Terrain == "slope" {
				for y := 0; y <= 64; y++ {
					for x := 0; x <= 64; x++ {
						w.Core.Alt[x+y*65] = clamp(x-30, 0, 8)
					}
				}
			} else if fixture.Terrain == "water-boundary" {
				x, y := 32+fixture.Direction[0], 32+fixture.Direction[1]
				for _, vertex := range []int{x + y*65, x + 1 + y*65, x + (y+1)*65, x + 1 + (y+1)*65} {
					w.Core.Alt[vertex] = 0
				}
			}
			w.Core.Peeps = []legacy.Peep{{Flags: legacy.OnMove, Player: 0, Population: 1000, AtPos: 32 + 32*64, MovementSpeed: fixture.Speed}}
			w.Core.MapWho = [4096]uint16{}
			w.Core.MapWho[32+32*64] = 1
			w.initializeNativeFollower(0)
			initial := fixture.Initial
			a := &w.NativeFollowers[0].Actor
			a.X, a.Y, a.VX, a.VY, a.Timer, a.State, a.ReturnState, a.Animation = int16(initial.FixedX), int16(initial.FixedY), initial.VelocityX, initial.VelocityY, initial.Timer, initial.State, initial.ReturnState, int(initial.Animation)
			for _, expected := range fixture.Trace {
				if expected.Decision {
					break
				}
				w.Core.TickWithComputer([2]bool{})
				a = &w.NativeFollowers[0].Actor
				_, rendered, ok := w.FollowerMotion.Frame(*a)
				if !ok {
					t.Fatal("native frame unavailable")
				}
				got := nativeFollowerMotionActor{Kind: a.Kind, Owner: a.Player + 1, Flags: a.Flags, State: a.State, ReturnState: a.ReturnState, Speed: a.Speed, FixedX: uint16(a.X), FixedY: uint16(a.Y), Animation: uint16(a.Animation), Next: a.Next, Previous: a.Previous, RenderedAnimation: uint16(rendered), VelocityX: a.VX, VelocityY: a.VY, Timer: a.Timer, Population: a.Population}
				if got != expected.Actor {
					t.Fatalf("tick%d: Go %+v; native %+v", expected.Tick, got, expected.Actor)
				}
				if position := (int(a.X) >> 8) + (int(a.Y)>>8)*64; w.Core.Peeps[0].AtPos != position {
					t.Fatal("Core position did not commit at native crossing")
				}
				var heads [8192]byte
				for index, head := range w.Core.MapWho {
					binary.BigEndian.PutUint16(heads[index*2:], head*52)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(heads[:])); got != expected.CellHeadsSHA256 {
					t.Fatal("single-follower native membership differs")
				}
			}
		})
	}
}

func TestFollowerWorldSavedFractionalContinuation(t *testing.T) {
	w := flatGroundWorld(t)
	w.Scenery = [SceneryCapacity]SceneryActor{}
	w.rebuildSceneryIndex()
	w.Core.Peeps = []legacy.Peep{{Flags: legacy.OnMove, Player: 0, Population: 100000, AtPos: 32 + 32*64, MovementSpeed: 20}, {Flags: legacy.OnMove, Player: 1, Population: 100000, AtPos: 44 + 44*64, MovementSpeed: 40}}
	w.Core.MapWho = [4096]uint16{}
	w.Core.MapWho[32+32*64] = 1
	w.Core.MapWho[44+44*64] = 2
	w.Core.Magnets[0].Flags, w.Core.Magnets[1].Flags = legacy.MagnetMode, legacy.MagnetMode
	w.Core.Magnets[0].Carried, w.Core.Magnets[1].Carried = 0, 0
	w.Core.Magnets[0].GoTo, w.Core.Magnets[1].GoTo = 40+32*64, 36+44*64
	w.initializeNativeFollower(0)
	w.initializeNativeFollower(1)
	for range 5 {
		w.Core.TickWithComputer([2]bool{})
	}
	if uint8(w.NativeFollowers[0].Actor.X) == 128 {
		t.Fatal("capture did not reach a fractional state")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 65 {
		w.Core.TickWithComputer([2]bool{})
		restored.Core.TickWithComputer([2]bool{})
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("saved fractional motion diverged")
	}
}

func TestFollowerWorldReuseStartsNewGeneration(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Flags: legacy.OnMove, Player: 0, Population: 100, AtPos: 2000, MovementSpeed: 40}, {Player: 0, Population: 0, AtPos: 2000}}
	w.NativeFollowers[1] = NativeFollower{Active: true, Generation: 17, Actor: FollowerMotionActor{Kind: 2, Player: 0, X: 123, Y: 456, VX: 91, VY: -27, Speed: 20, Timer: 8, State: 4, Animation: 12}}
	if slot := w.Core.AllocateHeroClone(0); slot != 1 {
		t.Fatal("expected reused slot")
	}
	a := w.NativeFollowers[1]
	if a.Generation != 18 || !a.Active || a.Actor.X != int16((2000%64)*256+128) || a.Actor.Y != int16((2000/64)*256+128) || a.Actor.VX != 0 || a.Actor.VY != 0 || a.Actor.Timer != 0 || a.Actor.Speed != 40 || a.Actor.Animation != 0 {
		t.Fatalf("reused slot retained motion: %+v", a)
	}
}

func TestFollowerWorldMagnetDefersFirstStep(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Flags: legacy.OnMove, Player: 0, Population: 10000, AtPos: 2000, MovementSpeed: 20}}
	w.Core.MapWho = [4096]uint16{}
	w.Core.MapWho[2000] = 1
	w.Core.Magnets[0].Flags, w.Core.Magnets[0].Carried, w.Core.Magnets[0].GoTo = legacy.MagnetMode, 0, 2005
	w.initializeNativeFollower(0)
	before := w.NativeFollowers[0].Actor.X
	w.Core.TickWithComputer([2]bool{})
	a := w.NativeFollowers[0].Actor
	if a.X != before || a.State != 4 || a.ReturnState != 18 || a.Timer != 12 {
		t.Fatalf("magnet did not defer: %+v", a)
	}
	w.Core.TickWithComputer([2]bool{})
	if w.NativeFollowers[0].Actor.X != before+20 {
		t.Fatal("magnet first step has wrong native speed")
	}
}

func TestFollowerWorldContactOccursOnceAtNativeCrossing(t *testing.T) {
	w := flatGroundWorld(t)
	w.Scenery = [SceneryCapacity]SceneryActor{}
	w.rebuildSceneryIndex()
	w.Core.Peeps = []legacy.Peep{{Flags: legacy.OnMove, Player: 0, Population: 10000, AtPos: 2000, MovementSpeed: 20}, {Flags: legacy.InTown, Player: 1, Population: 10000, AtPos: 2001}}
	w.Core.MapWho = [4096]uint16{}
	w.Core.MapWho[2000], w.Core.MapWho[2001] = 1, 2
	w.initializeNativeFollower(0)
	w.reconcileActorGraph()
	a := &w.NativeFollowers[0].Actor
	if err := w.FollowerMotion.BeginLeg(a, a.X+256, a.Y); err != nil {
		t.Fatal(err)
	}
	a.State = 4
	contacts := 0
	w.Core.BeforeBattle = func(int, int) bool { contacts++; return true }
	for range 6 {
		w.updateNativeFollower(0)
	}
	if contacts != 0 || w.Core.Peeps[0].AtPos != 2000 {
		t.Fatal("contact preceded fractional cell crossing")
	}
	w.updateNativeFollower(0)
	if contacts != 1 || w.Core.Peeps[0].AtPos != 2001 {
		t.Fatal("native crossing did not resolve exactly one contact")
	}
}

func TestFollowerWorldSaveMigrationAndValidation(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Flags: legacy.OnMove, Player: 0, Population: 1000, AtPos: 2000, MovementSpeed: 20}}
	w.Core.MapWho = [4096]uint16{}
	w.Core.MapWho[2000] = 1
	w.initializeNativeFollower(0)
	s := w.Snapshot()
	s.Version = 11
	s.NativeFollowers = [legacy.MaxPeeps]NativeFollower{}
	s.Core.Peeps[0].MovementSpeed = 0
	restored, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	a := restored.NativeFollowers[0]
	if !a.Active || a.Generation == 0 || uint8(a.Actor.X) != 128 || uint8(a.Actor.Y) != 128 || a.Actor.Speed != restored.Level.Players[0].MovementSpeed() {
		t.Fatal("old save did not initialize centered native motion")
	}
	for _, change := range []func(*NativeFollower){
		func(n *NativeFollower) { n.Generation = 0 },
		func(n *NativeFollower) { n.Actor.Speed = 0 },
		func(n *NativeFollower) { n.Actor.X = -1 },
		func(n *NativeFollower) { n.Actor.State = 6 },
		func(n *NativeFollower) { n.Actor.VX = 99 },
		func(n *NativeFollower) { n.Actor.Next = 1 },
	} {
		invalid := w.Snapshot()
		change(&invalid.NativeFollowers[0])
		if _, err := Restore(testBundle(t), invalid); err == nil {
			t.Fatal("invalid saved motion record accepted")
		}
	}
}
