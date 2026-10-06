package app

import (
	"net"
	"testing"
	"time"
)

func TestNetworkSetupEditsAddressAndPreservesMode(t *testing.T) {
	g := &Game{}
	g.openNetworkSetup()
	if g.Screen != NetworkSetup || !g.networkHosting || g.connectionAddress != defaultConnectionAddress {
		t.Fatal("multiplayer setup did not open with a usable default")
	}
	g.applyNetworkSetupInput(networkSetupInput{Clear: true})
	g.applyNetworkSetupInput(networkSetupInput{Characters: []rune("192.168.1.20:2468 !")})
	if g.connectionAddress != "192.168.1.20:2468" {
		t.Fatal("address editing retained unsupported characters")
	}
	g.applyNetworkSetupInput(networkSetupInput{Backspace: true})
	if g.connectionAddress != "192.168.1.20:246" {
		t.Fatal("address backspace did not remove one character")
	}
	g.applyNetworkSetupInput(networkSetupInput{Clicked: true, X: 200, Y: 55})
	if g.networkHosting {
		t.Fatal("join selection did not change the connection mode")
	}
	g.applyNetworkSetupInput(networkSetupInput{Clicked: true, X: 40, Y: 180})
	if g.Screen != MainMenu || g.editingConnection {
		t.Fatal("back did not leave setup cleanly")
	}
	if g.connectionAddress != "192.168.1.20:246" {
		t.Fatal("returning to the menu discarded the typed address")
	}
}

func TestNetworkSetupHostContinuesToWorldSelection(t *testing.T) {
	g := &Game{Assets: &Assets{RulesID: "test-rules"}}
	g.openNetworkSetup()
	g.applyNetworkSetupInput(networkSetupInput{Enter: true})
	defer g.Network.Close()
	if g.Screen != ConquestBriefing || g.Network == nil || g.Network.started {
		t.Fatal("hosting did not wait for normal world selection")
	}
}

func TestNetworkSetupInvalidAddressDoesNotReplaceAConnection(t *testing.T) {
	g := &Game{Assets: &Assets{RulesID: "test-rules"}, Screen: NetworkSetup, connectionAddress: "localhost", networkHosting: true}
	if err := g.startConfiguredNetwork(); err == nil || g.Network != nil || g.Screen != NetworkSetup {
		t.Fatal("invalid address changed connection or screen state")
	}
	for _, address := range []string{"", ":2468", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:abc"} {
		if err := validateSetupAddress(address); err == nil {
			t.Fatalf("invalid address %q accepted", address)
		}
	}
	for _, address := range []string{"127.0.0.1:2468", "example.test:2468", "[::1]:2468"} {
		if err := validateSetupAddress(address); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNetworkSetupJoinReceivesTheHostsSelectedWorld(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	host, err := NewNetworkController(address, "", "test-rules")
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	if err := host.Start(controllerWorld(t)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		host.mu.Lock()
		ready := host.listener != nil
		host.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("host did not listen")
		}
		time.Sleep(time.Millisecond)
	}
	g := &Game{Assets: &Assets{RulesID: "test-rules"}, Screen: NetworkSetup, connectionAddress: address, networkHosting: false}
	if err := g.startConfiguredNetwork(); err != nil {
		t.Fatal(err)
	}
	defer g.Network.Close()
	if g.Screen != MainMenu || g.Network == nil {
		t.Fatal("join setup did not show connecting state")
	}
	for time.Now().Before(deadline) {
		host.Poll()
		if err := g.pollNetwork(); err != nil {
			t.Fatal(err)
		}
		if g.Screen == Playing && g.World != nil {
			if g.playerSide() != 1 || g.World.Level.Seed != 4311 {
				t.Fatal("joining setup selected the wrong player or world")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("join setup did not publish the host world")
}

func TestFailedNetworkStartDoesNotReplaceLiveGame(t *testing.T) {
	assets := menuTestGame(t).Assets
	controller, err := NewNetworkController("127.0.0.1:2468", "", "test-rules")
	if err != nil {
		t.Fatal(err)
	}
	controller.Close()
	world := controllerWorld(t)
	g := &Game{Assets: assets, World: world, Screen: ConquestBriefing, Network: controller}
	if err := g.startConquest(); err == nil || g.World != world || g.Screen != ConquestBriefing {
		t.Fatal("failed connection setup replaced the displayed game")
	}
}
