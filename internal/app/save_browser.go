package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// SaveBrowser keeps selection and overwrite confirmation separate from live
// game state. A failed load never replaces the current world.
type SaveBrowser struct {
	Directory, Name, Error string
	Files                  []string
	Saving, Confirm        bool
	Offset                 int
	Return                 Screen
}

func (g *Game) openSaveBrowser(saving bool) error {
	if g.Network != nil {
		return fmt.Errorf("leave the two-player game before saving or loading")
	}
	if saving && g.World == nil {
		return fmt.Errorf("no game to save")
	}
	path := g.SavePath
	if path == "" {
		path = "go-populous2.json"
	}
	directory, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	browser := &SaveBrowser{Directory: directory, Name: filepath.Base(path), Saving: saving, Return: g.Screen}
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if entry.Type().IsRegular() && (ext == ".json" || ext == ".gam") {
			browser.Files = append(browser.Files, entry.Name())
		}
	}
	sort.Strings(browser.Files)
	g.SaveBrowser, g.Screen = browser, SaveBrowserScreen
	return nil
}

func (g *Game) closeSaveBrowser() {
	if g.SaveBrowser != nil {
		g.Screen = g.SaveBrowser.Return
		g.SaveBrowser = nil
	}
}

func (g *Game) acceptSaveBrowser() {
	b := g.SaveBrowser
	if b == nil {
		return
	}
	if b.Name == "" || b.Name == "." || filepath.Base(b.Name) != b.Name || strings.ContainsAny(b.Name, `/\`) {
		b.Error = "Choose a filename without folders"
		return
	}
	if filepath.Ext(b.Name) == "" {
		b.Name += ".json"
	}
	path := filepath.Join(b.Directory, b.Name)
	if b.Saving && !b.Confirm {
		if _, err := os.Lstat(path); err == nil {
			b.Confirm, b.Error = true, "Replace the existing save?"
			return
		} else if !os.IsNotExist(err) {
			b.Error = err.Error()
			return
		}
	}
	previous := g.SavePath
	g.SavePath = path
	var err error
	if b.Saving {
		err = g.saveGame()
	} else {
		err = g.loadGame()
	}
	if err != nil {
		g.SavePath = previous
		b.Error, b.Confirm = err.Error(), false
		return
	}
	g.SaveBrowser = nil
	if b.Saving {
		g.Screen = b.Return
	}
}

func (g *Game) updateSaveBrowser(x, y int, clicked bool) {
	b := g.SaveBrowser
	if b == nil {
		g.Screen = MainMenu
		return
	}
	if clicked && y >= 169 {
		if x < 150 {
			g.closeSaveBrowser()
		} else {
			g.acceptSaveBrowser()
		}
		return
	}
	if clicked && y >= 41 && y < 113 {
		index := b.Offset + (y-41)/12
		if index < len(b.Files) {
			b.Name, b.Confirm, b.Error = b.Files[index], false, ""
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		b.Offset = min(max(0, len(b.Files)-6), b.Offset+1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		b.Offset = max(0, b.Offset-1)
	}
	for _, char := range ebiten.AppendInputChars(nil) {
		if char >= 32 && char <= 126 && !strings.ContainsRune(`/\:*?"<>|`, char) && len(b.Name) < 96 {
			b.Name += string(char)
			b.Confirm, b.Error = false, ""
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(b.Name) > 0 {
		b.Name = b.Name[:len(b.Name)-1]
		b.Confirm = false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) || (ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyA)) {
		b.Name, b.Confirm, b.Error = "", false, ""
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.acceptSaveBrowser()
	}
}

func (g *Game) drawSaveBrowser() {
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	b := g.SaveBrowser
	if b == nil {
		return
	}
	title, action := "LOAD GAME", "LOAD"
	if b.Saving {
		title, action = "SAVE GAME", "SAVE"
	}
	if b.Confirm {
		action = "REPLACE"
	}
	g.text(title, 112, 12)
	g.text("UP/DOWN: BROWSE SAVES", 24, 27)
	for row := 0; row < 6 && b.Offset+row < len(b.Files); row++ {
		name := b.Files[b.Offset+row]
		if name == b.Name {
			g.text(">", 8, 41+row*12)
		}
		g.text(strings.ToUpper(name[:min(len(name), 35)]), 24, 41+row*12)
	}
	g.text("FILE: "+strings.ToUpper(b.Name[max(0, len(b.Name)-29):])+"_", 16, 119)
	if b.Error != "" {
		message := g.Message
		g.Message = b.Error
		g.drawMessage(136)
		g.Message = message
	}
	g.button("CANCEL", 40, 171, 100)
	g.button(action, 180, 171, 100)
}
