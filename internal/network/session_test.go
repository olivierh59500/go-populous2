package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"go-populous2/internal/engine"
)

func networkWorld(t *testing.T) *engine.World {
	t.Helper()
	land := engine.Landscape{}
	for stage := 1; stage < engine.TownStages; stage++ {
		land.WorkTicks[stage] = 8
		land.EmigrationDivisor[stage] = 3
		land.PopulationLimit[stage] = 1000
		land.PopulationAdd[stage] = 5
		land.ManaAdd[stage] = 5
	}
	level := engine.Level{Seed: 4311}
	for owner := range level.Players {
		level.Players[owner] = engine.PlayerOptions{Groups: 2, Population: 500, Mana: 100000, MovementSpeed: 20}
		for _, id := range []engine.PowerID{engine.RaiseLower, engine.PapalMagnet, engine.FireColumn} {
			level.Players[owner].Powers[id] = true
		}
	}
	w, err := engine.NewWorld(level, land)
	if err != nil {
		t.Fatal(err)
	}
	for owner := range w.Players {
		w.Players[owner].Computer = false
	}
	return w
}
func testContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 5*time.Second)
}
func pairedSessions(t *testing.T, left, right net.Conn) (*Session, *Session, *engine.World, *engine.World) {
	t.Helper()
	ctx, cancel := testContext(t)
	defer cancel()
	a := networkWorld(t)
	type joined struct {
		s *Session
		w *engine.World
		e error
	}
	done := make(chan joined, 1)
	go func() { s, w, e := Join(ctx, right, "rules-test-v1"); done <- joined{s, w, e} }()
	host, err := Host(ctx, left, a, "rules-test-v1")
	if err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.e != nil {
		t.Fatal(result.e)
	}
	return host, result.s, a, result.w
}
func runRound(t *testing.T, a, b *Session, wa, wb *engine.World, ca, cb []Command) (error, error) {
	t.Helper()
	ctx, cancel := testContext(t)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := b.Advance(ctx, wb, cb); done <- err }()
	_, left := a.Advance(ctx, wa, ca)
	return left, <-done
}

func TestRealTCPTwoPeersApplyTheSameOrderedInputs(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	fail := make(chan error, 1)
	go func() {
		c, e := listener.Accept()
		if e != nil {
			fail <- e
			return
		}
		accepted <- c
	}()
	client, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var server net.Conn
	select {
	case server = <-accepted:
	case err := <-fail:
		t.Fatal(err)
	}
	a, b, wa, wb := pairedSessions(t, server, client)
	defer a.Close()
	defer b.Close()
	for round := 0; round < 40; round++ {
		ca := []Command{{Kind: "mode", Mode: engine.Settle}}
		cb := []Command{{Kind: "rally", Target: engine.PowerTarget{X: 45, Y: 45}}}
		if round == 10 {
			ca = append(ca, Command{Kind: "power", Power: engine.FireColumn, Target: engine.PowerTarget{X: 20, Y: 20}})
		}
		left, right := runRound(t, a, b, wa, wb, ca, cb)
		if left != nil || right != nil {
			t.Fatalf("round%d errors %v/%v", round, left, right)
		}
		if !reflect.DeepEqual(wa.Snapshot(), wb.Snapshot()) {
			t.Fatalf("peer state diverged at round%d", round)
		}
	}
}

type fragmentedConn struct{ net.Conn }

func (c fragmentedConn) Write(data []byte) (int, error) {
	return c.Conn.Write(data[:min(3, len(data))])
}
func (c fragmentedConn) Read(data []byte) (int, error) { return c.Conn.Read(data[:min(5, len(data))]) }
func TestPartialNetworkReadsAndWritesPreserveFrameBoundaries(t *testing.T) {
	l, r := net.Pipe()
	a, b, wa, wb := pairedSessions(t, fragmentedConn{l}, fragmentedConn{r})
	defer a.Close()
	defer b.Close()
	left, right := runRound(t, a, b, wa, wb, nil, nil)
	if left != nil || right != nil {
		t.Fatal(left, right)
	}
	if wa.Tick != 1 || !reflect.DeepEqual(wa.Snapshot(), wb.Snapshot()) {
		t.Fatal("partial frames changed lockstep inputs")
	}
}

