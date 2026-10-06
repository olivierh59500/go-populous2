package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	EditingDirectory       bool
	DirectoryInput         string
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
	if g.Assets.FileLayout != nil && clicked {
		layout := g.Assets.FileLayout
		if b.Confirm && g.Assets.OverwriteLayout != nil {
			layout = g.Assets.OverwriteLayout
		}
		if b.Error != "" && !b.Confirm && g.Assets.FileErrorLayout != nil {
			layout = g.Assets.FileErrorLayout
		}
		if g.handleFileAction(layout.ActionAt(x, y)) {
			return
		}
		clicked = false
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
		if b.EditingDirectory {
			if char >= 32 && char <= 126 && len(b.DirectoryInput) < 1024 {
				b.DirectoryInput += string(char)
			}
			continue
		}
		if char >= 32 && char <= 126 && !strings.ContainsRune(`/\:*?"<>|`, char) && len(b.Name) < 96 {
			b.Name += string(char)
			b.Confirm, b.Error = false, ""
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(b.Name) > 0 {
		if b.EditingDirectory {
			if len(b.DirectoryInput) > 0 {
				b.DirectoryInput = b.DirectoryInput[:len(b.DirectoryInput)-1]
			}
			return
		}
		b.Name = b.Name[:len(b.Name)-1]
		b.Confirm = false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) || (ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyA)) {
		b.Name, b.Confirm, b.Error = "", false, ""
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if b.EditingDirectory {
			g.finishFileDirectory()
			return
		}
		g.acceptSaveBrowser()
	}
}

func (g *Game) drawSaveBrowser() {
	if g.drawOriginalFileBrowser() {
		return
	}
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

func (g *Game) handleFileAction(action string) bool {
	b := g.SaveBrowser
	if b == nil {
		return false
	}
	switch action {
	case "cancel":
		if b.Confirm {
			b.Confirm = false
			b.Error = ""
		} else {
			g.closeSaveBrowser()
		}
	case "replace", "submit":
		g.acceptSaveBrowser()
	case "dismiss":
		b.Error = ""
	case "scroll-up":
		b.Offset = max(0, b.Offset-1)
	case "scroll-down":
		b.Offset = min(max(0, len(b.Files)-12), b.Offset+1)
	case "directory":
		b.EditingDirectory = true
		b.DirectoryInput = b.Directory
	case "name":
		b.EditingDirectory = false
	default:
		if strings.HasPrefix(action, "file-") {
			index, err := strconv.Atoi(strings.TrimPrefix(action, "file-"))
			if err == nil && b.Offset+index < len(b.Files) {
				b.Name = b.Files[b.Offset+index]
				b.Confirm, b.Error = false, ""
			}
		} else {
			return false
		}
	}
	return true
}
func (g *Game) finishFileDirectory() {
	b := g.SaveBrowser
	path, err := filepath.Abs(b.DirectoryInput)
	if err != nil {
		b.Error = err.Error()
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		b.Error = err.Error()
		return
	}
	var names []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.Type().IsRegular() && (ext == ".json" || ext == ".gam") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	b.Directory, b.Files, b.Offset, b.EditingDirectory = path, names, 0, false
}
func (g *Game) drawOriginalFileBrowser() bool {
	b := g.SaveBrowser
	l := g.Assets.FileLayout
	if b == nil || l == nil {
		return false
	}
	values := map[string]string{"directory": b.Directory, "name": b.Name, "verb": "LOAD"}
	if b.Saving {
		values["verb"] = "SAVE"
	}
	if b.EditingDirectory {
		values["directory"] = b.DirectoryInput
	}
	for row := 0; row < 12; row++ {
		key := fmt.Sprintf("file-%d", row)
		if b.Offset+row < len(b.Files) {
			values[key] = strings.ToUpper(b.Files[b.Offset+row])
		}
	}
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(l.Palette[0]), image.Point{}, draw.Src)
	l.Draw(g.framebuffer, g.Assets.Visual.Font, values, nil)
	if b.Confirm && g.Assets.OverwriteLayout != nil {
		g.Assets.OverwriteLayout.Draw(g.framebuffer, g.Assets.Visual.Font, map[string]string{"message": b.Name}, nil)
	} else if b.Error != "" && g.Assets.FileErrorLayout != nil {
		g.Assets.FileErrorLayout.Draw(g.framebuffer, g.Assets.Visual.Font, map[string]string{"message": strings.ToUpper(b.Error)}, nil)
	}
	return true
}
