package app

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestStormDrawsKeepStrikeImpactCloudOrderAndHeight(t *testing.T) {
	plan := stormDraws(engine.StormEffect{Active: true, Timer: 1, Frame: 3, ImpactActive: true, ImpactFrame: 2, WaterImpact: true}, 100, 120)
	if len(plan) != 3 || !plan[0].Strike || plan[0].Frame != 1 || plan[0].Y != 45 || plan[1].Animation != "fire-impact/water" || plan[1].Y != 128 || plan[2].Animation != "storm/cloud" || plan[2].Frame != 3 {
		t.Fatal("storm draw order or coordinates differ", plan)
	}
	if got := stormDraws(engine.StormEffect{Active: true}, 100, 50); len(got) != 1 || got[0].Y != 0 {
		t.Fatal("cloud was not clamped at display top")
	}
}

func TestLavaSelectsActualFlatDirectionsAndRampArt(t *testing.T) {
	for d, name := range [4]string{"north", "east", "south", "west"} {
		if lavaAnimation(15, d) != "lava/"+name {
			t.Fatal("flat lava direction differs")
		}
	}
	for shape, name := range map[uint8]string{3: "lava/slope-3", 6: "lava/slope-6", 9: "lava/slope-9", 12: "lava/slope-12"} {
		if lavaAnimation(shape, 0) != name {
			t.Fatal("ramp lava art differs")
		}
	}
	if lavaAnimation(7, 0) != "" || volcanoHasIndependentSprite() {
		t.Fatal("unsupported terrain or volcano received invented sprite art")
	}
}

func TestPrivateStormLayerCoordinatesAgainstOriginalRequests(t *testing.T) {
	path := os.Getenv("POPULOUS2_STORM_RENDER_TRACE")
	assetsPath := os.Getenv("POPULOUS2_ENDING_TEST_DIR")
	if path == "" || assetsPath == "" {
		t.Skip("set private storm render trace and portable asset directory")
	}
	assets, err := LoadAssets(os.DirFS(assetsPath))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name    string
				D       [8]uint32
				Initial []struct {
					Address, Width int
					Value          uint32
				}
			}
			Sprites []struct {
				Sprite int
				X, Y   int16
				Height uint16
			}
		}
	}
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, c := range catalog.Cases {
		cloud, impact, timer := 0, 0, 0
		for _, p := range c.Input.Initial {
			switch p.Address {
			case 0x76f4 + 10:
				cloud = int(p.Value)
			case 0x76f4 + 20:
				timer = int(p.Value)
			case 0x76f4 + 26:
				impact = int(p.Value)
			}
		}
		if timer < 1 || timer > 2 {
			continue
		}
		t.Run(c.Input.Name, func(t *testing.T) {
			_ = cloud
			e := engine.StormEffect{Active: true, Timer: timer}
			if impact != 0 {
				e.ImpactActive = true
				e.ImpactFrame = (impact - 0x49c) / 4
				if impact >= 0x5ec {
					e.WaterImpact = true
					e.ImpactFrame = (impact - 0x5ec) / 4
				}
			}
			x, y := int(int16(c.Input.D[0])), int(int16(c.Input.D[1]))
			var got []struct {
				Sprite int
				X, Y   int16
				Height uint16
			}
			for _, d := range stormDraws(e, x, y) {
				if !d.Strike {
					continue
				}
				animation := assets.Visual.Animations[d.Animation]
				for _, layer := range animation.Frames[d.Frame].Layers {
					sprite := assets.Visual.Sprites[0][layer.Sprite]
					left, top, height, visible := strikeLayerGeometry(layer, sprite, d.X, d.Y, y)
					if !d.Strike {
						left = d.X + layer.X - sprite.AnchorX
						top = d.Y + layer.Y - sprite.AnchorY
						height = sprite.Image.Bounds().Dy()
						visible = true
					}
					if visible {
						got = append(got, struct {
							Sprite int
							X, Y   int16
							Height uint16
						}{layer.Sprite, int16(left), int16(top), uint16(height)})
					}
				}
			}
			var expectedStrikes []struct {
				Sprite int
				X, Y   int16
				Height uint16
			}
			for _, sprite := range c.Sprites {
				if sprite.Sprite == 396 || sprite.Sprite == 397 {
					expectedStrikes = append(expectedStrikes, sprite)
				}
			}
			actual, _ := json.Marshal(got)
			expected, _ := json.Marshal(expectedStrikes)
			if !bytes.Equal(actual, expected) {
				t.Fatal("storm sprite requests differ", string(actual), string(expected))
			}
		})
		checked++
	}
	if checked == 0 {
		t.Fatal("private storm render corpus supplied no valid cloud cases")
	}
}

func TestPrivateShortenedStrikePixelsAgainstPreparedArt(t *testing.T) {
	originalPath, portablePath := os.Getenv("POPULOUS2_EXPORT_TEST_DIR"), os.Getenv("POPULOUS2_ENDING_TEST_DIR")
	if originalPath == "" || portablePath == "" {
		t.Skip("set private original and portable asset directories")
	}
	source, err := populous2.LoadFS(os.DirFS(originalPath))
	if err != nil {
		t.Fatal(err)
	}
	portable, err := visualassets.LoadFS(os.DirFS(portablePath))
	if err != nil {
		t.Fatal(err)
	}
	for land := range 4 {
		bank, err := populous2.DecodeNativeSpriteBitmapBank(source, land)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []int{396, 397} {
			sprite := bank.Sprites[id]
			for height := 1; height <= sprite.Height; height++ {
				want, err := populous2.DecodeNativeMaskedPlanes(sprite.Planes[:sprite.Width/8*5*height], sprite.Width, height, source.Landscapes[land].Palettes[0])
				if err != nil {
					t.Fatal(err)
				}
				got := portable.StormStrikeImage(land, id, height)
				if got == nil || got.Bounds() != image.Rect(0, 0, sprite.Width, height) || !bytes.Equal(got.Pix, want.Pix) {
					t.Fatal("shortened storm artwork pixels differ", land, id, height)
				}
			}
		}
	}
}
