package populous2

// NativeFollowerRegisterContext is the register continuation of $11252.
// Inactive records, pass initialization and the $12462/$1249e epilogue do not
// reset these registers. It belongs to the ordered native pass, not to a
// particular actor. The full words are retained because byte assignments
// preserve their high byte, while MOVEQ and MOVEM.W affect the entire long.
type NativeFollowerRegisterContext struct {
	D4, D5 uint32
	Frame  *NativeFrameRegisterContext
	// AddressBase is the BSS relocation base for MOVE.L An,D4. The Go
	// runtime's address convention is BSS-relative (zero); CPU fixtures may
	// supply their actual relocated base without substituting host pointers.
	AddressBase uint32
}

func NewNativeFollowerFrameContext(frame *NativeFrameRegisterContext) NativeFollowerRegisterContext {
	if frame == nil {
		return NativeFollowerRegisterContext{}
	}
	return NativeFollowerRegisterContext{D4: frame.D[4], D5: frame.D[5], AddressBase: frame.AddressBase, Frame: frame}
}

func (c *NativeFollowerRegisterContext) importFrame() {
	if c.Frame != nil {
		c.D4, c.D5 = c.Frame.D[4], c.Frame.D[5]
	}
}

func (c *NativeFollowerRegisterContext) exportFrame() {
	if c.Frame != nil {
		c.Frame.D[4], c.Frame.D[5] = c.D4, c.D5
	}
}

type nativeFollowerSavedContext struct {
	Context NativeFollowerRegisterContext
	D       [8]uint32
}

func (c *NativeFollowerRegisterContext) saveContext() nativeFollowerSavedContext {
	c.importFrame()
	s := nativeFollowerSavedContext{Context: *c}
	if c.Frame != nil {
		s.D = c.Frame.D
	}
	return s
}

func (c *NativeFollowerRegisterContext) restoreContext(saved nativeFollowerSavedContext) {
	*c = saved.Context
	if c.Frame != nil {
		c.Frame.D = saved.D
	}
}

func (c NativeFollowerRegisterContext) RecordAddress(ref NativeRecordReference) uint32 {
	return c.AddressBase + uint32(cleanupRecordAddress(ref))
}

func (c *NativeFollowerRegisterContext) Byte4(value uint8) {
	c.importFrame()
	c.D4 = c.D4&0xffffff00 | uint32(value)
	c.exportFrame()
}
func (c *NativeFollowerRegisterContext) Byte5(value uint8) {
	c.importFrame()
	c.D5 = c.D5&0xffffff00 | uint32(value)
	c.exportFrame()
}
func (c *NativeFollowerRegisterContext) Word4(value uint16) {
	c.importFrame()
	c.D4 = c.D4&0xffff0000 | uint32(value)
	c.exportFrame()
}
func (c *NativeFollowerRegisterContext) Word5(value uint16) {
	c.importFrame()
	c.D5 = c.D5&0xffff0000 | uint32(value)
	c.exportFrame()
}
func (c *NativeFollowerRegisterContext) Long4(value uint32) {
	c.importFrame()
	c.D4 = value
	c.exportFrame()
}
func (c *NativeFollowerRegisterContext) Long5(value uint32) {
	c.importFrame()
	c.D5 = value
	c.exportFrame()
}

// RestoreWord4/5 are MOVEM.W loads, which sign-extend the restored word into
// the entire data register. MOVE.W alone would leave the upper word intact.
func (c *NativeFollowerRegisterContext) RestoreWord4(value uint16) {
	c.importFrame()
	c.D4 = uint32(int32(int16(value)))
	c.exportFrame()
}
func (c *NativeFollowerRegisterContext) RestoreWord5(value uint16) {
	c.importFrame()
	c.D5 = uint32(int32(int16(value)))
	c.exportFrame()
}

