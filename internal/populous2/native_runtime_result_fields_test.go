package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNativeRuntimeResultFieldsAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_runtime_result_fields_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Input struct {
				Name, Mode                                   string
				Selected, Eliminated, GameMode, World, Score uint16
				Ticks                                        uint32
				Local, Opponent                              CampaignResultStatistics
				D                                            [8]uint32
			}
			D                                             [8]uint32
			BSSHash, CodeHash, FrontHash, BackHash, Error string
			Score                                         uint16
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 96 {
		t.Fatal("original raw result fields corpus incomplete", err)
	}
	for _, f := range corpus.Cases {
		if f.Error != "" {
			t.Fatal(f.Input.Name, f.Error)
		}
		h := nativeRuntimeHostTest(t)
		frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: h.Memory.BSSBase}
		fixture := campaignResultFixture{}
		fixture.Input.Selected = f.Input.Selected
		fixture.Input.Eliminated = f.Input.Eliminated
		fixture.Input.GameMode = f.Input.GameMode
		fixture.Input.World = f.Input.World
		fixture.Input.Ticks = f.Input.Ticks
		fixture.Input.Local = f.Input.Local
		fixture.Input.Opponent = f.Input.Opponent
		raw := campaignResultFixtureMemory(fixture)
		for i, v := range raw {
			if err := h.Memory.BSS.Write8(i, v); err != nil {
				t.Fatal(err)
			}
		}
		if err := h.Memory.BSS.Write32(0x1a, 0xa00000); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.BSS.Write32(0x1e, 0xa10000); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.Code.Write16(0x3b62, f.Input.Eliminated); err != nil {
			t.Fatal(err)
		}
		state := NativeRuntimeResultState{}
		if err := state.prepareRequester(h, &frame); err != nil {
			t.Fatal(f.Input.Name, err)
		}
		if frame.D != f.D || state.Score != f.Score {
			t.Fatalf("actual result registers/score differ for%s: got%x native%x score%d/%d", f.Input.Name, frame.D, f.D, state.Score, f.Score)
		}
		bss, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		if fileFrameHash(bss) != f.BSSHash {
			t.Fatal("actual result metric/stat mutations differ", f.Input.Name)
		}
		code := h.Code.RawData()[:len(h.Bundle.Executable.Hunks[0].Data)]
		if fileFrameHash(code) != f.CodeHash {
			t.Fatal("actual result numeric fields/requester CODE differs", f.Input.Name)
		}
	}
}
