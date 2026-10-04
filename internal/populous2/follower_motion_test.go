package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

type nativeFollowerMotionActor struct {
	Kind, Owner, Flags, State, ReturnState, Speed uint8
	FixedX, FixedY, Animation, Next, Previous     uint16
	RenderedAnimation                             uint16
	VelocityX, VelocityY, Timer                   int16
	Population                                    int32
}

type nativeFollowerMotionStep struct {
	Tick                     int
	Actor                    nativeFollowerMotionActor
	Decision, Moved, Crossed bool
	CellHeadsSHA256          string
	RNG                      uint32
}

type nativeFollowerMotionFixture struct {
	Terrain   string
	Direction [2]int
	Speed     uint8
	Source    [2]uint16
	Target    [2]uint16
	Initial   nativeFollowerMotionActor
	Trace     []nativeFollowerMotionStep
}

// Independent native execution retains every fractional step, admission,
// cell-list relocation and rendered direction bank. The boundary stops before
// the next decision, leaving target selection and contact states separate.
func TestFollowerMotionAgainstOriginal68000Traces(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_motion_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures       []nativeFollowerMotionFixture
		ZeroSpeedProof struct {
			EnteredException bool
			ExceptionVector  int
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 72 || !catalog.ZeroSpeedProof.EnteredException || catalog.ZeroSpeedProof.ExceptionVector != 5 {
		t.Fatal("native follower-motion fixture catalog is incomplete")
	}
	rules, err := DecodeFollowerMotionRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fmt.Sprintf("%s-speed%d-%d,%d", fixture.Terrain, fixture.Speed, fixture.Direction[0], fixture.Direction[1]), func(t *testing.T) {
			actor := FollowerMotionActor{Kind: 2, Player: 0, State: 4, ReturnState: 2, Speed: fixture.Speed, X: int16(fixture.Source[0]), Y: int16(fixture.Source[1]), Population: 1000}
			var heads [4096]uint16
			heads[int(actor.X)>>8+(int(actor.Y)>>8)*64] = 52
			callbacks := FollowerMotionCallbacks{
				Admit: func(actor *FollowerMotionActor, x, y int16) bool {
					if fixture.Terrain == "water-boundary" {
						return int(x)>>8 != 32+fixture.Direction[0] || int(y)>>8 != 32+fixture.Direction[1]
					}
					return true
				},
				Move: func(actor *FollowerMotionActor, oldX, oldY int16) {
					heads[int(oldX)>>8+(int(oldY)>>8)*64] = 0
					heads[int(actor.X)>>8+(int(actor.Y)>>8)*64] = 52
				},
			}
			assert := func(want nativeFollowerMotionActor, tick int) {
				t.Helper()
				frame, rendered, ok := rules.Frame(actor)
				if !ok || len(frame.Layers) == 0 {
					t.Fatalf("update %d lacks its original direction image", tick)
				}
				got := nativeFollowerMotionActor{Kind: actor.Kind, Owner: actor.Player + 1, Flags: actor.Flags, State: actor.State, ReturnState: actor.ReturnState, Speed: actor.Speed, FixedX: uint16(actor.X), FixedY: uint16(actor.Y), Animation: uint16(actor.Animation), Next: actor.Next, Previous: actor.Previous, RenderedAnimation: uint16(rendered), VelocityX: actor.VX, VelocityY: actor.VY, Timer: actor.Timer, Population: actor.Population}
				if got != want {
					t.Fatalf("update %d actor mismatch:\nGo: %+v\n68000: %+v", tick, got, want)
				}
			}
			if err := rules.BeginLeg(&actor, int16(fixture.Target[0]), int16(fixture.Target[1])); err != nil {
				t.Fatal(err)
			}
			assert(fixture.Initial, 0)
			for _, expected := range fixture.Trace {
				step := rules.Tick(&actor, callbacks)
				assert(expected.Actor, expected.Tick)
				if step.Moved != expected.Moved || step.Crossed != expected.Crossed || step.NeedDecision != expected.Decision {
					t.Fatalf("update %d events %+v; native moved=%t crossed=%t decision=%t", expected.Tick, step, expected.Moved, expected.Crossed, expected.Decision)
				}
				var raw [8192]byte
				for index, head := range heads {
					binary.BigEndian.PutUint16(raw[index*2:], head)
				}
				if hash := fmt.Sprintf("%x", sha256.Sum256(raw[:])); hash != expected.CellHeadsSHA256 {
					t.Fatalf("update %d linked membership differs: %s", expected.Tick, hash)
				}
				if expected.RNG != 4311 {
					t.Fatal("isolated native walking unexpectedly consumed randomness")
				}
			}
		})
	}
}

func TestFollowerMotionZeroSpeedReportsNativeTrap(t *testing.T) {
	rules, err := DecodeFollowerMotionRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	actor := FollowerMotionActor{X: 8320, Y: 8320, Speed: 0, Timer: 77, VX: 3, VY: -4}
	before := actor
	if err := rules.BeginLeg(&actor, 8576, 8320); !errors.Is(err, ErrFollowerZeroSpeed) || actor != before {
		t.Fatal("native divide-by-zero case was accepted or partially changed")
	}
}

// The decision states were observed inside the original dispatcher on a
// slope. The callback supplies that independently recorded decision; this
// test verifies the motion controller's execution order, not search AI.
func TestFollowerMotionOriginalDecisionFallthrough(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_motion_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		DispatchProofs []struct {
			Before, Decided, After nativeFollowerMotionActor
			MotionEntries          int
			Prepasses              int
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.DispatchProofs) != 2 {
		t.Fatal("native full-dispatch proofs missing")
	}
	rules, err := DecodeFollowerMotionRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, proof := range catalog.DispatchProofs {
		a := proof.Before
		actor := FollowerMotionActor{Kind: a.Kind, Player: a.Owner - 1, Flags: a.Flags, State: a.State, ReturnState: a.ReturnState, Speed: a.Speed, X: int16(a.FixedX), Y: int16(a.FixedY), VX: a.VelocityX, VY: a.VelocityY, Timer: a.Timer, Animation: int(a.Animation), Population: a.Population}
		calls, prepasses := 0, 0
		step := rules.Tick(&actor, FollowerMotionCallbacks{Prepass: func(*FollowerMotionActor) bool {
			prepasses++
			return true
		}, Decide: func(actor *FollowerMotionActor) bool {
			calls++
			if actor.Animation != int(proof.Decided.Animation) || actor.X != int16(proof.Decided.FixedX) || actor.Y != int16(proof.Decided.FixedY) {
				t.Fatal("decision was dispatched before the original clock/expiry transition")
			}
			actor.State, actor.ReturnState = proof.Decided.State, proof.Decided.ReturnState
			actor.VX, actor.VY, actor.Timer = proof.Decided.VelocityX, proof.Decided.VelocityY, proof.Decided.Timer
			return true
		}})
		want := proof.After
		if calls != 1 || prepasses != proof.Prepasses || !step.Moved || step.NeedDecision || actor.X != int16(want.FixedX) || actor.Y != int16(want.FixedY) || actor.Animation != int(want.Animation) || actor.Timer != want.Timer || actor.State != want.State {
			t.Fatalf("original same-update dispatch differs: %+v %+v", actor, step)
		}
		if proof.Before.State == 4 && proof.MotionEntries != 2 {
			t.Fatal("native expiry did not reenter motion during the same update")
		}
	}
}
