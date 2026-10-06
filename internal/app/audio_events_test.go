package app

import "testing"

func TestAnimationSoundGateAdmitsEachCueOncePerSimulationPass(t *testing.T) {
	var gate AnimationSoundGate
	for repeat := 0; repeat < 4; repeat++ {
		got := gate.Admit(17, 78)
		if got != (repeat == 0) {
			t.Fatal("display redraw restarted a simulation sound")
		}
	}
	if !gate.Admit(17, 79) || gate.Admit(17, 79) {
		t.Fatal("distinct cues were not independently admitted")
	}
	if !gate.Admit(18, 78) || !gate.Admit(18, 79) {
		t.Fatal("next simulation pass did not admit its cues")
	}
	if !gate.Admit(0, 78) {
		t.Fatal("loading an earlier saved tick retained old sound admissions")
	}
	if gate.Admit(0, 0) || gate.Admit(0, 133) || gate.Admit(0, -1) {
		t.Fatal("empty or absent cue was admitted")
	}
}
