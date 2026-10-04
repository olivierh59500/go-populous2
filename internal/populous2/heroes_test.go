package populous2

import "testing"

func TestNativeHeroArtAndAttributes(t *testing.T) {
	b := testBundle(t)
	for _, id := range heroIDs {
		for direction := range 8 {
			layers := b.HeroRules.Layers(id, direction, 0)
			if len(layers) == 0 {
				t.Fatalf("hero %d direction %d missing", id, direction)
			}
			for _, layer := range layers {
				if b.Sprites[0][layer.Sprite].Image == nil {
					t.Fatalf("missing hero sprite %d", layer.Sprite)
				}
			}
		}
	}
	if got := b.HeroRules.Layers(Perseus, 0, 0); len(got) != 1 || got[0].Sprite != 181 {
		t.Fatalf("native Perseus composite image changed: %+v", got)
	}
	if len(b.HeroRules.Layers(Adonis, 0, 0)) < 2 {
		t.Fatal("Adonis composite layers lost")
	}
	if _, err := DecodeHeroRules(nil); err == nil {
		t.Fatal("missing native table accepted")
	}
	for _, tc := range []struct {
		id         SpellID
		exp        [6]uint8
		population int
		speed      uint8
	}{
		{Perseus, [6]uint8{64}, 100, 28},
		{Heracles, [6]uint8{0, 0, 64}, 200, 28},
		{Odysseus, [6]uint8{0, 0, 0, 64}, 100, 48},
		{Helen, [6]uint8{0, 0, 0, 0, 0, 255}, 100, 51},
	} {
		population, speed, ok := HeroAttributes(tc.id, 100, 20, tc.exp)
		if !ok || population != tc.population || speed != tc.speed {
			t.Fatalf("hero %d got %d/%d", tc.id, population, speed)
		}
	}
	if _, speed, _ := HeroAttributes(Odysseus, 100, 200, [6]uint8{}); speed != 255 {
		t.Fatal("movement speed overflow")
	}
}

func TestHeraclesConversionDoublesPopulationWithoutLegacyAwards(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	index := w.Core.Magnets[0].Carried - 1
	p := w.Core.Peeps[index]
	w.Core.Magnets[0].Mana = 1000000
	scores := w.Core.Scores
	if !w.Cast(0, Heracles, Target{}) {
		t.Fatal("funded hero conversion rejected")
	}
	if w.Core.Peeps[index].Population != p.Population*2 || w.Core.Peeps[index].Weapons != p.Weapons {
		t.Fatal("legacy weapons rule leaked into Heracles conversion")
	}
	if w.Core.Scores != scores {
		t.Fatal("Populous I knight score applied")
	}
	if w.Heroes[index].Speed != w.Level.Players[0].MovementSpeed() {
		t.Fatal("original walker speed lost")
	}
}
