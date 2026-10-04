package populous2

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestRequesterAgainstOriginal68000CompilationAndClicks(t *testing.T) {
	data, err := os.ReadFile("testdata/menu_requester_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			Name                       string
			Offset                     int
			Parameters                 []string
			EnableOptions              bool   `json:"enable_options"`
			VisibleStartup             bool   `json:"visible_startup"`
			DefinitionHex              string `json:"template_hex"`
			PreparedHex                string `json:"text_hex"`
			Column, Row, Width, Height int
			Clicks                     []struct {
				X, Y, Command int
				TextHex       string `json:"text_hex"`
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Fixtures) != 16 {
		t.Fatalf("invalid native requester fixture catalog: %v", err)
	}
	bundle := testBundle(t)
	baseRules, err := DecodeNativeRequesterRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			definition, err := hex.DecodeString(fixture.DefinitionHex)
			if fixture.Offset != 0 {
				definition, err = NativeRequesterTemplate(bundle.Executable, fixture.Offset)
			}
			if err != nil {
				t.Fatal(err)
			}
			parameters := make([][]byte, len(fixture.Parameters))
			for index, value := range fixture.Parameters {
				parameters[index] = []byte(value)
			}
			rules := baseRules
			rules.EnableOptionMarkers(fixture.EnableOptions)
			requester, err := rules.Compile(definition, parameters)
			if fixture.VisibleStartup {
				requester, err = rules.Startup(bundle.Executable)
			}
			if err != nil {
				t.Fatal(err)
			}
			text, err := hex.DecodeString(fixture.PreparedHex)
			if err != nil || !bytes.Equal(requester.Text, text) || requester.Column != fixture.Column || requester.Row != fixture.Row || requester.Width != fixture.Width || requester.Height != fixture.Height {
				t.Fatalf("prepared requester differs from original: %+v", requester)
			}
			for _, click := range fixture.Clicks {
				if got := rules.Click(requester, click.X, click.Y); got != click.Command {
					t.Fatalf("click %d,%d: action %d, native %d", click.X, click.Y, got, click.Command)
				}
				text, err := hex.DecodeString(click.TextHex)
				if err != nil || !bytes.Equal(requester.Text, text) {
					t.Fatal("native radio glyph transition differs")
				}
			}
		})
	}
}
