package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeWorldAndOpponentScreensAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/world_screens_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Worlds []struct {
			State             NativeWorldRequesterState
			Background        int
			TextHex, RGBAHash string
		}
		Opponents []struct {
			World, Reaction, Aggression uint16
			Background                  int
			TextHex, RGBAHash           string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Worlds) != 144 || len(catalog.Opponents) != 512 {
		t.Fatal("native world/opponent screen corpus incomplete")
	}
	b := testBundle(t)
	r, err := DecodeNativeInGameRequesterRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	background := func(pattern int) []byte {
		pixels := make([]byte, 320*200)
		if pattern != 0 {
			for y := range 200 {
				for x := range 320 {
					pixels[y*320+x] = byte((x*3 + y*5 + 7) & 15)
				}
			}
		}
		return pixels
	}
	check := func(name, textHex, hash string, frame NativeWorldScreen, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(name, err)
		}
		text, err := hex.DecodeString(textHex)
		if err != nil {
			t.Fatal(err)
		}
		if string(frame.Requester.Text) != string(text) {
			t.Fatalf("%s text differs", name)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(frame.Image.Pix)); got != hash {
			t.Fatalf("%s framebuffer differs: %s / %s", name, got, hash)
		}
	}
	for _, fixture := range catalog.Worlds {
		frame, err := r.WorldScreen(b, fixture.State, background(fixture.Background), nil)
		check(fmt.Sprintf("world%d/land%d/background%d", fixture.State.World, fixture.State.Landscape, fixture.Background), fixture.TextHex, fixture.RGBAHash, frame, err)
	}
	for _, fixture := range catalog.Opponents {
		frame, err := r.OpponentScreen(b, fixture.World, fixture.Reaction, fixture.Aggression, background(fixture.Background))
		check(fmt.Sprintf("opponent%d/reaction%x/aggression%x/background%d", fixture.World, fixture.Reaction, fixture.Aggression, fixture.Background), fixture.TextHex, fixture.RGBAHash, frame, err)
	}
}
