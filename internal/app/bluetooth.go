package app

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"go-populous2/internal/network"
	"go-populous2/internal/platformbridge"
)

// Android owns Bluetooth permissions, discovery and secure RFCOMM sockets. Its
// callbacks only enqueue messages; Update owns all Game and simulation changes.
// The private authenticated proxy carries the ordinary Go lockstep protocol.

func (g *Game) SetBluetoothAvailable(available bool) {
	if g == nil || g.bluetoothAvailable == nil {
		return
	}
	g.bluetoothAvailable.Store(available)
	if !available {
		g.CancelBluetooth()
	}
}

func (g *Game) BluetoothAvailable() bool {
	return g != nil && g.bluetoothAvailable != nil && g.bluetoothAvailable.Load()
}

// RequestBluetoothHost opens an ephemeral loopback listener before asking
// Android to advertise it. The selected host world is sent to the other device
// by the checked session handshake; neither player runs an independent AI.
func (g *Game) RequestBluetoothHost(levelIndex int) error {
	if g == nil || g.bluetooth == nil || !g.BluetoothAvailable() {
		return fmt.Errorf("Bluetooth is unavailable on this device")
	}
	if g.Network != nil {
		return fmt.Errorf("leave the current multiplayer game first")
	}
	if g.Assets == nil || levelIndex < 0 || levelIndex >= len(g.Assets.Levels) {
		return fmt.Errorf("select a valid conquest world before hosting")
	}
	token, err := bluetoothToken()
	if err != nil {
		return err
	}
	defer clear(token)
	if err := g.bluetooth.Reserve(platformbridge.Hosting, "Waiting for a Bluetooth player"); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		g.bluetooth.ReleaseReservation()
		return fmt.Errorf("prepare Bluetooth listener: %w", err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		g.bluetooth.ReleaseReservation()
		return err
	}
	controller, err := bluetoothHostController(listener, token, g.Assets.RulesID, g.bluetooth)
	if err != nil {
		_ = listener.Close()
		g.bluetooth.ReleaseReservation()
		return err
	}
	g.Network, g.LevelIndex, g.CustomGame = controller, levelIndex, false
	if err := g.startConquest(); err != nil {
		controller.Close()
		g.Network = nil
		return err
	}
	if err := g.bluetooth.EnqueueCommand("BT_HOST|" + port + "|" + hex.EncodeToString(token)); err != nil {
		g.cancelBluetoothNow()
		return err
	}
	g.Message, g.messageUntil = "Waiting for a Bluetooth player", g.Updates+150
	return nil
}

func (g *Game) RequestBluetoothJoin() error {
	if g == nil || g.bluetooth == nil || !g.BluetoothAvailable() {
		return fmt.Errorf("Bluetooth is unavailable on this device")
	}
	if g.Network != nil {
		return fmt.Errorf("leave the current multiplayer game first")
	}
	token, err := bluetoothToken()
	if err != nil {
		return err
	}
	defer clear(token)
	if err := g.bluetooth.Reserve(platformbridge.Joining, "Select a Bluetooth device"); err != nil {
		return err
	}
	g.bluetooth.SetPreamble(token)
	if err := g.bluetooth.EnqueueCommand("BT_JOIN|" + hex.EncodeToString(token)); err != nil {
		g.bluetooth.ReleaseReservation()
		return err
	}
	g.Message, g.messageUntil = "Select a Bluetooth device", g.Updates+150
	return nil
}

// The following methods are safe to call from Android's Activity callback
// threads. None of them changes Game state or closes a gameplay socket.
func (g *Game) PollPlatformCommand() string {
	if g == nil {
		return ""
	}
	return g.bluetooth.PollCommand()
}

func (g *Game) PostBluetoothStatus(status string) {
	if g != nil {
		g.bluetooth.EnqueueEvent(platformbridge.Event{Kind: platformbridge.EventStatus, Status: platformbridge.CleanText(status, 120)})
	}
}

func (g *Game) PostBluetoothReady(address, name string) {
	if g != nil {
		g.bluetooth.EnqueueEvent(platformbridge.Event{Kind: platformbridge.EventReady, Address: strings.TrimSpace(address), Name: platformbridge.CleanText(name, 64)})
	}
}

func (g *Game) PostBluetoothFailure(message string) {
	if g != nil {
		g.bluetooth.EnqueueEvent(platformbridge.Event{Kind: platformbridge.EventFailure, Status: platformbridge.CleanText(message, 160)})
	}
}

func (g *Game) CancelBluetooth() {
	if g != nil {
		g.bluetooth.EnqueueEvent(platformbridge.Event{Kind: platformbridge.EventCancel})
	}
}

func (g *Game) BluetoothStatusText() string {
	if g == nil {
		return ""
	}
	return g.bluetooth.StatusText()
}

