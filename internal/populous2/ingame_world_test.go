package populous2

import "testing"

func TestNativeInGameWorldActionsPreserveDeferredCommands(t *testing.T) {
	b := testBundle(t)
	rules, err := DecodeNativeInGameRequesterRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := nativeInGameFixtures(t)
	if len(fixtures.MenuActions) != 2808 {
		t.Fatal("original in-game action corpus incomplete")
	}
	for index, fixture := range fixtures.MenuActions {
		w, err := NewWorld(b, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		m := w.nativeCleanupMemory()
		w.NativeProfileSide = uint8(fixture.Input.Profile)
		w.NativeGameMode = fixture.Input.GameMode
		god, _ := NativeDeityAddress(w.NativeProfileSide)
		for _, field := range []struct {
			Address int
			Value   uint16
		}{
			{0xeb42, fixture.Input.Profile}, {0xeb44, fixture.Input.GameMode},
			{god + 0x1a, fixture.Input.ControlMode}, {0xf0e, fixture.Input.PaintFlag},
			{0xeb58, 0xabcd},
		} {
			if err := m.Write16(field.Address, field.Value); err != nil {
				t.Fatal(err)
			}
		}
		if err := m.Write32(0xeb6a, 0xeb56); err != nil {
			t.Fatal(err)
		}
		clock, turn := w.NativeClock, w.Core.GameTurn
		context := NativeCommandRegisterContext{}
		step, err := w.ApplyNativeInGameMenu(rules, fixture.Action, &context)
		if err != nil {
			t.Fatal(err)
		}
		state, err := w.NativeInGameMenuState()
		if err != nil {
			t.Fatal(err)
		}
		profile := fixture.Input.Profile
		if fixture.SwitchProfile != 0 {
			profile = fixture.SwitchProfile
		}
		queued, _ := m.Read8(0xeb57)
		xy, _ := m.Read16(0xeb58)
		if state.Profile != profile || state.ControlMode != fixture.ControlMode || state.PaintFlag != fixture.PaintFlag || queued != fixture.DeferredCommand || xy != 0xabcd {
			t.Fatalf("native in-game World action%d fixture%d differs: state%+v queued%d xy%x", fixture.Action, index, state, queued, xy)
		}
		if step.Close != (fixture.Stop == "resume-check") || step.SwitchProfile != fixture.SwitchProfile || step.DeferredCommand != fixture.DeferredCommand {
			t.Fatalf("native in-game continuation fixture%d differs", index)
		}
		if w.NativeClock != clock || w.Core.GameTurn != turn {
			t.Fatal("menu executed simulation before its deferred command stage")
		}
	}
}

func TestNativeInGameResumeRequiresActualMultiplayerPort(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	context := NativeCommandRegisterContext{}
	m := w.nativeCleanupMemory()
	if err := m.Write8(0xeb57, 108); err != nil {
		t.Fatal(err)
	}
	if err := m.Write16(0xeb58, 0xabcd); err != nil {
		t.Fatal(err)
	}
	step, err := w.ResumeNativeInGameMenu(&NativeSerialResume{}, NativeSerialPort{}, &context)
	if err != nil || !step.Complete || step.Waiting {
		t.Fatal("actual solo resume failed", err)
	}
	queued, _ := m.Read8(0xeb57)
	xy, _ := m.Read16(0xeb58)
	if queued != 108 || xy != 0xabcd {
		t.Fatal("resume gate executed or cleared a pending command")
	}
	if err := m.Write8(0xeb5e, 6); err != nil {
		t.Fatal(err)
	}
	if step, err := w.ResumeNativeInGameMenu(&NativeSerialResume{}, NativeSerialPort{}, &context); err == nil || step.Complete {
		t.Fatal("multiplayer resume fabricated a successful exchange")
	}
}
