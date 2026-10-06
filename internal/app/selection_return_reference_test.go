package app

import (
	"encoding/binary"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
)

func TestPrivateSelectionReturnMatchesOriginalMainFrames(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original asset directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeActorRenderRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := populous2.DecodeNativeSpriteBitmapBank(source, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                                      string
		current, backup, frames, target, removeAt int
	}{
		{"first temporary inspection", 1, 0, 0, 2, 0},
		{"repeated temporary inspection", 1, 3, 20, 2, 0},
		{"empty with pending return", 0, 3, 2, 2, 0},
		{"temporary actor removed", 1, 3, 20, 2, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := selectionReturnGame()
			g.SelectedFollower = tc.current
			g.SelectionReturn = FollowerSelectionReturn{BackupFollower: tc.backup, FramesLeft: uint16(tc.frames)}
			raw := make([]byte, 0x11280)
			const base = 0x200000
			pointer := func(id int) uint32 {
				if id == 0 {
					return 0
				}
				return base + 0x76c0 + uint32(id*52)
			}
			selectedID := func() int {
				p := binary.BigEndian.Uint32(raw[0xf36:])
				if p == 0 {
					return 0
				}
				return int(p-base-0x76c0) / 52
			}
			for id := 1; id <= 3; id++ {
				at := 0x76c0 + id*52
				raw[at], raw[at+12], raw[at+22] = 2, 1, 4
				binary.BigEndian.PutUint16(raw[at+16:], 0xffec)
			}
			binary.BigEndian.PutUint16(raw[0xf30:], uint16(tc.frames))
			binary.BigEndian.PutUint32(raw[0xf32:], pointer(tc.backup))
			binary.BigEndian.PutUint32(raw[0xf36:], pointer(tc.current))
			memory := selectedPanelReferenceMemory(raw)
			frame := populous2.NativeFrameRegisterContext{AddressBase: base}
			var addresses [7]populous2.NativeRequesterAddress
			addresses[1] = populous2.NativeRequesterAddress{Address: pointer(tc.target)}
			_, err := populous2.RunNativeGameplayEditorInput(0x29d2, populous2.NativeGameplayEditorInputCallbacks{NativeStartupResetFrameCallbacks: populous2.NativeStartupResetFrameCallbacks{Memory: memory, Code: memory, RAM: memory, Frame: &frame}}, &addresses)
			if err != nil {
				t.Fatal(err)
			}
			g.selectFollowerTemporarily(tc.target)
			check := func(main int) {
				backup := binary.BigEndian.Uint32(raw[0xf32:])
				backupID := 0
				if backup != 0 {
					backupID = int(backup-base-0x76c0) / 52
				}
				if g.SelectedFollower != selectedID() || g.SelectionReturn.FramesLeft != binary.BigEndian.Uint16(raw[0xf30:]) || g.SelectionReturn.BackupFollower != backupID {
					t.Fatalf("main frame %d: Go current %d backup %d remaining %d; source current %d backup %d remaining %d", main, g.SelectedFollower, g.SelectionReturn.BackupFollower, g.SelectionReturn.FramesLeft, selectedID(), backupID, binary.BigEndian.Uint16(raw[0xf30:]))
				}
			}
			check(0)
			imageState := rules.Frames.Images.NewImageState()
			bitmap := make([]byte, 32000)
			cb := populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Bitmap: bitmap, Sprite: bank.Paint}
			for main := 1; main <= 101; main++ {
				if main == tc.removeAt {
					raw[0x76c0+tc.target*52+12] = 0
					g.World.Followers[tc.target] = engine.Follower{}
				}
				_, err := rules.Frames.Selected(cb, populous2.NativeRenderFrameChildren{DrawActor: func(at int, _ *populous2.NativeFrameRegisterContext) error {
					_, e := rules.Actor(at, populous2.NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: cb}, &populous2.NativeActorRenderState{}, populous2.NativeActorRenderChildren{})
					return e
				}})
				if err != nil {
					t.Fatal(err)
				}
				g.advanceSelectionPresentation()
				check(main)
			}
		})
	}
}