func (g *Game) updateBluetooth() {
	if g == nil || g.bluetooth == nil {
		return
	}
	for _, event := range g.bluetooth.TakeEvents() {
		switch event.Kind {
		case platformbridge.EventStatus:
			if event.Status != "" && g.bluetooth.SetStatus(event.Status) {
				g.Message, g.messageUntil = event.Status, g.Updates+150
				if c := g.Network; c != nil && c.bluetooth {
					c.mu.Lock()
					if !c.status.Ready && c.status.Failure == nil {
						c.status.Message = event.Status
					}
					c.mu.Unlock()
				}
			}
		case platformbridge.EventReady:
			g.acceptBluetoothProxy(event.Address, event.Name)
		case platformbridge.EventFailure:
			g.failBluetooth(event.Status)
		case platformbridge.EventCancel:
			g.cancelBluetoothNow()
		}
	}
	if g.Network != nil && g.Network.bluetooth {
		status := g.Network.Status()
		if status.Failure != nil {
			g.failBluetooth("Connection interrupted; game paused")
		} else if status.Ready {
			g.bluetooth.SetStatus("Two-player game connected")
		}
	}
}

func (g *Game) acceptBluetoothProxy(address, name string) {
	if g.bluetooth.Phase() != platformbridge.Joining || g.Network != nil {
		return
	}
	address, err := platformbridge.NormalizeLoopbackAddress(address)
	if err != nil {
		g.failBluetooth(err.Error())
		return
	}
	token := g.bluetooth.Preamble()
	defer clear(token)
	controller, err := bluetoothJoinController(address, token, g.Assets.RulesID, g.bluetooth)
	if err != nil {
		g.failBluetooth(err.Error())
		return
	}
	if err := controller.Start(nil); err != nil {
		controller.Close()
		g.failBluetooth(err.Error())
		return
	}
	g.Network = controller
	g.bluetooth.ClearPreamble()
	status := "Connecting over Bluetooth"
	if name != "" {
		status = "Connecting to " + name
	}
	g.bluetooth.SetStatus(status)
	controller.mu.Lock()
	controller.status.Message = status
	controller.mu.Unlock()
	g.Message, g.messageUntil = status, g.Updates+150
}

func (g *Game) failBluetooth(message string) {
	message = platformbridge.CleanText(message, 160)
	if message == "" {
		message = "Connection failed"
	}
	g.cancelBluetoothNow()
	g.Screen = MainMenu
	g.Message, g.messageUntil = "Bluetooth: "+message, g.Updates+300
}

func (g *Game) cancelBluetoothNow() {
	if g == nil || g.bluetooth == nil {
		return
	}
	if g.Network != nil && g.Network.bluetooth {
		g.Network.Close()
		g.Network = nil
		// A detached paused world remains available to display or save; it must
		// not silently resume locally after the second player disconnects.
		g.Paused = true
	}
	g.bluetooth.Stop()
}

func bluetoothToken() ([]byte, error) {
	token := make([]byte, network.BluetoothTokenBytes)
	if _, err := cryptorand.Read(token); err != nil {
		return nil, fmt.Errorf("create Bluetooth proxy authentication: %w", err)
	}
	return token, nil
}

func bluetoothController(rules string, bridge *platformbridge.Bridge, hosting bool) (*NetworkController, error) {
	if rules == "" || bridge == nil {
		return nil, fmt.Errorf("Bluetooth game rules or platform bridge are missing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &NetworkController{ctx: ctx, cancel: cancel, rules: rules, hosting: hosting, bluetooth: true, updates: make(chan networkUpdate, 1), status: NetworkStatus{Side: -1}}, nil
}

func bluetoothHostController(listener net.Listener, token []byte, rules string, bridge *platformbridge.Bridge) (*NetworkController, error) {
	if listener == nil || len(token) != network.BluetoothTokenBytes {
		return nil, fmt.Errorf("Bluetooth proxy listener or token are missing")
	}
	c, err := bluetoothController(rules, bridge, true)
	if err != nil {
		return nil, err
	}
	secret := append([]byte(nil), token...)
	c.openConnection = func(ctx context.Context) (net.Conn, error) {
		defer clear(secret)
		defer listener.Close()
		return network.AcceptBluetoothProxy(ctx, listener, secret)
	}
	c.closeTransport = func() { _ = listener.Close(); bridge.Stop() }
	return c, nil
}

func bluetoothJoinController(address string, token []byte, rules string, bridge *platformbridge.Bridge) (*NetworkController, error) {
	if _, err := platformbridge.NormalizeLoopbackAddress(address); err != nil {
		return nil, err
	}
	if len(token) != network.BluetoothTokenBytes {
		return nil, fmt.Errorf("Bluetooth proxy authentication is unavailable")
	}
	c, err := bluetoothController(rules, bridge, false)
	if err != nil {
		return nil, err
	}
	secret := append([]byte(nil), token...)
	c.openConnection = func(ctx context.Context) (net.Conn, error) {
		defer clear(secret)
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return network.DialBluetoothProxy(ctx, address, secret)
	}
	c.closeTransport = func() { bridge.Stop() }
	return c, nil
}
