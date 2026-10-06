package populous2

// NativeHostMouse converts a modern absolute cursor into wrapping JOY0DAT
// counters. Samples approach large cursor jumps across successive VBlanks so
// each signed native delta remains representable. NativeInputState performs
// all actual position, button-latch and pointer mutations.
type NativeHostMouse struct{ CounterX, CounterY uint8 }

func (s *NativeHostMouse) Sample(input *NativeInputState, x, y int, left, right bool) NativeMouseSample {
	if input == nil {
		return NativeMouseSample{CounterX: s.CounterX, CounterY: s.CounterY, Left: left, Right: right}
	}
	goalX := max(0, min(639, x*2))
	goalY := max(0, min(int(input.Mouse.MaximumY), y*2))
	dx := max(-100, min(100, goalX-int(input.Mouse.PositionX)))
	dy := max(-100, min(100, goalY-int(input.Mouse.PositionY)))
	s.CounterX = uint8(int(s.CounterX) + dx)
	s.CounterY = uint8(int(s.CounterY) + dy)
	// A click belongs to its actual position, after counters reach the
	// requested cursor. This avoids latching the intermediate jump point.
	arrived := int(input.Mouse.PositionX)+dx == goalX && int(input.Mouse.PositionY)+dy == goalY
	return NativeMouseSample{CounterX: s.CounterX, CounterY: s.CounterY, Left: left && arrived, Right: right && arrived}
}
