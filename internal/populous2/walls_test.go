package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func wallRulesForTest(t *testing.T) WallRules {
	t.Helper()
	rules, err := DecodeWallRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

// These actor/head fingerprints come from executing the original relocated
// $1626c and $161cc on a 68000, not from a second Go implementation.
func TestWallsAgainstNative68000ActorStates(t *testing.T) {
	type placement struct{ player, x, y int }
	fixtures := []struct {
		name     string
		calls    []placement
		tiles    map[int]uint8
		ticks    int
		accepted []bool
		sha256   string
	}{
		{"turn", []placement{{0, 32, 32}, {0, 33, 32}, {0, 34, 32}, {0, 34, 33}, {0, 33, 33}}, nil, 0, []bool{true, true, true, true, true}, "b507958740f45e37cdddf4585d329bc248a33f1a07b37d5717fbb1cf62b8ca67"},
		{"disconnected-head", []placement{{0, 32, 32}, {0, 40, 40}, {0, 33, 32}}, nil, 0, []bool{true, false, true}, "e36051ea0c282a0eb138aeb7e947d6e164caf58547d4aabc85d9698e41ecd57c"},
		{"opposing", []placement{{0, 32, 32}, {1, 33, 32}, {0, 34, 32}}, nil, 0, []bool{true, true, true}, "983c5a5ac11f12ea8e81db190e44c0c348f763c26db01d6c20b29f397f9adb26"},
		{"cardinals", []placement{{0, 32, 32}, {0, 32, 31}, {0, 33, 32}, {0, 31, 32}, {0, 32, 33}}, nil, 0, []bool{true, true, true, true, true}, "ec2a69b74188622768a6fe849ea4c8966a976d1efe74dabd82b50aa8f3027cc1"},
		{"road-gate", []placement{{0, 32, 32}, {0, 33, 32}}, map[int]uint8{32 + 32*64: 197, 33 + 32*64: 198}, 0, []bool{true, true}, "940374aff4b3375bbe8f957cc1b2689b8fead1fe2c9bf48466ae4e9be89ff8e6"},
		{"roads-cross", []placement{{0, 32, 32}, {0, 33, 32}, {0, 34, 32}}, map[int]uint8{32 + 32*64: 201, 33 + 32*64: 202, 34 + 32*64: 216}, 0, []bool{true, true, true}, "6134e9eaf6d37b85ac8fcdb300ca6b001453fd8baed4c1caed6e710a856678b5"},
		{"water-and-farms", []placement{{0, 32, 32}, {0, 33, 32}, {1, 34, 32}, {0, 34, 32}}, map[int]uint8{32 + 32*64: 0, 33 + 32*64: 47, 34 + 32*64: 63}, 0, []bool{false, true, true, false}, "33a008a236caf25992130ed83250df827ce5cd66fbc9a50964797fd98abf777f"},
		{"edges", []placement{{0, 0, 0}, {0, 1, 0}, {1, 63, 63}, {1, 63, 62}}, nil, 0, []bool{true, true, true, true}, "50612b5a1478376300437d05f47d8642a72838809b028712c88b396107109209"},
		{"terminal-animation", []placement{{0, 32, 32}, {0, 33, 32}, {0, 33, 33}}, nil, 16, []bool{true, true, true}, "9e06817bf89cdf7e1e906eb2b587771e455be04f5b9fb435fd0c6545afdc9f68"},
	}
	rules := wallRulesForTest(t)
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			var state WallState
			tileAt := func(x, y int) uint8 {
				if tile, found := fixture.tiles[x+y*64]; found {
					return tile
				}
				return 15
			}
			for index, call := range fixture.calls {
				if got := state.Place(&rules, call.player, call.x, call.y, tileAt); got != fixture.accepted[index] {
					t.Fatalf("placement %d accepted=%t, native=%t", index, got, fixture.accepted[index])
				}
			}
			for tick := 0; tick < fixture.ticks; tick++ {
				state.Tick(&rules, tileAt)
			}
			if got := wallFingerprint(state); got != fixture.sha256 {
				t.Fatalf("wall actors differ from native: %s", got)
			}
		})
	}
}

