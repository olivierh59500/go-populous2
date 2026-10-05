package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestWorldScenarioSchedulerAgainstOriginalMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/scenario_script_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input scenarioScriptInput
			Trace []scenarioScriptSnapshot
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked, updates := 0, 0
	for _, f := range catalog.Cases {
		checked++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := &scenarioScriptMemory{}
			for i := range 4096 {
				m[0xf44+i*4] = 0xa8
				m[0xf45+i*4] = 15
			}
			copy(m[0xdde:], f.Input.Parameters[:])
			_ = m.write16(0xf0a, f.Input.Offset)
			_ = m.write16(0xf42, f.Input.Clock)
			_ = m.write16(0xeb44, f.Input.GameMode)
			_ = m.write32(0xdc4, 0xaabbccdd)
			_ = m.write32(0xeb28, f.Input.Seed)
			w := installNativeFixtureWorld(t, m[:], nil)
			s := w.Core.Snapshot()
			s.RNG = f.Input.Seed
			w.Core = legacy.WorldFromSnapshot(s, w.Core.Rules)
			w.NativeGameMode = f.Input.GameMode
			w.nativeCallDepth++
			for _, want := range f.Trace {
				updates++
				if f.Input.Mode == "load" {
					err = LoadScenarioScript(f.Input.Parameters, w.nativeCleanupMemory())
				} else {
					execute := func(int) error { return nil }
					if f.Input.Execute {
						execute = w.executeNativeScriptCommand
					}
					_, err = TickScenarioScript(ScenarioScriptCallbacks{Memory: w.nativeCleanupMemory(), Execute: execute})
				}
				if err != nil {
					t.Fatal(err)
				}
				all := nativeFixtureWorldImage(w, m[:])
				if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash || w.Core.RandomState() != want.RNG {
					for _, change := range want.Changes {
						if all[change.Address] != change.Value {
							t.Errorf("native byte%x got%x want%x", change.Address, all[change.Address], change.Value)
						}
					}
					t.Fatal("World complete script/command memory/RNG differs")
				}
			}
			w.nativeCallDepth--
		})
	}
	if checked != 272 || updates != 278 {
		t.Fatalf("World scheduler coverage: %dcases/%dupdates", checked, updates)
	}
}

func scenarioTestParameters(events ...ScenarioScriptEvent) [60]byte {
	var raw [60]byte
	for i, e := range events {
		a := i * 6
		binary.BigEndian.PutUint16(raw[a:], e.Time)
		raw[a+2], raw[a+3], raw[a+4], raw[a+5] = e.Reserved, e.Command, e.X, e.Y
	}
	return raw
}

func TestWorldScenarioCursorStormAliasAndSavedContinuation(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	w.NativeGameMode = 8
	raw := scenarioTestParameters(ScenarioScriptEvent{Time: 1, Command: 64, X: 30, Y: 30}, ScenarioScriptEvent{Time: 1, Command: 22, X: 32, Y: 32})
	if err := LoadScenarioScript(raw, w.nativeCleanupMemory()); err != nil {
		t.Fatal(err)
	}
	w.NativeClock = 1
	beforeMana := w.Core.Magnets[1].Mana
	if err := w.tickNativeScenarioScript(); err != nil {
		t.Fatal(err)
	}
	cursor, _ := w.nativeCleanupMemory().Read16(0xf0a)
	firstTime, _ := w.nativeCleanupMemory().Read16(0xdde)
	if cursor != 6 || firstTime != 0 || w.Core.Magnets[1].Mana != beforeMana {
		t.Fatal("script owner/editor debit or Storm caller alias differs")
	}
	copy, err := ReadSave(b, bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.tickNativeScenarioScript(); err != nil {
		t.Fatal(err)
	}
	if err := copy.tickNativeScenarioScript(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
		t.Fatal("saved script alias/cursor continuation differs")
	}
	cursor, _ = w.nativeCleanupMemory().Read16(0xf0a)
	if cursor != 12 {
		t.Fatal("same-time second event did not wait for next call")
	}
}

func TestAllOriginalCampaignScriptCommandsHaveWorldBodies(t *testing.T) {
	b := testBundle(t)
	for _, command := range []uint8{6, 22, 40, 46, 62, 64, 90, 92, 94, 96, 98, 100} {
		w, err := NewWorld(b, 0, true)
		if err != nil {
			t.Fatal(err)
		}
		raw := scenarioTestParameters(ScenarioScriptEvent{Time: 1, Command: command, X: 32, Y: 32})
		if err := LoadScenarioScript(raw, w.nativeCleanupMemory()); err != nil {
			t.Fatal(err)
		}
		w.NativeClock = 1
		before := w.Core.Magnets
		if err := w.tickNativeScenarioScript(); err != nil {
			t.Fatalf("command%d: %v", command, err)
		}
		if w.Core.Magnets[0].Mana != before[0].Mana || w.Core.Magnets[1].Mana != before[1].Mana {
			t.Fatal("neutral event charged player mana")
		}
	}
}

func TestPartialNativeRandomAliasesReachGenerator(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	m := w.nativeCleanupMemory()
	if err := m.Write32(0xeb28, 0x12345678); err != nil {
		t.Fatal(err)
	}
	if err := m.Write16(0xeb28, 0xabcd); err != nil {
		t.Fatal(err)
	}
	if w.Core.RandomState() != 0xabcd5678 {
		t.Fatal("native wall/neutral high-word alias lost")
	}
	if err := m.Write8(0xeb2b, 0x90); err != nil {
		t.Fatal(err)
	}
	if w.Core.RandomState() != 0xabcd5690 {
		t.Fatal("native RNG byte alias lost")
	}
}
