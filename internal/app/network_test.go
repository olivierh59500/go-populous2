package app

import (
	"net"
	"reflect"
	"testing"
	"time"

	"go-populous2/internal/engine"
	"go-populous2/internal/network"
)

func controllerWorld(t *testing.T) *engine.World {
	t.Helper()
	land := engine.Landscape{}
	for stage := 1; stage < engine.TownStages; stage++ {
		land.WorkTicks[stage] = 8
		land.EmigrationDivisor[stage] = 3
		land.PopulationLimit[stage] = 1000
		land.PopulationAdd[stage] = 1
	}
	level := engine.Level{Seed: 4311}
	for side := range level.Players {
		level.Players[side] = engine.PlayerOptions{Groups: 2, Population: 500, Mana: 10000, MovementSpeed: 20}
	}
	w, err := engine.NewWorld(level, land)
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func controllerPair(t *testing.T) (*NetworkController, *NetworkController, *engine.World, *engine.World) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	host, err := NewNetworkController(address, "", "rules-test-v1")
	if err != nil {
		t.Fatal(err)
	}
	join, err := NewNetworkController("", address, "rules-test-v1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(host.Close)
	t.Cleanup(join.Close)
	if err := host.Start(controllerWorld(t)); err != nil {
		t.Fatal(err)
	}
	// Allow the asynchronous host to bind before the joining dial begins.
	deadline := time.Now().Add(2 * time.Second)
	for {
		host.mu.Lock()
		ready := host.listener != nil
		host.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("host did not begin listening")
		}
		time.Sleep(time.Millisecond)
	}
	if err := join.Start(nil); err != nil {
		t.Fatal(err)
	}
	var a, b *engine.World
	for time.Now().Before(deadline) {
		if w, _, status := host.Poll(); status.Failure != nil {
			t.Fatal(status.Failure)
		} else if w != nil {
			a = w
		}
		if w, _, status := join.Poll(); status.Failure != nil {
			t.Fatal(status.Failure)
		} else if w != nil {
			b = w
		}
		if a != nil && b != nil {
			return host, join, a, b
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("controllers did not complete their handshake")
	return nil, nil, nil, nil
}

func TestNetworkControllerPublishesOnlyCompletedDetachedWorlds(t *testing.T) {
	host, join, a, b := controllerPair(t)
	if host.Status().Side != 0 || join.Status().Side != 1 || a.Players[1].Computer || b.Players[1].Computer {
		t.Fatal("multiplayer handshake did not assign sides or disable AI")
	}
	beforeA, beforeB := a.Snapshot(), b.Snapshot()
	if err := host.Submit(network.Command{Kind: "mode", Mode: engine.Fight}); err != nil {
		t.Fatal(err)
	}
	if err := join.Submit(network.Command{Kind: "rally", Target: engine.PowerTarget{X: 45, Y: 45}}); err != nil {
		t.Fatal(err)
	}
	if !host.BeginRound(a) || host.BeginRound(a) {
		t.Fatal("controller allowed duplicate in-flight rounds")
	}
	if !join.BeginRound(b) {
		t.Fatal("joining controller did not begin a round")
	}
	deadline := time.Now().Add(2 * time.Second)
	var nextA, nextB *engine.World
	for time.Now().Before(deadline) {
		if w, _, status := host.Poll(); status.Failure != nil {
			t.Fatal(status.Failure)
		} else if w != nil {
			nextA = w
		}
		if w, _, status := join.Poll(); status.Failure != nil {
			t.Fatal(status.Failure)
		} else if w != nil {
			nextB = w
		}
		if nextA != nil && nextB != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if nextA == nil || nextB == nil || nextA.Tick != 1 || !reflect.DeepEqual(nextA.Snapshot(), nextB.Snapshot()) {
		t.Fatal("round did not publish identical complete states")
	}
	if !reflect.DeepEqual(beforeA, a.Snapshot()) || !reflect.DeepEqual(beforeB, b.Snapshot()) {
		t.Fatal("background worker changed a displayed world")
	}
}

func TestNetworkControllerCloseInterruptsWaitingPeerAndKeepsQueuedInput(t *testing.T) {
	host, join, a, _ := controllerPair(t)
	if !host.BeginRound(a) {
		t.Fatal("host did not enter waiting round")
	}
	if err := host.Submit(network.Command{Kind: "mode", Mode: engine.Join}); err != nil {
		t.Fatal(err)
	}
	join.Close()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, _, status := host.Poll()
		if status.Failure != nil {
			host.mu.Lock()
			count := len(host.queued)
			host.mu.Unlock()
			if count != 1 || a.Tick != 0 {
				t.Fatal("disconnect discarded queued input or mutated the visible game")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("disconnected controller did not pause")
}

func TestNetworkControllerRejectsAmbiguousAndInvalidAddresses(t *testing.T) {
	for _, addresses := range [][2]string{{"", ""}, {"127.0.0.1:1234", "127.0.0.1:1234"}, {"localhost", ""}} {
		if controller, err := NewNetworkController(addresses[0], addresses[1], "rules"); err == nil {
			controller.Close()
			t.Fatal("invalid multiplayer configuration was accepted")
		}
	}
}

func TestNetworkControllerJoinCanBeginBeforeHostListens(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	join, err := NewNetworkController("", address, "rules-test-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer join.Close()
	if err := join.Start(nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if status := join.Status(); status.Failure != nil {
		t.Fatal("joining failed before the host could begin")
	}
	host, err := NewNetworkController(address, "", "rules-test-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	if err := host.Start(controllerWorld(t)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		host.Poll()
		world, _, status := join.Poll()
		if status.Failure != nil {
			t.Fatal(status.Failure)
		}
		if world != nil && status.Ready && status.Side == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("joining did not retry until the host became available")
}
