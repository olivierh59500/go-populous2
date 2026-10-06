package populous2

import "testing"

func TestNativeHostMouseFeedsOriginalWrappingCountersAndClickPosition(t *testing.T) {
	input, err := NewNativeInputState(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	mouse := NativeHostMouse{CounterX: 250, CounterY: 248}
	if err := input.ResetMouseCounters(NativeMouseSample{CounterX: 250, CounterY: 248}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		sample := mouse.Sample(&input, 319, 199, true, false)
		if sample.Left && i < 6 {
			t.Fatal("click latched before cursor reached its target")
		}
		if _, err := input.PollMouse(sample, 0, 0x400000, nil); err != nil {
			t.Fatal(err)
		}
	}
	if input.word(0x138) != 319 || input.word(0x13a) != 199 || input.word(0x140) != 1 || input.word(0x134) != 319 || input.word(0x136) != 199 {
		t.Fatal("native click/position differs after wrapping counters")
	}
	for i := 0; i < 8; i++ {
		sample := mouse.Sample(&input, 0, 0, false, false)
		if _, err := input.PollMouse(sample, 0, 0x400000, nil); err != nil {
			t.Fatal(err)
		}
	}
	if input.word(0x138) != 0 || input.word(0x13a) != 0 || input.word(0x144) != 0 {
		t.Fatal("native cursor return/button release failed")
	}
}
