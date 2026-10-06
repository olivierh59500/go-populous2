package populous2

import (
	"encoding/json"
	"fmt"
	"go-populous2/internal/engine"
	"os"
	"testing"
)

// TestIndependentEngineCampaignReferenceOptional compares one complete source
// main frame per independent Step, including source rendering, clock,
// simulation and deferred commands with the same actual campaign record.
func TestIndependentEngineCampaignReferenceOptional(t *testing.T) {
	compareIndependentMainFrames(t, 160)
}

// compareIndependentMainFrames includes source rendering, clock advancement,
// simulation and deferred commands while supplying no player actions.
func compareIndependentMainFrames(t *testing.T, frames int) {
	if os.Getenv("POPULOUS2_REFERENCE_COMPARE") != "1" {
		t.Skip("optional local reference comparison")
	}
	data, err := os.ReadFile("testdata/native_runtime_campaign_gameplay_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Input        struct{ World int }
			Startup      nativeStockSnapshot
			StartupPolls []int
			FrameInputs  []struct {
				Tick    int
				Samples []nativeStockSample
				Polls   []int
			}
		}
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	sample := corpus.Cases[0]
	for _, candidate := range corpus.Cases {
		if candidate.Input.World == 12 {
			sample = candidate
			break
		}
	}
	g := nativeRuntimeGameplayStart(t, sample.Startup, sample.StartupPolls)
	h, c := g.Host, g.Frame
	bundle := testBundle(t)
	rawLevel := bundle.Levels[sample.Input.World]
	campaign, err := engine.DecodeCampaign(bundle.Raw["conquest.pak"])
	if err != nil {
		t.Fatal(err)
	}
	land, err := engine.DecodeLandscape(bundle.Raw[fmt.Sprintf("land%d.dat", rawLevel.Terrain)])
	if err != nil {
		t.Fatal(err)
	}
	independent, err := engine.NewWorld(campaign[sample.Input.World], land)
	if err != nil {
		t.Fatal(err)
	}
	_ = g.newFrame(t, nil)
	first := map[string]int{}
	compare := func(pass int) {
		heightRules, e := DecodeNativeRenderFrameRules(h.Bundle.Executable)
		if e != nil {
			t.Fatal(e)
		}
		if e = heightRules.BindCode(h.Memory.Code, h.Code.Logical().Read32); e != nil {
			t.Fatal(e)
		}
		for at := range independent.Heights {
			q := NativeFrameRegisterContext{D: [8]uint32{uint32(at % 65), uint32(at / 65)}, AddressBase: h.Memory.BSSBase}
			if e = heightRules.TerrainHeight(h.Memory.BSS, &q); e != nil {
				t.Fatal(e)
			}
			height := int(int16(q.D[2]))
			if height != int(independent.Heights[at]) {
				if _, ok := first["terrain"]; !ok {
					first["terrain"] = pass
					t.Logf("pass%d authoritative terrain corner%d Go%d reference%d", pass, at, independent.Heights[at], height)
				}
				break
			}
		}

		for owner := 0; owner < 2; owner++ {
			metric, e := h.Memory.BSS.Read16(0xe76a + (owner+1)*314 + 0x44)
			if e != nil {
				t.Fatal(e)
			}
			if independent.Players[owner].Statistics.Metric != metric {
				key := fmt.Sprintf("metric%d", owner)
				if _, ok := first[key]; !ok {
					first[key] = pass
					t.Logf("pass%d %s Go%d reference%d", pass, key, independent.Players[owner].Statistics.Metric, metric)
				}
			}

			mana, e := h.Memory.BSS.Read32(0xe76a + (owner+1)*314)
			if e != nil {
				t.Fatal(e)
			}
			if int(mana) != independent.Players[owner].Mana {
				key := fmt.Sprintf("mana%d", owner)
				if _, ok := first[key]; !ok {
					first[key] = pass
					t.Logf("pass%d %s Go%d reference%d", pass, key, independent.Players[owner].Mana, mana)
				}
			}
		}
		rng, e := h.Memory.BSS.Read32(0xeb28)
		if e != nil {
			t.Fatal(e)
		}
		if uint32(independent.Snapshot().Random) != rng {
			if _, ok := first["rng"]; !ok {
				first["rng"] = pass
				t.Logf("pass%d RNG Go%x reference%x", pass, independent.Snapshot().Random, rng)
			}
		}
		for id := 1; id < 400; id++ {
			at := 0x76c0 + id*52
			owner, _ := h.Memory.BSS.Read8(at + 12)
			kind, _ := h.Memory.BSS.Read8(at)
			state, _ := h.Memory.BSS.Read8(at + 22)
			population, _ := h.Memory.BSS.Read32(at + 26)
			x, _ := h.Memory.BSS.Read16(at + 6)
			y, _ := h.Memory.BSS.Read16(at + 8)
			stage, _ := h.Memory.BSS.Read8(at + 1)
			f := independent.Followers[id]
			active := owner != 0
			fx, fy := f.Position()
			stateMatch := !active && f.State == engine.Inactive || active && ((kind == 4 && f.State == engine.Town) || (kind == 2 && f.State == engine.Walking) || (state == 14 || state == 16) && f.State == engine.Fighting)
			if active != (f.State != engine.Inactive) || active && (int(owner)-1 != int(f.Owner) || int32(population) != int32(f.Population) || x != uint16(fx*256) || y != uint16(fy*256) || kind == 4 && stage != f.Stage || !stateMatch) {
				if _, ok := first["followers"]; !ok {
					first["followers"] = pass
					t.Logf("pass%d follower%d Go owner%d pop%d state%d xy%.4f,%.4f stage%d; reference owner%d pop%d kind%d state%d xy%.4f,%.4f stage%d", pass, id, f.Owner, f.Population, f.State, fx, fy, f.Stage, int(owner)-1, int32(population), kind, state, float64(x)/256, float64(y)/256, stage)
				}
				break
			}
		}
	}
	t.Logf("campaign world %d, landscape %d, seed 0x%x; compare %d no-input integrated main frames", sample.Input.World, rawLevel.Terrain, campaign[sample.Input.World].Seed, frames)
	compare(0)
	frame := g.newFrame(t, nil)
	frame.RenderChildren.Callbacks.SkipCopyProtection = true
	for tick := 0; tick < frames; tick++ {
		if err = h.Session.BeginRaw(h.World, c); err != nil {
			t.Fatal(err)
		}
		complete := false
		for poll := 0; poll < 32 && !complete; poll++ {
			input := &h.Session.Presentation.Input
			if _, err = h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(input.Mouse.CounterX), CounterY: uint8(input.Mouse.CounterY)}, h.Memory.BSS, &h.Session.Frame); err != nil {
				t.Fatal(err)
			}
			complete, err = frame.Advance()
			if err != nil {
				t.Fatal(tick, poll, err)
			}
		}
		if !complete {
			t.Fatal("bounded main frame remained pending")
		}
		c = h.Session.Frame
		independent.Step()
		compare(tick + 1)
	}

	h.Session.finish(nil)

	t.Logf("first normalized discrepancies: %v", first)
	if len(first) > 0 {
		t.Fail()
	}
}