func TestDivergentPeerPausesBothWorldsBeforeMutation(t *testing.T) {
	l, r := net.Pipe()
	a, b, wa, wb := pairedSessions(t, l, r)
	defer a.Close()
	defer b.Close()
	wb.Players[1].Mana++
	beforeA, beforeB := wa.Snapshot(), wb.Snapshot()
	left, right := runRound(t, a, b, wa, wb, []Command{{Kind: "mode", Mode: engine.Fight}}, nil)
	if left == nil || right == nil || !reflect.DeepEqual(beforeA, wa.Snapshot()) || !reflect.DeepEqual(beforeB, wb.Snapshot()) {
		t.Fatal("desynchronized peers mutated game state")
	}
}

func TestDisconnectedSessionDoesNotApplyPendingCommands(t *testing.T) {
	l, r := net.Pipe()
	a, b, wa, _ := pairedSessions(t, l, r)
	defer a.Close()
	b.Close()
	before := wa.Snapshot()
	ctx, cancel := testContext(t)
	defer cancel()
	if _, err := a.Advance(ctx, wa, []Command{{Kind: "mode", Mode: engine.Fight}}); err == nil || !reflect.DeepEqual(before, wa.Snapshot()) {
		t.Fatal("disconnection applied unconfirmed local input")
	}
}

func TestClosingSessionInterruptsAWaitingRound(t *testing.T) {
	l, r := net.Pipe()
	a, b, wa, _ := pairedSessions(t, l, r)
	defer b.Close()
	before := wa.Snapshot()
	done := make(chan error, 1)
	go func() { _, err := a.Advance(context.Background(), wa, nil); done <- err }()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !reflect.DeepEqual(before, wa.Snapshot()) {
			t.Fatal("closed exchange mutated its world")
		}
	case <-time.After(time.Second):
		t.Fatal("Close could not interrupt a waiting exchange")
	}
}

func TestUnexpectedFrameAndOversizedLengthAreRejected(t *testing.T) {
	for _, name := range []string{"kind", "size"} {
		t.Run(name, func(t *testing.T) {
			l, r := net.Pipe()
			defer l.Close()
			defer r.Close()
			ctx, cancel := testContext(t)
			defer cancel()
			cleanup := contextDeadline(ctx, l)
			defer cleanup()
			go func() {
				if name == "size" {
					var header [4]byte
					binary.BigEndian.PutUint32(header[:], maximumFrameBytes+1)
					r.Write(header[:])
				} else {
					writeMessage(r, message{Version: ProtocolVersion, Kind: "unexpected", Side: 0, Rules: "rules"})
				}
			}()
			if name == "size" {
				if _, err := readMessage(l); err == nil {
					t.Fatal("oversized frame was accepted")
				}
			} else {
				if _, _, err := Join(ctx, l, "rules"); err == nil {
					t.Fatal("unexpected handshake kind was accepted")
				}
			}
		})
	}
}

func TestRulesMismatchAndInvalidInputsDoNotEnterGameplay(t *testing.T) {
	l, r := net.Pipe()
	defer l.Close()
	defer r.Close()
	ctx, cancel := testContext(t)
	defer cancel()
	w := networkWorld(t)
	done := make(chan error, 1)
	go func() { _, _, err := Join(ctx, r, "different-rules"); r.Close(); done <- err }()
	if _, err := Host(ctx, l, w, "rules-test-v1"); err == nil {
		t.Fatal("different rules entered gameplay")
	}
	if err := <-done; err == nil {
		t.Fatal("joining peer accepted incompatible rules")
	}
	for _, commands := range [][]Command{{{Kind: "unknown"}}, {{Kind: "power", Power: 255}}, {{Kind: "rally", Target: engine.PowerTarget{X: 64}}}} {
		if err := validateCommands(commands, 0); err == nil {
			t.Fatal("malformed input was accepted")
		}
	}
}

func ExampleCommand() {
	command := Command{Kind: "power", Power: engine.FireColumn, Target: engine.PowerTarget{X: 20, Y: 20}}
	fmt.Println(command.Kind, command.Target.X, command.Target.Y)
	// Output: power 20 20
}
