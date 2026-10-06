package app

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"go-populous2/internal/engine"
	"go-populous2/internal/network"
	"go-populous2/internal/platformbridge"
)

func TestBluetoothControllersPublishOnlyCompletedLockstepWorlds(t *testing.T) {
	hostBridge, joinBridge := platformbridge.New(), platformbridge.New()
	_ = hostBridge.Reserve(platformbridge.Hosting, "WAITING")
	_ = joinBridge.Reserve(platformbridge.Joining, "CONNECTING")
	hostListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer hostListener.Close()
	joinListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer joinListener.Close()
	hostToken := bytes.Repeat([]byte{0x17}, network.BluetoothTokenBytes)
	joinToken := bytes.Repeat([]byte{0x31}, network.BluetoothTokenBytes)
	host, err := bluetoothHostController(hostListener, hostToken, "mobile-rules", hostBridge)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	join, err := bluetoothJoinController(joinListener.Addr().String(), joinToken, "mobile-rules", joinBridge)
	if err != nil {
		t.Fatal(err)
	}
	defer join.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proxyErrors := make(chan error, 1)
	go func() {
		bridgeHost, err := network.DialBluetoothProxy(ctx, hostListener.Addr().String(), hostToken)
		if err != nil {
			proxyErrors <- err
			return
		}
		defer bridgeHost.Close()
		bridgeJoin, err := network.AcceptBluetoothProxy(ctx, joinListener, joinToken)
		if err != nil {
			proxyErrors <- err
			return
		}
		defer bridgeJoin.Close()
		finished := make(chan struct{}, 1)
		go func() { _, _ = io.Copy(bridgeHost, bridgeJoin); finished <- struct{}{} }()
		go func() { _, _ = io.Copy(bridgeJoin, bridgeHost); finished <- struct{}{} }()
		select {
		case <-finished:
		case <-ctx.Done():
		}
	}()
	if err := host.Start(controllerWorld(t)); err != nil {
		t.Fatal(err)
	}
	if err := join.Start(nil); err != nil {
		t.Fatal(err)
	}
	var hostWorld, joinWorld *engine.World
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-proxyErrors:
			t.Fatal(err)
		default:
		}
		if world, _, status := host.Poll(); status.Failure != nil {
			t.Fatal(status.Failure)
		} else if world != nil {
			hostWorld = world
		}
		if world, _, status := join.Poll(); status.Failure != nil {
			t.Fatal(status.Failure)
		} else if world != nil {
			joinWorld = world
		}
		if hostWorld != nil && joinWorld != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if hostWorld == nil || joinWorld == nil || !reflect.DeepEqual(hostWorld.Snapshot(), joinWorld.Snapshot()) {
		t.Fatal("Bluetooth controllers did not receive the identical host snapshot")
	}
	for side := range hostWorld.Players {
		if hostWorld.Players[side].Computer || hostWorld.Players[side].Assisted {
			t.Fatal("a Bluetooth game retained automatic player control")
		}
	}
	before := hostWorld.Snapshot()
	if err := host.Submit(network.Command{Kind: "mode", Mode: engine.Fight}); err != nil {
		t.Fatal(err)
	}
	if err := join.Submit(network.Command{Kind: "rally", Target: engine.PowerTarget{X: 24, Y: 25}}); err != nil {
		t.Fatal(err)
	}
	if !host.BeginRound(hostWorld) || host.BeginRound(hostWorld) || !join.BeginRound(joinWorld) {
		t.Fatal("Bluetooth backpressure allowed duplicate in-flight rounds")
	}
	var nextHost, nextJoin *engine.World
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if world, _, status := host.Poll(); status.Failure != nil {
			t.Fatal(status.Failure)
		} else if world != nil {
			nextHost = world
		}
		if world, _, status := join.Poll(); status.Failure != nil {
			t.Fatal(status.Failure)
		} else if world != nil {
			nextJoin = world
		}
		if nextHost != nil && nextJoin != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if nextHost == nil || nextJoin == nil || !reflect.DeepEqual(nextHost.Snapshot(), nextJoin.Snapshot()) || !reflect.DeepEqual(before, hostWorld.Snapshot()) {
		t.Fatal("Bluetooth controller exposed an incomplete or divergent round")
	}
	for i := 0; i < 64; i++ {
		if err := host.Submit(network.Command{Kind: "mode", Mode: engine.Join}); err != nil {
			t.Fatal(err)
		}
	}
	if err := host.Submit(network.Command{Kind: "mode", Mode: engine.Join}); err == nil {
		t.Fatal("Bluetooth input queue grew beyond its bounded capacity")
	}
	host.Close()
	if hostBridge.PollCommand() != "BT_CANCEL" || hostBridge.Phase() != platformbridge.Idle {
		t.Fatal("closing Bluetooth did not cancel the Android transport")
	}
}

