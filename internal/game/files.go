package game

import (
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous2/internal/populous2"
)

func (g *Game) openFileRequester(mode populous2.NativeFileRequesterMode) {
	drawer, filename := filepath.Split(g.SavePath)
	if drawer == "" {
		drawer = "."
	}
	cb := populous2.NativeFileRequesterCallbacks{
		List: populous2.ListNativeHostFiles, Exists: populous2.NativeHostFileExists,
		Load: func(raw []byte) error {
			path, err := populous2.ResolveNativeHostPath(raw, false)
			if err != nil {
				return err
			}
			return g.LoadFile(path)
		},
		Save: func(raw []byte) error {
			path, err := populous2.ResolveNativeHostPath(raw, true)
			if err != nil {
				return err
			}
			g.World.SetEffectView(g.CameraX, g.CameraY)
			if err := g.World.WriteGameFile(path); err != nil {
				return err
			}
			g.SavePath = path
			return nil
		},
	}
	r, err := populous2.NewNativeFileRequester(g.nativePresentation, mode, []byte(drawer), []byte(filename), cb)
	if err != nil {
		g.notify(err.Error())
		return
	}
	g.fileRequester = r
	g.fileEdit = nil
	g.fileBackground = nil
	if err := g.refreshFileRequester(); err != nil {
		g.failure = err
	}
}

func (g *Game) refreshFileRequester() error {
	r := g.fileRequester
	display := *r
	if r.Editing == populous2.NativeFileEditDrawer {
		display.Drawer = append(append([]byte(nil), g.fileEdit...), '_')
	} else if r.Editing == populous2.NativeFileEditName {
		display.Filename = append(append([]byte(nil), g.fileEdit...), '_')
	}
	plan, err := display.Plan()
	if err != nil {
		return err
	}
	img, err := g.nativePresentation.Overlay(plan, g.World.Landscape.Palettes[0])
	if err != nil {
		return err
	}
	if g.fileImage == nil {
		g.fileImage = ebiten.NewImageFromImage(img)
	} else {
		g.fileImage.WritePixels(img.Pix)
	}
	return nil
}

func (g *Game) updateFileRequester() error {
	r := g.fileRequester
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if r.Editing != populous2.NativeFileEditNone {
			r.Editing = populous2.NativeFileEditNone
		} else {
			r.Cancelled = true
		}
	} else if r.Editing != populous2.NativeFileEditNone {
		for _, ch := range ebiten.AppendInputChars(nil) {
			if ch >= ' ' && ch <= '~' && len(g.fileEdit) < 38 {
				g.fileEdit = append(g.fileEdit, byte(ch))
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.fileEdit) > 0 {
			g.fileEdit = g.fileEdit[:len(g.fileEdit)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if err := r.FinishEdit(g.fileEdit); err != nil {
				return err
			}
		}
	} else if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if y >= 40 {
			if err := r.Click(x/2, (y-40)/2); err != nil {
				return err
			}
			if r.Editing == populous2.NativeFileEditDrawer {
				g.fileEdit = append([]byte(nil), r.Drawer...)
			} else if r.Editing == populous2.NativeFileEditName {
				g.fileEdit = append([]byte(nil), r.Filename...)
			}
		}
	}
	if r.Done || r.Cancelled {
		g.fileRequester, g.fileImage, g.fileBackground, g.fileEdit = nil, nil, nil, nil
		return nil
	}
	return g.refreshFileRequester()
}

func (g *Game) drawFileRequester(screen *ebiten.Image) {
	if g.fileBackground == nil {
		g.fileBackground = ebiten.NewImage(Width, Height)
		g.fileBackground.DrawImage(screen, nil)
	}
	screen.DrawImage(g.fileBackground, nil)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2, 2)
	op.GeoM.Translate(0, 40)
	screen.DrawImage(g.fileImage, op)
}
