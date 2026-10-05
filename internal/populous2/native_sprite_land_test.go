package populous2

import "testing"

func TestNativePreparedSpritesMatchAllLoadedLandscapeBanks(t *testing.T) {
	b := testBundle(t)
	for land := 0; land < 4; land++ {
		bank, err := DecodeNativeSpriteBitmapBank(b, land)
		if err != nil {
			t.Fatal("landscape sprite resource", land, err)
		}
		for index, s := range b.Sprites[land] {
			if s.Image == nil {
				continue
			}
			prepared := bank.Sprites[index]
			img, err := DecodeNativeMaskedPlanes(prepared.Planes, prepared.Width, prepared.Height, b.Landscapes[land].Palettes[0])
			if err != nil {
				t.Fatal(err)
			}
			// Both mask coverage and all four native color planes must agree
			// with the independently decoded raw DIF/PIF sprite atlas.
			for y := 0; y < prepared.Height; y++ {
				for x := 0; x < prepared.Width; x++ {
					if got, want := img.RGBAAt(x, y), s.Image.RGBAAt(x, y); got != want {
						t.Fatalf("LAND%d sprite%d pixel%d/%d differs: %v/%v", land, index, x, y, got, want)
					}
				}
			}
		}
	}
}
