package populous2

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestWorldCampaignScoreAndProgressionAgainstNativeReferences(t *testing.T) {
	data, err := os.ReadFile("testdata/campaign_result_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []campaignResultFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := campaignResultFixtureMemory(f)
			w := installNativeFixtureWorld(t, m[:], nil)
			w.NativeProfileSide, w.NativeGameMode, w.NativeClock = uint8(f.Input.Selected), f.Input.GameMode, f.Input.Ticks
			w.Level.Number = int(f.Input.World)
			w.Deity.Bolts = f.Input.Local.Bolts
			if f.Input.Mode == "score" {
				sides, err := w.nativeCampaignStatistics()
				if err != nil {
					t.Fatal(err)
				}
				score, err := w.CampaignResult.Score(w.NativeClock, sides[w.NativeProfileSide], sides[3-w.NativeProfileSide])
				if f.Trap {
					if err == nil {
						t.Fatal("native divide-zero fault lost")
					}
					return
				}
				if err != nil || score.Value != f.Score || score.RatioWord != f.Ratio || score.DivideOverflow != f.Overflow {
					t.Fatal("World native score/statistics differ")
				}
			} else {
				w.NativeResult = NativeGameResult{Detected: true, Eliminated: f.Input.Eliminated, Score: CampaignScore{Value: f.Input.Score}}
				p, err := w.AdvanceCampaign()
				if err != nil {
					t.Fatal(err)
				}
				stop := "reload"
				if p.OpenDeity {
					stop = "deity"
				}
				if p.Complete {
					stop = "complete"
				}
				if p.NextWorld != f.World || w.Deity.Bolts != f.Bolts || stop != f.Stop {
					t.Fatal("World native result progression/bolts differ")
				}
				before := w.Deity.Bolts
				again, err := w.AdvanceCampaign()
				if err != nil || again != p || w.Deity.Bolts != before {
					t.Fatal("result applied more than once")
				}
			}
		})
	}
	if len(catalog.Cases) != 1096 {
		t.Fatal("World campaign native coverage incomplete")
	}
}

func TestNativeCampaignCommandWeightsAndClock(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	w.Core.Magnets[0].Mana = 1000000
	deity, _ := NativeDeityAddress(1)
	weight := func() uint16 { v, _ := w.runtimeMemory().Read16(deity + 0x138); return v }
	if !w.Cast(0, Heracles, Target{}) {
		t.Fatal("hero rejected")
	}
	if weight() != uint16(Heracles)%6+1 {
		t.Fatal("native shared debit weight missing")
	}
	before := weight()
	pos := w.Core.Peeps[0].AtPos
	w.Sculpt(0, pos%64, pos/64, true)
	if weight() != before {
		t.Fatal("propagated sculpt incorrectly counted as a weighted command")
	}
	w.PlaceLightning(0, 32, 32)
	if weight() != before {
		t.Fatal("lightning marker counted")
	}
	if !w.ActivateLightning(0) || weight() != before+uint16(Lightning)%6+1 {
		t.Fatal("lightning activation weight missing")
	}
	before = weight()
	w.DismissLightning(0)
	if weight() != before {
		t.Fatal("lightning cancel counted")
	}
	for _, mode := range []uint16{2, 4, 6, 8, 10} {
		w.NativeGameMode = mode
		before = weight()
		w.recordNativePowerUse(0, Storm)
		want := before
		if mode != 8 {
			want += uint16(Storm)%6 + 1
		}
		if weight() != want {
			t.Fatal("editor debit gate differs")
		}
	}
	w.NativeFreeCommands = 1
	before = weight()
	w.recordNativePowerUse(0, Storm)
	if weight() != before {
		t.Fatal("free command gate differs")
	}
	w.NativeFreeCommands = 0
	w.NativeGameMode = 4
	w.NativeClock = 0x1234fffe
	for range 3 {
		w.Tick()
	}
	if w.NativeClock != 0x12350001 {
		t.Fatal("native long frame counter did not cross low-word boundary")
	}
	clock, err := w.nativeCleanupMemory().Read32(0xf40)
	low, err2 := w.nativeCleanupMemory().Read16(0xf42)
	if err != nil || err2 != nil || clock != w.NativeClock || low != uint16(w.NativeClock) {
		t.Fatal("native long/lowword clock aliases differ")
	}
	for owner := uint8(1); owner <= 2; owner++ {
		d, _ := NativeDeityAddress(owner)
		identity, _ := w.runtimeMemory().Read16(d + 0x18)
		options, _ := w.runtimeMemory().Read16(d + 0x4a)
		if identity != uint16(owner) || options != w.Rules[owner-1].Raw {
			t.Fatal("native identity/options bridge differs")
		}
	}
}

func TestNativeResultLatchesBeforeEffectsAndSurvivesSave(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 40, false)
	if err != nil {
		t.Fatal(err)
	}
	w.Core.Computer[1].Mode = 0
	for index, p := range w.Core.Peeps {
		if p.Player == 1 {
			w.Core.Peeps[index].Population = 0
			w.NativeEntries[index] = NativeFollowerEntry{}
		}
	}
	w.tickNativeEffects()
	w.Tick()
	if !w.NativeResult.Detected || w.ResultForLocalProfile() != legacy.ResultWon {
		t.Fatal("native eliminated identity did not latch")
	}
	before := encodeSnapshot(t, w)
	clock := w.NativeClock
	w.Tick()
	if w.NativeClock != clock || !bytes.Equal(before, encodeSnapshot(t, w)) {
		t.Fatal("result continued simulation")
	}
	copy, err := ReadSave(b, bytes.NewReader(before))
	if err != nil {
		t.Fatal(err)
	}
	p, err := w.AdvanceCampaign()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := copy.AdvanceCampaign()
	if err != nil || p != p2 || w.Deity.Bolts != copy.Deity.Bolts {
		t.Fatal("saved pending result diverged")
	}
	applied := encodeSnapshot(t, w)
	copy, err = ReadSave(b, bytes.NewReader(applied))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := copy.AdvanceCampaign(); err != nil || !bytes.Equal(applied, encodeSnapshot(t, copy)) {
		t.Fatal("loaded applied result awarded again")
	}
}

func TestNativeResultSideOrderAndEditorSuppression(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	m := w.runtimeMemory()
	for owner := uint8(1); owner <= 2; owner++ {
		d, _ := NativeDeityAddress(owner)
		_, _ = m.Write32(d+4, 0)
	}
	w.NativeGameMode = 8
	w.detectNativeResult()
	if w.NativeResult.Detected {
		t.Fatal("editor result detected")
	}
	w.NativeGameMode = 2
	w.detectNativeResult()
	if w.NativeResult.Eliminated != 1 || w.ResultForLocalProfile() != legacy.ResultLost {
		t.Fatal("native both-empty first-side ordering differs")
	}
}