func (c NativeFollowerRegisterContext) AIContext() NativeAIRegisterContext {
	if c.Frame != nil {
		return NativeAIRegisterContext{D4: uint16(c.Frame.D[4]), D5: uint16(c.Frame.D[5])}
	}
	return NativeAIRegisterContext{D4: uint16(c.D4), D5: uint16(c.D5)}
}

// BeginLeg observes $13126. The same-cell branch uses MOVEQ followed by
// fractional byte loads. The different-cell branch makes no D4/D5 writes.
// Call before BeginLeg changes the source coordinates or invokes a callback.
func (c *NativeFollowerRegisterContext) BeginLeg(source FollowerMotionActor, targetX, targetY int16) {
	if source.X>>8 == targetX>>8 && source.Y>>8 == targetY>>8 {
		c.Long4(uint32(uint8(source.X)))
		c.Long5(uint32(uint8(source.Y)))
	}
}

// BeginLegWithContext is the additive ABI adapter for existing callers. A
// speed-zero trap can occur after same-cell register assignments, so those
// assignments precede the bounded Go controller's error return too.
func (rules *FollowerMotionRules) BeginLegWithContext(actor *FollowerMotionActor, targetX, targetY int16, context *NativeFollowerRegisterContext) error {
	if actor != nil && context != nil {
		context.BeginLeg(*actor, targetX, targetY)
	}
	return rules.BeginLeg(actor, targetX, targetY)
}

// TownStage is the byte assignment at $11740 before the evaluator call.
func (c *NativeFollowerRegisterContext) TownStage(stage uint8) { c.Byte4(stage) }

// TownEvaluation observes $13352. The saved D4.W is restored on every return;
// $133c2, when actually entered, leaves property mask$17 in D5.W. Farm clear
// $135ca and reform$12bd8 save/restore both complete registers instead.
func (c *NativeFollowerRegisterContext) TownEvaluation(savedD4 uint16, recomputed bool) {
	if recomputed {
		c.Word5(0x17)
	}
	c.RestoreWord4(savedD4)
}

// EntryScan begins only after nonzero map-head admission at $12778. Captive
// and empty-head paths preserve both previous registers.
func (c *NativeFollowerRegisterContext) EntryScan() {
	c.Long4(0)
	c.Long5(0)
}

// EntryCandidate is $127dc/$127f8/$12806. nativeAddress is the actual bounded
// native address convention used by the runtime image, never a Go pointer.
func (c *NativeFollowerRegisterContext) EntryCandidate(nativeAddress uint32, priority uint16) {
	c.Word5(priority)
	c.Long4(nativeAddress)
}

// EntryDispatch replaces priority with the original relative jump-table
// offset before homing/contact. Friendly town merge occurs before this load.
func (c *NativeFollowerRegisterContext) EntryDispatch(priority uint16) {
	switch priority {
	case 2:
		c.Word5(0x1a)
	case 4:
		c.Word5(0x64)
	case 6:
		c.Word5(0x0a)
	}
}

// HeroTarget begins $14414's selection. Only qualifying preferred targets
// update the best distance; fallback/captive/claimed records do not.
func (c *NativeFollowerRegisterContext) HeroTarget(sourceY uint8) {
	c.Byte4(sourceY)
	c.Word5(0x7fff)
}
func (c *NativeFollowerRegisterContext) HeroTargetDistance(distance uint16) {
	c.Word5(distance)
}

// HeroFallback is entered at $145cc after a nonzero initial probe. Reverse
// words remain in D4/D5 even if every alternative fails. Each actual alternate
// probe at$14600 restores their MOVEM.W values with sign extension.
func (c *NativeFollowerRegisterContext) HeroFallback(dx, dy int16) {
	c.Word4(uint16(-dx))
	c.Word5(uint16(-dy))
}
func (c *NativeFollowerRegisterContext) HeroAlternativeRestore() {
	c.importFrame()
	c.RestoreWord4(uint16(c.D4))
	c.RestoreWord5(uint16(c.D5))
}
