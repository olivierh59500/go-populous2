package populous2

import (
	"encoding/binary"
	"fmt"
)

// NativeSessionRenderBindings supply the prepared resources and genuine
// modal children for one landscape. They are built once by the host; drawing
// borrows the original chip buffers without allocating replacement bitmaps.
type NativeSessionRenderBindings struct {
	Rules        *NativeActorRenderRules
	Sprites      *NativeSpriteBitmapBank
	Tiles        *NativeTileBitmapBank
	Selected     NativeRenderFrameChildren
	Children     NativeActorRenderChildren
	DebugOverlay func(*NativeFrameRegisterContext) error
	Code         FollowerCleanupMemory
	// Window optionally resolves actual RAM surrounding an external bitmap.
	// The default uses the presentation's real HUNK4 allocation.
	Window func(uint32) (NativeBitmapWindow, error)
}

// RenderMain supplies the concrete synchronous renderer to Advance. Selected
// and town modals must complete in their child callbacks; a pending modal
// requires a separate resumable renderer and cannot be reported as success.
func (s *NativeFrameSession) RenderMain(bindings NativeSessionRenderBindings, state *NativeMainRenderState) func(FollowerCleanupMemory, *NativeFrameRegisterContext, *NativeImageRenderState, []byte, *uint32) (bool, error) {
	return func(memory FollowerCleanupMemory, frame *NativeFrameRegisterContext, image *NativeImageRenderState, bitmap []byte, phase *uint32) (bool, error) {
		if s == nil || s.Presentation == nil || state == nil || bindings.Rules == nil || bindings.Sprites == nil || bindings.Tiles == nil || phase == nil || frame == nil || image == nil || !winMemoryValid(memory) || len(bitmap) != 32000 {
			return false, fmt.Errorf("native session main rendering backing missing")
		}
		if *phase != 0 {
			return false, fmt.Errorf("native synchronous main renderer cannot resume a partial frame")
		}
		if err := state.Begin(); err != nil {
			return false, err
		}
		address, err := memory.Read32(0x1e)
		if err != nil {
			return false, err
		}
		var window NativeBitmapWindow
		if bindings.Window != nil {
			window, err = bindings.Window(address)
		} else {
			var at int
			at, err = s.Presentation.chipAt(address, 32000)
			window = NativeBitmapWindow{Bytes: s.Presentation.Chip, BitmapOffset: at}
		}
		if err != nil {
			return false, err
		}
		if window.BitmapOffset < 0 || window.BitmapOffset > len(window.Bytes)-32000 || &window.Bytes[window.BitmapOffset] != &bitmap[0] {
			return false, fmt.Errorf("native bitmap window does not own the drawing target")
		}
		backgroundAddress, err := memory.Read32(0x22)
		if err != nil {
			return false, err
		}
		background, err := s.bitmapAt(backgroundAddress)
		if err != nil {
			return false, err
		}
		if len(background) != 32000 {
			return false, fmt.Errorf("native persistent background bitmap missing")
		}
		effects := NativeActorEffectsCallbacks{
			NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: memory, Frame: frame, Image: image, Input: &s.Presentation.Input, Bitmap: bitmap, Sprite: bindings.Sprites.Paint},
			Cropped:                    bindings.Sprites.PaintCropped, Reinterpreted: bindings.Sprites.PaintReinterpreted,
		}
		world := NativeWorldRenderCallbacks{Effects: effects, Tiles: bindings.Tiles, Background: background, Children: bindings.Children,
			Tile: func(request NativeTileChunkRequest, _ []byte) error {
				return bindings.Tiles.PaintChunkWindow(request, window)
			},
		}
		// Mark the operation before the first mutation. A source failure retains
		// its prefix, and the session closes rather than replaying the renderer.
		*phase = 1
		code := bindings.Code
		if code.Write32 == nil && s.world != nil {
			code.Write32 = func(at int, value uint32) error {
				if at < 0 || at&1 != 0 || at > len(s.world.NativeAI.Code)-4 {
					return fmt.Errorf("native session CODE long%x unavailable", at)
				}
				binary.BigEndian.PutUint32(s.world.NativeAI.Code[at:], value)
				return nil
			}
		}
		if err := bindings.Rules.MainFrame(NativeMainRenderCallbacks{World: world, Selected: bindings.Selected, Code: code, DebugOverlay: bindings.DebugOverlay}, state); err != nil {
			return false, err
		}
		*phase = 2
		return true, nil
	}
}
