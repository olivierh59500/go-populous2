package platformbridge

import (
	"fmt"
	"sync"
	"testing"
)

func TestBluetoothBridgeCommandLifecycle(t *testing.T) {
	bridge := New()
	if got := bridge.PollCommand(); got != "" {
		t.Fatalf("empty bridge command = %q", got)
	}
	if err := bridge.Reserve(Joining, "SELECT"); err != nil {
		t.Fatal(err)
	}
	bridge.EnqueueCommand("BT_JOIN")
	if got := bridge.PollCommand(); got != "BT_JOIN" {
		t.Fatalf("join command = %q", got)
	}
	bridge.EnqueueCommand("BT_JOIN")
	if !bridge.Stop() {
		t.Fatal("active bridge was not stopped")
	}
	if got := bridge.PollCommand(); got != "BT_CANCEL" {
		t.Fatalf("cancel command = %q", got)
	}
	if got := bridge.PollCommand(); got != "" {
		t.Fatalf("unexpected trailing command = %q", got)
	}
	if bridge.Stop() {
		t.Fatal("stopping an idle bridge queued another cancellation")
	}
	if got := bridge.StatusText(); got != "" {
		t.Fatalf("status after cancellation = %q", got)
	}
}

func TestBluetoothBridgeCoalescesStatusAndIgnoresLateCallbacks(t *testing.T) {
	bridge := New()
	if err := bridge.Reserve(Hosting, "STARTING"); err != nil {
		t.Fatal(err)
	}
	bridge.EnqueueEvent(Event{Kind: EventStatus, Status: "ONE"})
	bridge.EnqueueEvent(Event{Kind: EventStatus, Status: "TWO"})
	events := bridge.TakeEvents()
	if len(events) != 1 || events[0].Status != "TWO" {
		t.Fatalf("coalesced events = %#v", events)
	}
	bridge.Stop()
	bridge.EnqueueEvent(Event{Kind: EventFailure, Status: "LATE"})
	if events := bridge.TakeEvents(); len(events) != 0 {
		t.Fatalf("late events after cancellation = %#v", events)
	}
}

func TestBluetoothBridgeCopiesAndClearsPreamble(t *testing.T) {
	bridge := New()
	if err := bridge.Reserve(Joining, "STARTING"); err != nil {
		t.Fatal(err)
	}
	source := []byte{1, 2, 3, 4}
	if !bridge.SetPreamble(source) {
		t.Fatal("active bridge rejected preamble")
	}
	source[0] = 99
	copyOne := bridge.Preamble()
	if copyOne[0] != 1 {
		t.Fatalf("stored preamble aliases caller: %v", copyOne)
	}
	copyOne[1] = 99
	if got := bridge.Preamble(); got[1] != 2 {
		t.Fatalf("returned preamble aliases bridge: %v", got)
	}
	bridge.Stop()
	if got := bridge.Preamble(); len(got) != 0 {
		t.Fatalf("preamble retained after stop: %v", got)
	}
}

func TestBluetoothBridgeConcurrentCallbacks(t *testing.T) {
	bridge := New()
	if err := bridge.Reserve(Joining, "STARTING"); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for index := 0; index < 8; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for count := 0; count < 100; count++ {
				bridge.EnqueueEvent(Event{Kind: EventStatus, Status: "SEARCHING"})
				_ = bridge.StatusText()
			}
		}()
	}
	workers.Wait()
	if events := bridge.TakeEvents(); len(events) == 0 || len(events) > 32 {
		t.Fatalf("bounded callback count = %d", len(events))
	}
}

func TestBluetoothBridgeBoundsNonStatusEvents(t *testing.T) {
	bridge := New()
	if err := bridge.Reserve(Joining, "STARTING"); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 40; index++ {
		bridge.EnqueueEvent(Event{Kind: EventReady, Address: fmt.Sprintf("127.0.0.1:%d", 1000+index)})
	}
	events := bridge.TakeEvents()
	if len(events) != 32 || events[0].Address != "127.0.0.1:1008" || events[31].Address != "127.0.0.1:1039" {
		t.Fatalf("bounded events = %#v", events)
	}
}

func TestNormalizeBluetoothLoopbackAddress(t *testing.T) {
	for input, want := range map[string]string{
		"127.0.0.1:1234": "127.0.0.1:1234",
		"[::1]:65535":    "[::1]:65535",
	} {
		got, err := NormalizeLoopbackAddress(input)
		if err != nil || got != want {
			t.Errorf("normalize(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{
		"localhost:1234", "192.168.1.1:1234", "127.0.0.1:0",
		"127.0.0.1:65536", "127.0.0.1", "",
	} {
		if got, err := NormalizeLoopbackAddress(input); err == nil {
			t.Errorf("normalize(%q) unexpectedly accepted %q", input, got)
		}
	}
}

func TestCleanBluetoothText(t *testing.T) {
	if got, want := CleanText("  Pixel\n\tplayer\x00  ", 64), "Pixel player"; got != want {
		t.Fatalf("clean text = %q, want %q", got, want)
	}
	if got, want := CleanText("éèàçù", 3), "éèà"; got != want {
		t.Fatalf("rune-limited text = %q, want %q", got, want)
	}
}

func TestBluetoothBridgeBoundsCommandsAndRetainsCancellation(t *testing.T) {
	bridge := New()
	if err := bridge.Reserve(Idle, "invalid"); err == nil {
		t.Fatal("an idle reservation was accepted")
	}
	if err := bridge.Reserve(Hosting, "WAITING"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 32; i++ {
		if err := bridge.EnqueueCommand("BT_HOST"); err != nil {
			t.Fatal(err)
		}
	}
	if err := bridge.EnqueueCommand("BT_HOST"); err == nil {
		t.Fatal("unbounded command queue")
	}
	bridge.Stop()
	if got := bridge.PollCommand(); got != "BT_CANCEL" || bridge.PollCommand() != "" {
		t.Fatalf("cancellation did not replace pending commands: %q", got)
	}
}

func TestBluetoothBridgePreservesTerminalLifecycleEvent(t *testing.T) {
	for _, kind := range []EventKind{EventCancel, EventFailure} {
		bridge := New()
		if err := bridge.Reserve(Joining, "CONNECTING"); err != nil {
			t.Fatal(err)
		}
		bridge.EnqueueEvent(Event{Kind: EventStatus, Status: "OLD"})
		bridge.EnqueueEvent(Event{Kind: kind, Status: "TERMINAL"})
		for i := 0; i < 100; i++ {
			bridge.EnqueueEvent(Event{Kind: EventReady, Address: "127.0.0.1:1234"})
			bridge.EnqueueEvent(Event{Kind: EventStatus, Status: "LATE"})
		}
		events := bridge.TakeEvents()
		if len(events) != 1 || events[0].Kind != kind {
			t.Fatalf("terminal callback was lost: %#v", events)
		}
		bridge.Stop()
	}
}
