package populous2

import (
	"encoding/json"
	"fmt"
	"go-populous2/internal/engine"
	"os"
	"testing"
)

// TestIndependentEngineCampaignReferenceOptional compares one complete source
// physics pass per independent Step. Rendering, input and audio presentation
// are outside this boundary; gameplay uses the same actual campaign record.
func TestIndependentEngineCampaignReferenceOptional(t *testing.T) {
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
	t.Logf("campaign world %d, landscape %d, seed 0x%x; compare 160 no-input physics passes", sample.Input.World, rawLevel.Terrain, campaign[sample.Input.World].Seed)
	compare(0)
	if err = h.Session.BeginRaw(h.World, c); err != nil {
		t.Fatal(err)
	}
	callbacks := NativeFrameSessionCallbacks{Audio: NativeFrameAudioCallbacks{Command: g.Audio.Command}, DirectSound: func(cue uint16) error { return g.Audio.DirectCue(cue, &h.Session.Frame) }, Bitmap: h.Bitmap}
	h.Session.directSound = callbacks.DirectSound
	h.Session.bitmapResolver = h.Bitmap
	for tick := 0; tick < 160; tick++ {
		_ = h.Memory.BSS.Write32(0xf40, uint32(tick+1))
		bitmap, err := h.Session.Presentation.BackBuffer()
		if err != nil {
			t.Fatal(err)
		}
		cb := h.Session.physicsCallbacks(callbacks, bitmap)
		cb.Swap = func(*NativeFrameRegisterContext) (bool, error) { return false, nil }
		h.Session.Pass = NativeFramePassState{}
		done, err := h.Session.Pass.TickFramePass(&h.Session.Frame, cb)
		if err != nil {
			t.Fatal(tick, err)
		}
		if done || h.Session.Pass.Stage != NativeFrameSwap {
			t.Fatal("reference did not stop after one complete physics pass")
		}
		independent.Step()
		compare(tick + 1)

		h.Session.Image.AudioBank = h.Session.Audio.Entries
		if err = h.ImageAudioCode.SetOwner(NativeImageCodeOwner); err != nil {
			t.Fatal(err)
		}
		if _, err = h.Session.Presentation.Swap(&h.Session.Frame); err != nil {
			t.Fatal(err)
		}

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