func TestBluetoothJoinCallbacksStayOnUpdateAndRejectRemoteProxy(t *testing.T) {
	g := &Game{Assets: &Assets{RulesID: "test"}, bluetooth: platformbridge.New(), bluetoothAvailable: &atomic.Bool{}}
	g.SetBluetoothAvailable(true)
	if err := g.RequestBluetoothJoin(); err != nil {
		t.Fatal(err)
	}
	if command := g.PollPlatformCommand(); len(command) != len("BT_JOIN|")+32 {
		t.Fatalf("join command has invalid authentication token: %q", command)
	}
	g.PostBluetoothReady("192.168.1.23:1234", "some device")
	if g.Network != nil || g.Message == "Bluetooth: Bluetooth proxy is not on loopback" {
		t.Fatal("Android callback changed game state immediately")
	}
	g.updateBluetooth()
	if g.Network != nil || g.bluetooth.Phase() != platformbridge.Idle || g.Message != "Bluetooth: Bluetooth proxy is not on loopback" {
		t.Fatalf("untrusted ready callback was not rejected: %s", g.Message)
	}
	g.PostBluetoothReady("127.0.0.1:1234", "late callback")
	g.updateBluetooth()
	if g.Network != nil {
		t.Fatal("late Android callback revived a canceled session")
	}
}

func TestBluetoothLifecycleCancellationPausesDetachedWorld(t *testing.T) {
	bridge := platformbridge.New()
	_ = bridge.Reserve(platformbridge.Joining, "CONNECTED")
	controller, err := bluetoothController("rules", bridge, false)
	if err != nil {
		t.Fatal(err)
	}
	controller.closeTransport = func() { bridge.Stop() }
	g := &Game{World: controllerWorld(t), Network: controller, bluetooth: bridge, Screen: Playing}
	before := g.World.Snapshot()
	g.CancelBluetooth()
	if g.Network == nil || g.Paused {
		t.Fatal("lifecycle callback changed the game outside Update")
	}
	g.updateBluetooth()
	if g.Network != nil || !g.Paused || !reflect.DeepEqual(before, g.World.Snapshot()) {
		t.Fatal("lifecycle cancellation changed or resumed the detached world")
	}
}

func TestBluetoothAvailabilityIsSafeForUninitializedGame(t *testing.T) {
	g := &Game{}
	g.SetBluetoothAvailable(true)
	if g.BluetoothAvailable() {
		t.Fatal("uninitialized game exposed an Android capability")
	}
	g.CancelBluetooth()
	if g.PollPlatformCommand() != "" || g.BluetoothStatusText() != "" {
		t.Fatal("uninitialized platform bridge exposed stale work")
	}
}

func TestPrivateMobileBluetoothHostPreservesCampaignRulesAndAllPowerChoices(t *testing.T) {
	path := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if path == "" {
		t.Skip("locally exported original artwork is not distributed")
	}
	assets, err := LoadAssets(assetsOnlyFS{os.DirFS(path)})
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewMobile(assets)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	g.SetBluetoothAvailable(true)
	g.Profile.Experience = [6]uint8{1, 2, 3, 4, 5, 6}
	const levelIndex = 7
	level := assets.Levels[levelIndex]
	expected, err := engine.NewWorld(level, assets.Landscapes[level.Landscape])
	if err != nil {
		t.Fatal(err)
	}
	expected.Players[0].Experience = g.Profile.Experience
	for side := range expected.Players {
		expected.Players[side].Computer, expected.Players[side].Assisted = false, false
	}
	if err := g.RequestBluetoothHost(levelIndex); err != nil {
		t.Fatal(err)
	}
	before := g.World.Snapshot()
	if !reflect.DeepEqual(expected.Snapshot(), before) || g.Screen != Playing || g.Network == nil {
		t.Fatal("mobile Bluetooth hosting changed campaign rules, powers or initial state")
	}
	seen := make(map[engine.PowerID]bool)
	for element := engine.People; element <= engine.Water; element++ {
		if err := g.handleMobileAction("element-" + strconv.Itoa(int(element))); err != nil {
			t.Fatal(err)
		}
		g.mobile.Overlay = "powers"
		buttons := g.mobileHUDButtons()
		for _, power := range engine.Powers {
			if power.Element != element {
				continue
			}
			action := "power-" + strconv.Itoa(int(power.ID))
			for _, button := range buttons {
				if button.Action == action {
					if button.Enabled != level.Players[0].Powers[power.ID] {
						t.Fatalf("touch power %s bypassed the campaign availability rule", power.Name)
					}
					seen[power.ID] = true
				}
			}
		}
	}
	if len(seen) != len(engine.Powers) || len(seen) != 29 || !reflect.DeepEqual(before, g.World.Snapshot()) {
		t.Fatal("touch menus omitted a power or edited the hosted world while selecting a category")
	}
}
