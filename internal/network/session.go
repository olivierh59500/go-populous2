package network

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"

	"go-populous2/internal/engine"
)

// Digest covers all semantic continuation state, including private motion and
// random/allocation state captured by the versioned snapshot.
func Digest(world *engine.World) (string, error) {
	if world == nil {
		return "", fmt.Errorf("network world is missing")
	}
	snapshot := world.Snapshot()
	if _, err := snapshot.Restore(); err != nil {
		return "", err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Host sends a detached initial state and verifies the joining peer's exact
// digest and rules identifier before returning a usable session.
func Host(ctx context.Context, conn net.Conn, world *engine.World, rules string) (*Session, error) {
	if conn == nil || rules == "" {
		return nil, fmt.Errorf("network connection or rules identifier is missing")
	}
	cleanup := contextDeadline(ctx, conn)
	defer cleanup()
	digest, err := Digest(world)
	if err != nil {
		return nil, err
	}
	snapshot := world.Snapshot()
	if err := writeMessage(conn, message{Version: ProtocolVersion, Kind: "hello", Side: 0, Digest: digest, Rules: rules, Snapshot: &snapshot}); err != nil {
		return nil, err
	}
	reply, err := readMessage(conn)
	if err != nil {
		return nil, err
	}
	if reply.Kind != "ready" || reply.Side != 1 || reply.Digest != digest || reply.Rules != rules || reply.Snapshot != nil || len(reply.Commands) != 0 {
		return nil, fmt.Errorf("network handshake state or rules mismatch")
	}
	return &Session{conn: conn, side: 0, rules: rules}, nil
}

// Join validates the initial state on a new World. A failed handshake never
// modifies the caller's running simulation.
func Join(ctx context.Context, conn net.Conn, rules string) (*Session, *engine.World, error) {
	if conn == nil || rules == "" {
		return nil, nil, fmt.Errorf("network connection or rules identifier is missing")
	}
	cleanup := contextDeadline(ctx, conn)
	defer cleanup()
	hello, err := readMessage(conn)
	if err != nil {
		return nil, nil, err
	}
	if hello.Kind != "hello" || hello.Side != 0 || hello.Rules != rules || hello.Snapshot == nil || len(hello.Commands) != 0 {
		return nil, nil, fmt.Errorf("invalid network handshake or incompatible rules")
	}
	world, err := hello.Snapshot.Restore()
	if err != nil {
		return nil, nil, err
	}
	digest, err := Digest(world)
	if err != nil {
		return nil, nil, err
	}
	if digest != hello.Digest {
		return nil, nil, fmt.Errorf("network initial state digest mismatch")
	}
	if err := writeMessage(conn, message{Version: ProtocolVersion, Kind: "ready", Side: 1, Digest: digest, Rules: rules}); err != nil {
		return nil, nil, err
	}
	return &Session{conn: conn, side: 1, rules: rules}, world, nil
}

type CommandResult struct {
	Side, Index int
	Error       string
}

// Advance exchanges one input round, validates the peer, then applies blue
// commands followed by red commands and one simulation step. A disconnected,
// timed out or divergent session pauses before either side's local mutation.
func (s *Session) Advance(ctx context.Context, world *engine.World, commands []Command) ([]CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return nil, s.failure
	}
	if err := validateCommands(commands, s.side); err != nil {
		return nil, err
	}
	digest, err := Digest(world)
	if err != nil {
		return nil, err
	}
	cleanup := contextDeadline(ctx, s.conn)
	defer cleanup()
	local := message{Version: ProtocolVersion, Kind: "round", Side: s.side, Round: s.round, Digest: digest, Rules: s.rules, Commands: commands}
	var peer message
	if s.side == 0 {
		err = writeMessage(s.conn, local)
		if err == nil {
			peer, err = readMessage(s.conn)
		}
	} else {
		peer, err = readMessage(s.conn)
		if err == nil {
			err = writeMessage(s.conn, local)
		}
	}
	if err != nil {
		s.failure = fmt.Errorf("network session paused: %w", err)
		return nil, s.failure
	}
	if peer.Kind != "round" || peer.Side != 1-s.side || peer.Round != s.round || peer.Digest != digest || peer.Rules != s.rules || peer.Snapshot != nil {
		s.failure = fmt.Errorf("network session paused: unexpected round or divergent state")
		return nil, s.failure
	}
	if err := validateCommands(peer.Commands, peer.Side); err != nil {
		s.failure = fmt.Errorf("network session paused: %w", err)
		return nil, s.failure
	}
	// Both peers acknowledge the validated round before either simulation can
	// change. A rejected input therefore cannot leave only one side advanced.
	ready := message{Version: ProtocolVersion, Kind: "round-ready", Side: s.side, Round: s.round, Digest: digest, Rules: s.rules}
	var acknowledgement message
	if s.side == 0 {
		err = writeMessage(s.conn, ready)
		if err == nil {
			acknowledgement, err = readMessage(s.conn)
		}
	} else {
		acknowledgement, err = readMessage(s.conn)
		if err == nil {
			err = writeMessage(s.conn, ready)
		}
	}
	if err != nil {
		s.failure = fmt.Errorf("network session paused before round confirmation: %w", err)
		return nil, s.failure
	}
	if acknowledgement.Kind != "round-ready" || acknowledgement.Side != 1-s.side || acknowledgement.Round != s.round || acknowledgement.Digest != digest || acknowledgement.Rules != s.rules || acknowledgement.Snapshot != nil || len(acknowledgement.Commands) != 0 {
		s.failure = fmt.Errorf("network session paused: invalid round confirmation")
		return nil, s.failure
	}
	ordered := [2][]Command{}
	ordered[s.side], ordered[peer.Side] = commands, peer.Commands
	var results []CommandResult
	for side := 0; side < 2; side++ {
		for index, command := range ordered[side] {
			result := CommandResult{Side: side, Index: index}
			if err := applyCommand(world, side, command); err != nil {
				result.Error = err.Error()
			}
			results = append(results, result)
		}
	}
	world.Step()
	s.round++
	return results, nil
}

func validateCommands(commands []Command, side int) error {
	if side < 0 || side > 1 || len(commands) > maximumCommands {
		return fmt.Errorf("invalid network input count or side")
	}
	for _, command := range commands {
		switch command.Kind {
		case "power":
			if _, ok := engine.PowerByID(command.Power); !ok {
				return fmt.Errorf("unknown network power")
			}
			if command.Target.X < 0 || command.Target.X > 64 || command.Target.Y < 0 || command.Target.Y > 64 || command.Target.EndX < 0 || command.Target.EndX > 64 || command.Target.EndY < 0 || command.Target.EndY > 64 || command.Target.Direction > 3 {
				return fmt.Errorf("invalid network power target")
			}
		case "mode":
			if command.Mode > engine.Fight {
				return fmt.Errorf("invalid network follower mode")
			}
		case "rally", "terrain-secondary":
			if command.Target.X < 0 || command.Target.X >= 64 || command.Target.Y < 0 || command.Target.Y >= 64 {
				return fmt.Errorf("invalid network rally target")
			}
		case "evacuate":
			if command.Follower <= 0 || command.Follower >= engine.FollowerCapacity {
				return fmt.Errorf("invalid network follower")
			}
		case "lightning-activate", "lightning-dismiss":
		default:
			return fmt.Errorf("unknown network input kind %q", command.Kind)
		}
	}
	return nil
}
func applyCommand(world *engine.World, side int, command Command) error {
	switch command.Kind {
	case "power":
		return world.Cast(side, command.Power, command.Target)
	case "mode":
		if !world.SetMode(side, command.Mode) {
			return fmt.Errorf("follower mode was rejected")
		}
	case "rally":
		if !world.SetRally(side, command.Target.X, command.Target.Y) {
			return fmt.Errorf("rally order was rejected")
		}
	case "evacuate":
		if world.Followers[command.Follower].Owner != uint8(side) {
			return fmt.Errorf("follower belongs to the other player")
		}
		if !world.Evacuate(command.Follower) {
			return fmt.Errorf("town evacuation was rejected")
		}
	case "terrain-secondary":
		if !world.Sprog(side, command.Target.X, command.Target.Y) {
			target := command.Target
			target.Lower = true
			return world.Cast(side, engine.RaiseLower, target)
		}
	case "lightning-activate":
		return world.ActivateLightning(side)
	case "lightning-dismiss":
		if !world.DismissLightning(side) {
			return fmt.Errorf("lightning marker was absent")
		}
	}
	return nil
}