func independentReferenceWord(h *NativeRuntimeHost, a int) uint16 {
	v, _ := h.Memory.BSS.Read16(a)
	return v
}
func independentReferenceLong(h *NativeRuntimeHost, a int) uint32 {
	v, _ := h.Memory.BSS.Read32(a)
	return v
}

func independentReferenceByte(h *NativeRuntimeHost, a int) uint8 {
	v, _ := h.Memory.BSS.Read8(a)
	return v
}

func independentReferenceCodeWord(h *NativeRuntimeHost, a int) uint16 {
	v, _ := h.Memory.Code.Read16(a)
	return v
}

func TestIndependentEngineMixedLavaReferenceOptional(t *testing.T) {
	if os.Getenv("POPULOUS2_REFERENCE_COMPARE") != "1" {
		t.Skip("optional local reference comparison")
	}
	f := lavaFixture{}
	f.Input.Mode = "tick"
	f.Input.Tile = 15
	f.Input.Owner = 1
	f.Input.X = 20
	f.Input.Y = 20
	f.Input.Timer = 2
	f.Input.Life = 100
	f.Input.Direction = 2
	f.Input.Fraction = 128
	f.Input.Seed = 4311
	v, _, _ := lavaFixtureMemory(t, f)
	m := v.m
	targets := []struct {
		at          int
		kind, owner uint8
	}{{0x76f4, 2, 1}, {0x6bd0, 22, 3}, {0x5f50, 26, 1}, {0xe74e, 20, 1}}
	for _, actor := range targets {
		ref := NativeRecordReference(actor.at - 0x76c0)
		_ = m.unlink(ref)
		m.putByte(actor.at, actor.kind)
		m.putByte(actor.at+12, actor.owner)
		m.putWord(actor.at+6, 20*256+128)
		m.putWord(actor.at+8, 20*256+128)
		if actor.kind == 2 {
			m.putByte(actor.at+22, 2)
			_ = m.write32(actor.at+26, 1000)
		}
		if err := m.insert(ref); err != nil {
			t.Fatal(err)
		}
	}
	callbacks := lavaCompleteCallbacks(t, v, 1)
	head := NativeRecordReference(m.word(0xf44 + (20+20*64)*4 + 2))
	for visits := 0; head != 0 && visits < 20; visits++ {
		if err := v.lava.push(head, 1, callbacks); err != nil {
			t.Fatal(err)
		}
		head = NativeRecordReference(m.word(cleanupRecordAddress(head) + 2))
	}
	land, err := engine.DecodeLandscape(testBundle(t).Raw["land0.dat"])
	if err != nil {
		t.Fatal(err)
	}
	w := &engine.World{Editor: true, Landscape: land}
	var heights [engine.CornerSize * engine.CornerSize]uint8
	for at := range heights {
		heights[at] = 1
	}
	if err = w.EditorSetTerrain(heights); err != nil {
		t.Fatal(err)
	}
	if err = w.EditorPlaceFollower(0, 20, 20, 1000); err != nil {
		t.Fatal(err)
	}
	if err = w.EditorPlaceScenery(engine.SceneryTree, 20, 20); err != nil {
		t.Fatal(err)
	}
	w.Earth.Walls[0] = engine.WallActor{Active: true, Owner: 0, X: 20, Y: 20}
	w.Actors.Link(engine.ActorRef{Kind: engine.ActorWall, Index: 0}, 20*256+128, 20*256+128)
	w.Magnets[0] = engine.MagnetActor{Owner: 0, X: 20*256 + 128, Y: 20*256 + 128}
	w.Actors.Link(engine.ActorRef{Kind: engine.ActorMagnet, Index: 0}, w.Magnets[0].X, w.Magnets[0].Y)
	if err = w.ApplyLavaParcel(20, 20, 1); err != nil {
		t.Fatal(err)
	}
	refs := []engine.ActorRef{{Kind: engine.ActorFollower, Index: 1}, {Kind: engine.ActorScenery, Index: 0}, {Kind: engine.ActorWall, Index: 0}, {Kind: engine.ActorMagnet, Index: 0}}
	for index, actor := range targets {
		x, y, linked := w.Actors.Position(refs[index])
		if !linked || x != int(m.word(actor.at+6)) || y != int(m.word(actor.at+8)) {
			t.Fatalf("actor kind%d Go%d,%d reference%d,%d", actor.kind, x, y, m.word(actor.at+6), m.word(actor.at+8))
		}
	}
	if w.Nature.Scenery[0].Kind != engine.SceneryBurningTree || m.byte(0x6bd0) != 30 {
		t.Fatal("mixed lava tree transition differs")
	}
	if w.Followers[1].Population != 1000 || int32(func() uint32 { v, _ := m.read32(0x76f4 + 26); return v }()) != 1000 || w.FireDamage.Deaths[1].Mode != engine.FireVictimBurning || m.byte(0x76f4+22) != 60 {
		t.Fatal("mixed lava follower retention differs")
	}
}
