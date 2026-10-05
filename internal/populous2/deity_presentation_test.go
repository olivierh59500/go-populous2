package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestFullNativeDeityScreenAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/deity_screen_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Name, Password, TextHex, Hash, RGBAHash string
		Parts                                   [3]uint8
		Experience                              [6]uint8
		Bolts                                   uint16
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 512 {
		t.Fatal("original full deity corpus incomplete")
	}
	bundle := testBundle(t)
	p, err := DecodeNativePresentation(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		d := Deity{Name: r.Name, FaceParts: r.Parts, Experience: r.Experience, Bolts: r.Bolts}
		frame, err := p.DeityScreen(bundle, d, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		text, err := hex.DecodeString(r.TextHex)
		if err != nil {
			t.Fatal(err)
		}
		if frame.Password != r.Password || string(frame.Requester.Text) != string(text) {
			t.Fatalf("native full deity text/password differs for%s", r.Name)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(frame.Image.Pix)); got != r.RGBAHash {
			t.Fatalf("native full deity pixels/palette differ for%s: %s want%s", r.Name, got, r.RGBAHash)
		}
	}
}
