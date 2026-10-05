package populous2

import (
	"encoding/binary"
	"fmt"
)

const ScenarioScriptEventCount = 10
const ScenarioScriptRecordSize = 6

type ScenarioScriptEvent struct {
	Time                    uint16
	Reserved, Command, X, Y uint8
}

// DecodeScenarioScript gives names to the exact sixty-byte world table copied
// from template+$7a into BSS$dde by $10e68. A zero time stops execution even
// when later records contain commands. The unused byte remains preserved.
func DecodeScenarioScript(parameters [60]byte) [ScenarioScriptEventCount]ScenarioScriptEvent {
	var events [ScenarioScriptEventCount]ScenarioScriptEvent
	for i := range events {
		at := i * ScenarioScriptRecordSize
		events[i] = ScenarioScriptEvent{Time: binary.BigEndian.Uint16(parameters[at:]), Reserved: parameters[at+2], Command: parameters[at+3], X: parameters[at+4], Y: parameters[at+5]}
	}
	return events
}

// LoadScenarioScript is the exact $10e76 byte copy plus cursor reset$10e7c.
// The remaining per-player template, terrain and RNG initialization belong
// to their own original routines.
func LoadScenarioScript(parameters [60]byte, m FollowerCleanupMemory) error {
	if m.Write8 == nil || m.Write16 == nil {
		return fmt.Errorf("native scenario script memory missing")
	}
	for i, value := range parameters {
		if err := m.Write8(0xdde+i, value); err != nil {
			return err
		}
	}
	return m.Write16(0xf0a, 0)
}

type ScenarioScriptCallbacks struct {
	Memory FollowerCleanupMemory
	// Execute is original $17500 on the four-byte scratch record at$dc4.
	// Owner3 is the native neutral/free source, not local player0. It may
	// change other raw records before returning, but must not roll back the
	// already advanced script cursor after an inadmissible/no-op command.
	Execute func(int) error
}

type ScenarioScriptStep struct {
	Terminated, Pending, Dispatched bool
	Offset, NextOffset              uint16
	Event                           ScenarioScriptEvent
	Owner                           uint8
}

// TickScenarioScript translates $17e9a. It compares event time unsigned with
// the original clock word$f42 and dispatches at most one event per call. The
// cursor is a signed ADDA.W offset in the original BSS; raw aliases remain
// explicit memory accesses, while provider errors bound unavailable storage.
func TickScenarioScript(cb ScenarioScriptCallbacks) (ScenarioScriptStep, error) {
	var step ScenarioScriptStep
	m := cb.Memory
	if m.Read8 == nil || m.Read16 == nil || m.Write8 == nil || m.Write16 == nil {
		return step, fmt.Errorf("native scenario scheduler memory missing")
	}
	offset, err := m.Read16(0xf0a)
	if err != nil {
		return step, err
	}
	step.Offset, step.NextOffset = offset, offset
	if offset&1 != 0 {
		return step, fmt.Errorf("native scenario scheduler word read at odd address")
	}
	a := 0xdde + int(int16(offset))
	time, err := m.Read16(a)
	if err != nil {
		return step, err
	}
	step.Event.Time = time
	if time == 0 {
		step.Terminated = true
		return step, nil
	}
	clock, err := m.Read16(0xf42)
	if err != nil {
		return step, err
	}
	if time > clock {
		step.Pending = true
		return step, nil
	}
	step.NextOffset = offset + 6
	if err := m.Write16(0xf0a, step.NextOffset); err != nil {
		return step, err
	}
	if err := m.Write8(0xdc4, 3); err != nil {
		return step, err
	}
	step.Owner = 3
	mode, err := m.Read16(0xeb44)
	if err != nil {
		return step, err
	}
	if mode == 8 {
		if err := m.Write8(0xdc4, 2); err != nil {
			return step, err
		}
		step.Owner = 2
	}
	command, err := m.Read8(a + 3)
	if err != nil {
		return step, err
	}
	step.Event.Command = command
	if err := m.Write8(0xdc5, command); err != nil {
		return step, err
	}
	packed, err := m.Read16(a + 4)
	if err != nil {
		return step, err
	}
	step.Event.X, step.Event.Y = uint8(packed>>8), uint8(packed)
	if err := m.Write16(0xdc6, packed); err != nil {
		return step, err
	}
	if cb.Execute == nil {
		return step, fmt.Errorf("native scenario command executor missing")
	}
	step.Dispatched = true
	return step, cb.Execute(0xdc4)
}
