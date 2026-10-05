package populous2

import "testing"

func TestNativeCampaignBlitterRejectsOutsidePhysicalWindow(t *testing.T) {
	var state NativeCampaignBlitterState
	state.Words[0] = 0x5a5a
	before := state
	for _, at := range []uint32{0xdfeffe, 0xdff001, 0xdff100, 0xffffffff} {
		if e := state.Write16(at, 0x1234, FollowerCleanupMemory{}); e == nil || state != before {
			t.Fatalf("invalid hardwareword%x mutated sourcewindow", at)
		}
	}
	for _, at := range []uint32{0xdfeffe, 0xdff001, 0xdff0fe, 0xdff100, 0xffffffff} {
		if e := state.Write32(at, 0x12345678, FollowerCleanupMemory{}); e == nil || state != before {
			t.Fatalf("invalid hardwarelong%x partiallymutated sourcewindow", at)
		}
	}
}
