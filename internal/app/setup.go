package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"net"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const defaultConnectionAddress = "127.0.0.1:2468"

func (g *Game) openNetworkSetup() {
	if g.connectionAddress == "" {
		g.connectionAddress = defaultConnectionAddress
		g.networkHosting = true
	}
	g.editingConnection = true
	g.Screen = NetworkSetup
}

func (g *Game) updateNetworkSetup(x, y int, clicked bool) {
	input := networkSetupInput{X: x, Y: y, Clicked: clicked, Characters: ebiten.AppendInputChars(nil), Backspace: inpututil.IsKeyJustPressed(ebiten.KeyBackspace), Enter: inpututil.IsKeyJustPressed(ebiten.KeyEnter), Clear: inpututil.IsKeyJustPressed(ebiten.KeyDelete)}
	g.applyNetworkSetupInput(input)
}

type networkSetupInput struct {
	X, Y                             int
	Clicked, Backspace, Enter, Clear bool
	Characters                       []rune
}

func (g *Game) applyNetworkSetupInput(input networkSetupInput) {
	if input.Clicked {
		switch {
		case input.Y >= 48 && input.Y < 65 && input.X >= 28 && input.X < 148:
			g.networkHosting = true
		case input.Y >= 48 && input.Y < 65 && input.X >= 172 && input.X < 292:
			g.networkHosting = false
		case input.Y >= 92 && input.Y < 117 && input.X >= 20 && input.X < 300:
			g.editingConnection = true
		case input.Y >= 171 && input.Y < 190 && input.X >= 20 && input.X < 120:
			g.Screen = MainMenu
			g.editingConnection = false
			return
		case input.Y >= 171 && input.Y < 190 && input.X >= 164 && input.X < 300:
			input.Enter = true
		}
	}
	if g.editingConnection {
		for _, character := range input.Characters {
			if len(g.connectionAddress) >= 253 {
				break
			}
			if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune(".:-[]_", character) {
				g.connectionAddress += string(character)
			}
		}
		if input.Backspace && len(g.connectionAddress) > 0 {
			g.connectionAddress = g.connectionAddress[:len(g.connectionAddress)-1]
		}
		if input.Clear {
			g.connectionAddress = ""
		}
	}
	if input.Enter {
		if err := g.startConfiguredNetwork(); err != nil {
			g.Message, g.messageUntil = err.Error(), g.Updates+200
		}
	}
}

func validateSetupAddress(address string) error {
	address = strings.TrimSpace(address)
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return fmt.Errorf("ENTER HOST:PORT")
	}
	if host == "" {
		return fmt.Errorf("ENTER A HOST NAME OR ADDRESS")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("PORT MUST BE BETWEEN 1 AND 65535")
	}
	return nil
}

func (g *Game) startConfiguredNetwork() error {
	address := strings.TrimSpace(g.connectionAddress)
	if err := validateSetupAddress(address); err != nil {
		return err
	}
	listen, connect := "", address
	if g.networkHosting {
		listen, connect = address, ""
	}
	if err := g.ConfigureNetwork(listen, connect); err != nil {
		return err
	}
	g.editingConnection = false
	if g.networkHosting {
		g.Screen = ConquestBriefing
		g.Message = "CHOOSE A WORLD TO HOST"
	} else {
		g.Screen = MainMenu
		g.Message = "CONNECTING TO THE OTHER PLAYER"
	}
	g.messageUntil = g.Updates + 200
	return nil
}

func (g *Game) drawNetworkSetup() {
	draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	g.text("TWO PLAYERS", 116, 15)
	host, join := "HOST A GAME", "JOIN A GAME"
	if g.networkHosting {
		host = "[ HOST ]"
	} else {
		join = "[ JOIN ]"
	}
	g.button(host, 28, 48, 120)
	g.button(join, 172, 48, 120)
	label := "HOST ADDRESS:PORT"
	if !g.networkHosting {
		label = "OTHER PLAYER ADDRESS:PORT"
	}
	g.text(label, 20, 77)
	address := g.connectionAddress
	if g.editingConnection {
		address += "_"
	}
	if len(address) > 34 {
		address = address[len(address)-34:]
	}
	draw.Draw(g.framebuffer, image.Rect(18, 92, 302, 116), image.NewUniform(color.RGBA{70, 66, 25, 255}), image.Point{}, draw.Src)
	g.text(address, 24, 101)
	g.text("TYPE ADDRESS / DELETE TO CLEAR", 20, 129)
	if g.networkHosting {
		g.text("THEN CHOOSE YOUR WORLD", 20, 144)
	} else {
		g.text("THE HOST CHOOSES THE WORLD", 20, 144)
	}
	if g.Updates < g.messageUntil {
		message := strings.ToUpper(g.Message)
		if len(message) > 35 {
			message = message[:35]
		}
		g.text(message, 20, 158)
	}
	g.button("BACK", 20, 173, 100)
	action := "CONTINUE"
	if !g.networkHosting {
		action = "CONNECT"
	}
	g.button(action, 164, 173, 136)
}
