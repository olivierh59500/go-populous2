package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativePresentationCatalog struct {
	Startup struct{ Hash, TextHex string }
	Results []struct {
		Input struct {
			Name                        string
			Selected, Eliminated, Score uint16
			Ticks                       uint32
			Local, Opponent             CampaignResultStatistics
		}
		Parameters    []string
		Metrics       [2]uint16
		Hash, TextHex string
	}
	Ending []struct {
		InitialPhase, Phase, Offset uint16
		Step, Cursor, LoopOffset    int
		Code                        uint32
		Hash                        string
	}
}

func readNativePresentationCatalog(t *testing.T) nativePresentationCatalog {
	t.Helper()
	data, err := os.ReadFile("testdata/presentation_screens_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var c nativePresentationCatalog
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Results) != 124 || len(c.Ending) != 1220 {
		t.Fatal("native screen presentation fixture catalog is incomplete")
	}
	return c
}

func TestNativeStartupAndResultScreensAgainstOriginalCPU(t *testing.T) {
	catalog := readNativePresentationCatalog(t)
	bundle := testBundle(t)
	p, err := DecodeNativePresentation(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	startup, err := p.StartupImage()
	if err != nil {
		t.Fatal(err)
	}
	var planes [32000]byte
	if err := nativePackScreen(startup.Pix, &planes); err != nil {
		t.Fatal(err)
	}
	text, err := hex.DecodeString(catalog.Startup.TextHex)
	if err != nil || !bytes.Equal(text, p.StartupRequester.Text) || fmt.Sprintf("%x", sha256.Sum256(planes[:])) != catalog.Startup.Hash {
		t.Fatal("startup bitmap/font composition differs from original CPU")
	}
	for index := range planes {
		planes[index] = byte(index*7 + 13)
	}
	background, err := nativeScreenIndices(planes[:])
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Results {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			r, err := p.Result(fixture.Input.Selected, fixture.Input.Eliminated, fixture.Input.Ticks, fixture.Input.Local, fixture.Input.Opponent, fixture.Input.Score)
			if err != nil {
				t.Fatal(err)
			}
			if r.DisplayedMetric != fixture.Metrics {
				t.Fatalf("native metric clamping differs: got%v want%v", r.DisplayedMetric, fixture.Metrics)
			}
			for index, value := range r.Parameters {
				if string(value) != fixture.Parameters[index] {
					t.Fatalf("native result field%d differs: got%q want%q", index, value, fixture.Parameters[index])
				}
			}
			text, err := hex.DecodeString(fixture.TextHex)
			if err != nil || !bytes.Equal(text, r.Requester.Text) {
				t.Fatal("native result text/field-width substitution differs")
			}
			img, err := p.Compose(background, r.Requester, bundle.Landscapes[0].Palettes[0])
			if err != nil {
				t.Fatal(err)
			}
			if err := nativePackScreen(img.Pix, &planes); err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(planes[:])) != fixture.Hash {
				t.Fatal("retained background and result font planes differ from original CPU")
			}
		})
	}
}

func TestNativeEndingFullLoopAgainstOriginalCPU(t *testing.T) {
	catalog := readNativePresentationCatalog(t)
	bundle := testBundle(t)
	p, err := DecodeNativePresentation(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	var ending *NativeEnding
	for _, fixture := range catalog.Ending {
		if fixture.Step == 0 {
			ending, err = NewNativeEnding(bundle.Raw["end.pak"], p, fixture.InitialPhase)
		} else {
			err = ending.Advance()
		}
		if err != nil {
			t.Fatalf("ending phase%d update%d: %v", fixture.InitialPhase, fixture.Step, err)
		}
		a := ending.Animation
		if ending.ScrollPhase != fixture.Phase || ending.TextOffset != fixture.Offset || a.Code != fixture.Code || a.Cursor != fixture.Cursor || a.LoopOffset != fixture.LoopOffset || fmt.Sprintf("%x", sha256.Sum256(a.Planes[:])) != fixture.Hash {
			t.Fatalf("ending phase%d update%d differs: phase%x/%x text%d/%d control%x/%x cursor%d/%d bookmark%d/%d planes%x/%s", fixture.InitialPhase, fixture.Step, ending.ScrollPhase, fixture.Phase, ending.TextOffset, fixture.Offset, a.Code, fixture.Code, a.Cursor, fixture.Cursor, a.LoopOffset, fixture.LoopOffset, sha256.Sum256(a.Planes[:]), fixture.Hash)
		}
	}
}
