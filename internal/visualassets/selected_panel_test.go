package visualassets

import (
	"encoding/json"
	"image"
	"reflect"
	"testing"
	"testing/fstest"
)

func testSelectionPanel() *SelectionPanel {
	p := &SelectionPanel{Version: 1, ActorX: 282, ActorY: 42, WeaponX: 254, WeaponY: 47, HitX: 245, HitY: 0, HitWidth: 70, HitHeight: 46, PopulationSprite: 177, Weapons: make([]Frame, 19), PopulationPoints: [8]image.Point{image.Pt(302, 24), image.Pt(296, 19), image.Pt(289, 16), image.Pt(281, 14), image.Pt(268, 14), image.Pt(259, 16), image.Pt(252, 19), image.Pt(246, 24)}}
	for i := range p.Weapons {
		p.Weapons[i] = Frame{Layers: []SpriteLayer{{Sprite: 11}}}
	}
	return p
}
func TestSelectionPanelPopulationUsesOriginalNibbleCoordinates(t *testing.T) {
	p := testSelectionPanel()
	if p.HitRect() != image.Rect(245, 0, 315, 46) {
		t.Fatal("selected panel hit bounds changed")
	}
	if !reflect.DeepEqual(p.PopulationIndicators(0x123), []image.Point{image.Pt(302, 21), image.Pt(296, 17), image.Pt(289, 15)}) {
		t.Fatal("population gauge did not use low nibble first with vertical subtraction")
	}
	if p.PopulationIndicators(0) != nil || len(p.PopulationIndicators(0xffffffff)) != 8 {
		t.Fatal("population indicator count differs from source")
	}
}
func TestSelectionPanelLoaderRejectsInvalidMetadataAndKeepsOptionalFile(t *testing.T) {
	if panel, err := LoadSelectionPanel(fstest.MapFS{}); err != nil || panel != nil {
		t.Fatal("missing optional panel prevented loading")
	}
	p := testSelectionPanel()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{SelectionPanelName: {Data: data}}
	loaded, err := LoadSelectionPanel(files)
	if err != nil || !reflect.DeepEqual(p, loaded) {
		t.Fatal("selection panel metadata changed during loading")
	}
	p.HitWidth = 999
	data, _ = json.Marshal(p)
	files[SelectionPanelName].Data = data
	if _, err := LoadSelectionPanel(files); err == nil {
		t.Fatal("out-of-screen hit rectangle accepted")
	}
}