func wallFingerprint(state WallState) string {
	var canonical bytes.Buffer
	fmt.Fprintf(&canonical, "%d|%d\n", state.Heads[0], state.Heads[1])
	for _, actor := range state.Actors {
		if !actor.Active && actor.Next == 0 {
			continue
		}
		active, player := 0, -1
		if actor.Active {
			active, player = 1, int(actor.Player)
		}
		fmt.Fprintf(&canonical, "%d|%d|%d|%d|%d|%d|%d\n", active, player, actor.X, actor.Y, actor.Variant, actor.Animation, actor.Next)
	}
	return fmt.Sprintf("%x", sha256.Sum256(canonical.Bytes()))
}

func TestWallCapacityRemovalAndSnapshot(t *testing.T) {
	rules := wallRulesForTest(t)
	var state WallState
	tiles := [4096]uint8{}
	for index := range tiles {
		tiles[index] = 15
	}
	tileAt := func(x, y int) uint8 { return tiles[x+y*64] }
	for index := 0; index < WallCapacity; index++ {
		y, x := index/64, index%64
		if y%2 == 1 {
			x = 63 - x
		}
		if !state.Place(&rules, 0, x, y, tileAt) {
			t.Fatalf("connected native pool allocation %d failed", index)
		}
	}
	before := state
	if state.Place(&rules, 0, 8, 3, tileAt) || state != before {
		t.Fatal("full native wall pool accepted or modified a cast")
	}
	last := state.Actors[WallCapacity-1]
	tiles[last.X+last.Y*64] = 0
	state.Tick(&rules, tileAt)
	if state.Actors[WallCapacity-1].Active || state.Heads[0] != WallCapacity-1 {
		t.Fatal("invalid underlying terrain did not remove the head wall")
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var restored WallState
	if err := json.Unmarshal(data, &restored); err != nil || restored != state {
		t.Fatalf("wall actor state did not round-trip: %v", err)
	}
}

func TestWallArtConstructionHoldsNativeTerminalFrame(t *testing.T) {
	rules := wallRulesForTest(t)
	var state WallState
	tileAt := func(int, int) uint8 { return 15 }
	if !state.Place(&rules, 0, 32, 32, tileAt) {
		t.Fatal("first wall rejected")
	}
	initial := rules.Layers(state.Actors[0])
	state.Tick(&rules, tileAt)
	state.Tick(&rules, tileAt)
	terminal := state.Actors[0]
	if len(initial) == 0 || len(rules.Layers(terminal)) == 0 || reflect.DeepEqual(initial, rules.Layers(terminal)) {
		t.Fatal("native construction layers were not decoded or advanced")
	}
	for tick := 0; tick < 20; tick++ {
		state.Tick(&rules, tileAt)
	}
	if state.Actors[0] != terminal {
		t.Fatal("native terminal wall frame looped instead of holding")
	}
	if _, err := DecodeWallRules(nil); err == nil {
		t.Fatal("missing native wall tables accepted")
	}
}

func TestWallValidationAllowsNativeReferenceQuirks(t *testing.T) {
	rules := wallRulesForTest(t)
	var state WallState
	tileAt := func(int, int) uint8 { return 15 }
	if !state.Place(&rules, 0, 32, 32, tileAt) || state.Place(&rules, 0, 40, 40, tileAt) {
		t.Fatal("native disconnected-head fixture failed")
	}
	if err := state.Validate(&rules); err != nil {
		t.Fatalf("inactive native head rejected: %v", err)
	}
	if !state.Place(&rules, 1, 40, 40, tileAt) || state.Heads[0] != state.Heads[1] {
		t.Fatal("native stale head was not reused by the other deity")
	}
	if err := state.Validate(&rules); err != nil {
		t.Fatalf("native cross-owner stale head rejected: %v", err)
	}
	state.Actors[1].Next = 2
	if err := state.Validate(&rules); err != nil {
		t.Fatalf("native self reference rejected: %v", err)
	}
	for _, corrupt := range []struct {
		name string
		edit func(*WallState)
	}{
		{"head", func(s *WallState) { s.Heads[0] = WallCapacity + 1 }},
		{"inactive-reference", func(s *WallState) { s.Actors[20].Next = WallCapacity + 1 }},
		{"owner", func(s *WallState) { s.Actors[0].Player = 2 }},
		{"x", func(s *WallState) { s.Actors[0].X = 64 }},
		{"y", func(s *WallState) { s.Actors[0].Y = -1 }},
		{"animation", func(s *WallState) { s.Actors[0].Animation = 123456 }},
	} {
		t.Run(corrupt.name, func(t *testing.T) {
			copy := state
			corrupt.edit(&copy)
			if err := copy.Validate(&rules); err == nil {
				t.Fatal("malformed wall save accepted")
			}
		})
	}
}

// Every fixture below records the original relocated $141a2 result and its
// actual wall/hero state changes, including strict threshold boundaries.
func TestWallCrossingAgainstNative68000(t *testing.T) {
	fixtures := []struct {
		player        int
		xp            uint8
		population    int
		hero          bool
		movement      int
		wallKind      uint8
		wallAnimation int
		heroState     uint8
		heroAnimation int
	}{
		{0, 0, 1, false, -2, 26, 1468, 0, 0},
		{0, 0, 1, true, -2, 26, 1468, 0, 0},
		{0, 0, 1000, false, -2, 26, 1468, 0, 0},
		{0, 0, 1000, true, -2, 26, 1468, 0, 0},
		{0, 0, 2999, false, -2, 26, 1468, 0, 0},
		{0, 0, 2999, true, -2, 26, 1468, 0, 0},
		{0, 0, 3000, false, -2, 26, 1468, 0, 0},
		{0, 0, 3000, true, -2, 26, 1468, 0, 0},
		{0, 0, 35767, false, -2, 26, 1468, 0, 0},
		{0, 0, 35767, true, -2, 26, 1468, 0, 0},
		{0, 0, 35768, false, -2, 26, 1468, 0, 0},
		{0, 0, 35768, true, -2, 26, 1468, 0, 0},
		{0, 0, 35769, false, 0, 26, 1468, 0, 0},
		{0, 0, 35769, true, 0, 26, 1468, 0, 0},
		{0, 0, 52767, false, 0, 26, 1468, 0, 0},
		{0, 0, 52767, true, 0, 26, 1468, 0, 0},
		{0, 0, 52768, false, 0, 26, 1468, 0, 0},
		{0, 0, 52768, true, 0, 26, 1468, 0, 0},
		{0, 0, 52769, false, 0, 28, 1960, 0, 0},
		{0, 0, 52769, true, 0, 28, 1960, 42, 1996},
		{0, 32, 1, false, -2, 26, 1468, 0, 0},
		{0, 32, 1, true, -2, 26, 1468, 0, 0},
		{0, 32, 1000, false, -2, 26, 1468, 0, 0},
		{0, 32, 1000, true, -2, 26, 1468, 0, 0},
		{0, 32, 2999, false, -2, 26, 1468, 0, 0},
		{0, 32, 2999, true, -2, 26, 1468, 0, 0},
		{0, 32, 3000, false, -2, 26, 1468, 0, 0},
		{0, 32, 3000, true, -2, 26, 1468, 0, 0},
		{0, 32, 39863, false, -2, 26, 1468, 0, 0},
		{0, 32, 39863, true, -2, 26, 1468, 0, 0},
		{0, 32, 39864, false, -2, 26, 1468, 0, 0},
		{0, 32, 39864, true, -2, 26, 1468, 0, 0},
		{0, 32, 39865, false, 0, 26, 1468, 0, 0},
		{0, 32, 39865, true, 0, 26, 1468, 0, 0},
		{0, 32, 56863, false, 0, 26, 1468, 0, 0},
		{0, 32, 56863, true, 0, 26, 1468, 0, 0},
		{0, 32, 56864, false, 0, 26, 1468, 0, 0},
		{0, 32, 56864, true, 0, 26, 1468, 0, 0},
		{0, 32, 56865, false, 0, 28, 1960, 0, 0},
		{0, 32, 56865, true, 0, 28, 1960, 42, 1996},
		{0, 255, 1, false, -2, 26, 1468, 0, 0},
		{0, 255, 1, true, -2, 26, 1468, 0, 0},
		{0, 255, 1000, false, -2, 26, 1468, 0, 0},
		{0, 255, 1000, true, -2, 26, 1468, 0, 0},
		{0, 255, 2999, false, -2, 26, 1468, 0, 0},
		{0, 255, 2999, true, -2, 26, 1468, 0, 0},
		{0, 255, 3000, false, -2, 26, 1468, 0, 0},
		{0, 255, 3000, true, -2, 26, 1468, 0, 0},
		{0, 255, 68407, false, -2, 26, 1468, 0, 0},
		{0, 255, 68407, true, -2, 26, 1468, 0, 0},
		{0, 255, 68408, false, -2, 26, 1468, 0, 0},
		{0, 255, 68408, true, -2, 26, 1468, 0, 0},
		{0, 255, 68409, false, 0, 26, 1468, 0, 0},
		{0, 255, 68409, true, 0, 26, 1468, 0, 0},
		{0, 255, 85407, false, 0, 26, 1468, 0, 0},
		{0, 255, 85407, true, 0, 26, 1468, 0, 0},
		{0, 255, 85408, false, 0, 26, 1468, 0, 0},
		{0, 255, 85408, true, 0, 26, 1468, 0, 0},
		{0, 255, 85409, false, 0, 28, 1960, 0, 0},
		{0, 255, 85409, true, 0, 28, 1960, 42, 1996},
		{1, 0, 1, false, -2, 26, 1468, 0, 0},
		{1, 0, 1, true, -2, 26, 1468, 0, 0},
		{1, 0, 1000, false, -2, 26, 1468, 0, 0},
		{1, 0, 1000, true, -2, 26, 1468, 0, 0},
		{1, 0, 2999, false, -2, 26, 1468, 0, 0},
		{1, 0, 2999, true, -2, 26, 1468, 0, 0},
		{1, 0, 3000, false, -2, 26, 1468, 0, 0},
		{1, 0, 3000, true, -2, 26, 1468, 0, 0},
		{1, 0, 68535, false, -2, 26, 1468, 0, 0},
		{1, 0, 68535, true, -2, 26, 1468, 0, 0},
		{1, 0, 68536, false, -2, 26, 1468, 0, 0},
		{1, 0, 68536, true, -2, 26, 1468, 0, 0},
		{1, 0, 68537, false, 0, 26, 1468, 0, 0},
		{1, 0, 68537, true, 0, 26, 1468, 0, 0},
		{1, 0, 85535, false, 0, 26, 1468, 0, 0},
		{1, 0, 85535, true, 0, 26, 1468, 0, 0},
		{1, 0, 85536, false, 0, 26, 1468, 0, 0},
		{1, 0, 85536, true, 0, 26, 1468, 0, 0},
		{1, 0, 85537, false, 0, 28, 1960, 0, 0},
		{1, 0, 85537, true, 0, 28, 1960, 42, 2004},
		{1, 32, 1, false, -2, 26, 1468, 0, 0},
		{1, 32, 1, true, -2, 26, 1468, 0, 0},
		{1, 32, 1000, false, -2, 26, 1468, 0, 0},
		{1, 32, 1000, true, -2, 26, 1468, 0, 0},
		{1, 32, 2999, false, -2, 26, 1468, 0, 0},
		{1, 32, 2999, true, -2, 26, 1468, 0, 0},
		{1, 32, 3000, false, -2, 26, 1468, 0, 0},
		{1, 32, 3000, true, -2, 26, 1468, 0, 0},
		{1, 32, 72631, false, -2, 26, 1468, 0, 0},
		{1, 32, 72631, true, -2, 26, 1468, 0, 0},
		{1, 32, 72632, false, -2, 26, 1468, 0, 0},
		{1, 32, 72632, true, -2, 26, 1468, 0, 0},
		{1, 32, 72633, false, 0, 26, 1468, 0, 0},
		{1, 32, 72633, true, 0, 26, 1468, 0, 0},
		{1, 32, 89631, false, 0, 26, 1468, 0, 0},
		{1, 32, 89631, true, 0, 26, 1468, 0, 0},
		{1, 32, 89632, false, 0, 26, 1468, 0, 0},
		{1, 32, 89632, true, 0, 26, 1468, 0, 0},
		{1, 32, 89633, false, 0, 28, 1960, 0, 0},
		{1, 32, 89633, true, 0, 28, 1960, 42, 2004},
		{1, 255, 1, false, -2, 26, 1468, 0, 0},
		{1, 255, 1, true, -2, 26, 1468, 0, 0},
		{1, 255, 1000, false, -2, 26, 1468, 0, 0},
		{1, 255, 1000, true, -2, 26, 1468, 0, 0},
		{1, 255, 2999, false, -2, 26, 1468, 0, 0},
		{1, 255, 2999, true, -2, 26, 1468, 0, 0},
		{1, 255, 3000, false, -2, 26, 1468, 0, 0},
		{1, 255, 3000, true, -2, 26, 1468, 0, 0},
		{1, 255, 101175, false, -2, 26, 1468, 0, 0},
		{1, 255, 101175, true, -2, 26, 1468, 0, 0},
		{1, 255, 101176, false, -2, 26, 1468, 0, 0},
		{1, 255, 101176, true, -2, 26, 1468, 0, 0},
		{1, 255, 101177, false, 0, 26, 1468, 0, 0},
		{1, 255, 101177, true, 0, 26, 1468, 0, 0},
		{1, 255, 118175, false, 0, 26, 1468, 0, 0},
		{1, 255, 118175, true, 0, 26, 1468, 0, 0},
		{1, 255, 118176, false, 0, 26, 1468, 0, 0},
		{1, 255, 118176, true, 0, 26, 1468, 0, 0},
		{1, 255, 118177, false, 0, 28, 1960, 0, 0},
		{1, 255, 118177, true, 0, 28, 1960, 42, 2004},
	}
	rules := wallRulesForTest(t)
	for index, fixture := range fixtures {
		decision := rules.DecideCrossing(fixture.player, fixture.player^1, fixture.xp, fixture.population, fixture.hero)
		movement, kind := 0, uint8(26)
		if decision.Crossing == WallBlocked {
			movement = -2
		}
		if decision.Crossing == WallBreak {
			kind = 28
		}
		if movement != fixture.movement || kind != fixture.wallKind || decision.HeroState != fixture.heroState || decision.HeroAnimation != fixture.heroAnimation {
			t.Fatalf("native wall movement fixture %d differs: %+v", index, decision)
		}
		var state WallState
		state.Actors[0] = WallActor{Active: true, Player: uint8(fixture.player ^ 1), X: 33, Y: 32, Animation: 0x5bc}
		if decision.Crossing == WallBreak && !state.Break(&rules, 0) {
			t.Fatal("native wall break failed")
		}
		if state.Actors[0].Animation != fixture.wallAnimation {
			t.Fatalf("fixture %d wall animation differs", index)
		}
		if !state.Actors[0].Active || len(rules.Layers(state.Actors[0])) == 0 {
			t.Fatal("broken wall actor or native art disappeared")
		}
		if decision.Crossing == WallBreak && state.At(33, 32) != -1 {
			t.Fatal("native broken actor still blocks as an intact wall")
		}
	}
	if got := rules.DecideCrossing(0, 0, 0, 1, false); got.Crossing != WallPass {
		t.Fatal("own wall did not bypass strength comparison")
	}
}
