package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeMenuWidgetsAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/presentation_widgets_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Options []struct {
			Side, Rules, Speed uint16
			TextHex, Hash      string
		}
		Deity []struct {
			Parts      [3]uint8
			Experience [6]uint8
			Layers     []NativeFaceLayer
			Hash       string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Options) != 2048 || len(catalog.Deity) != 512 {
		t.Fatal("native menu widget fixture catalog is incomplete")
	}
	bundle := testBundle(t)
	p, err := DecodeNativePresentation(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	var planes [32000]byte
	for index, fixture := range catalog.Options {
		r, err := p.Options(fixture.Side, fixture.Rules, fixture.Speed, nil)
		if err != nil {
			t.Fatal(err)
		}
		text, err := hex.DecodeString(fixture.TextHex)
		if err != nil || !bytes.Equal(r.Requester.Text, text) {
			t.Fatalf("native options%d prepared text differs: got%q want%q", index, r.Requester.Text, text)
		}
		img, err := p.Compose(make([]byte, 320*200), r.Requester, bundle.Landscapes[0].Palettes[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := nativePackScreen(img.Pix, &planes); err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(planes[:])) != fixture.Hash {
			t.Fatalf("native options%d glyph planes differ", index)
		}
	}
	for index, fixture := range catalog.Deity {
		pixels := make([]byte, 320*200)
		if err := p.Widgets.PaintDeity(pixels, fixture.Experience); err != nil {
			t.Fatal(err)
		}
		if err := nativePackScreen(pixels, &planes); err != nil {
			t.Fatal(err)
		}
		layers, err := p.Widgets.FaceLayers(fixture.Parts)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(layers, fixture.Layers) || fmt.Sprintf("%x", sha256.Sum256(planes[:])) != fixture.Hash {
			t.Fatalf("native deity widget%d differs: layers%+v/%+v hash%x/%s", index, layers, fixture.Layers, sha256.Sum256(planes[:]), fixture.Hash)
		}
	}
}
