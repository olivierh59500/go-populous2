package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeProfilePanelFrameRules struct {
	Render NativeRenderFrameRules
	Icons  map[int]NativePreparedSprite
}

func DecodeNativeProfilePanelFrameRules(exe *amiga.Executable) (NativeProfilePanelFrameRules, error) {
	var r NativeProfilePanelFrameRules
	var err error
	r.Render, err = DecodeNativeRenderFrameRules(exe)
	if err != nil {
		return r, err
	}
	if len(exe.Hunks) < 4 {
		return r, fmt.Errorf("native panel icon resource missing")
	}
	r.Icons = make(map[int]NativePreparedSprite)
	code := exe.Hunks[0].Data
	for descriptor := 0x214b2; descriptor < 0x2161a; descriptor += 12 {
		offset := int(binary.BigEndian.Uint32(code[descriptor:]))
		width := int(binary.BigEndian.Uint16(code[descriptor+4:])) * 2
		height := int(binary.BigEndian.Uint16(code[descriptor+6:]))
		routine := binary.BigEndian.Uint32(code[descriptor+8:])
		length := width / 8 * 5 * height
		if width != 32 || height <= 0 || routine != 0xf3a0 || offset < 0 || length > len(exe.Hunks[3].Data)-offset {
			return r, fmt.Errorf("native panel descriptor %#x unavailable", descriptor)
		}
		planes, err := PrepareNativeMaskedPlanes(exe.Hunks[3].Data[offset:offset+length], width, height)
		if err != nil {
			return r, err
		}
		r.Icons[descriptor] = NativePreparedSprite{Width: width, Height: height, Planes: planes}
	}
	return r, nil
}

type NativeProfileSwitchFramePlan struct {
	DestinationDeity, SourceDeity     int
	DestinationCommand, SourceCommand int
	A0, A1                            uint32 // Actual surviving command pointers, including signed aliases.
}

// SwitchProfile is complete $111ae. It swaps the actual eight-byte deity
// slice and two identity/control words, then the original command word unless
// the destination transport is6/8. D3 survives its MOVEM.L unchanged.
func (r *NativeProfilePanelFrameRules) SwitchProfile(m FollowerCleanupMemory, c *NativeFrameRegisterContext) (NativeProfileSwitchFramePlan, error) {
	var p NativeProfileSwitchFramePlan
	if r == nil || c == nil || !winMemoryValid(m) {
		return p, fmt.Errorf("native profile switch backing missing")
	}
	saved := c.D[3]
	defer func() { c.D[3] = saved }()
	old, err := m.Read16(0xeb42)
	if err != nil {
		return p, err
	}
	c.Word(1, old)
	c.Word(2, uint16(c.D[0]))
	c.D[2] = uint32(uint16(c.D[2])) * 314
	p.DestinationDeity = 0xe76a + int(int16(c.D[2]))
	c.Word(2, uint16(c.D[1]))
	c.D[2] = uint32(uint16(c.D[2])) * 314
	p.SourceDeity = 0xe76a + int(int16(c.D[2]))
	c.Word(3, 7)
	for index := 0; index < 8; index++ {
		value, err := m.Read8(p.DestinationDeity + 0x52 + index)
		if err != nil {
			return p, err
		}
		c.Byte(2, value)
		other, err := m.Read8(p.SourceDeity + 0x52 + index)
		if err != nil {
			return p, err
		}
		if err := m.Write8(p.DestinationDeity+0x52+index, other); err != nil {
			return p, err
		}
		if err := m.Write8(p.SourceDeity+0x52+index, uint8(c.D[2])); err != nil {
			return p, err
		}
		c.Word(3, uint16(c.D[3])-1)
	}
	for _, offset := range []int{0x1a, 0x18} {
		value, err := m.Read16(p.DestinationDeity + offset)
		if err != nil {
			return p, err
		}
		c.Word(2, value)
		other, err := m.Read16(p.SourceDeity + offset)
		if err != nil {
			return p, err
		}
		if err := m.Write16(p.DestinationDeity+offset, other); err != nil {
			return p, err
		}
		if err := m.Write16(p.SourceDeity+offset, uint16(c.D[2])); err != nil {
			return p, err
		}
	}
	c.Word(2, uint16(c.D[0]))
	c.D[2] = uint32(uint16(c.D[2])) * 10
	p.DestinationCommand = 0xeb4c + int(int16(c.D[2]))
	p.A0 = c.AddressBase + uint32(p.DestinationCommand)
	if err := m.Write32(0xeb6a, p.A0); err != nil {
		return p, err
	}
	c.Word(2, uint16(c.D[1]))
	c.D[2] = uint32(uint16(c.D[2])) * 10
	p.SourceCommand = 0xeb4c + int(int16(c.D[2]))
	p.A1 = c.AddressBase + uint32(p.SourceCommand)
	mode, err := m.Read16(p.DestinationCommand + 8)
	if err != nil {
		return p, err
	}
	c.Word(2, mode)
	if mode != 6 && mode != 8 {
		other, err := m.Read16(p.SourceCommand + 8)
		if err != nil {
			return p, err
		}
		if err := m.Write16(p.DestinationCommand+8, other); err != nil {
			return p, err
		}
		if err := m.Write16(p.SourceCommand+8, uint16(c.D[2])); err != nil {
			return p, err
		}
	}
	return p, m.Write16(0xeb42, uint16(c.D[0]))
}

type NativeProfilePanelFrameCallbacks struct {
	Memory    FollowerCleanupMemory
	Frame     *NativeFrameRegisterContext
	Image     *NativeImageRenderState
	Sprite    func(NativePresentationSprite, []byte) error
	Bitmap    func(uint32) ([]byte, error)
	Ownership func(bool, *NativeFrameRegisterContext) error // Original$e28/$e4c handoff.
}

type NativeProfilePanelFramePlan struct {
	Descriptors []int
	Sprites     []NativePresentationSprite
	HUD         NativeRenderFramePlan
}

// RestorePanel is complete $1da0: its five prepared icon blits target actual
// BSS$22, while the genuine following$1f5e HUD draws into BSS$1e. There are
// no resource/palette/audio loads or hidden typed profile normalization.
func (r *NativeProfilePanelFrameRules) RestorePanel(cb NativeProfilePanelFrameCallbacks) (NativeProfilePanelFramePlan, error) {
	var p NativeProfilePanelFramePlan
	if r == nil || cb.Frame == nil || cb.Image == nil || cb.Sprite == nil || cb.Bitmap == nil || cb.Ownership == nil || !winMemoryValid(cb.Memory) {
		return p, fmt.Errorf("native profile panel backing/handoff missing")
	}
	c, m := cb.Frame, cb.Memory
	owner, err := m.Read16(0xeb42)
	if err != nil {
		return p, err
	}
	c.Word(0, owner)
	c.D[0] = uint32(uint16(c.D[0])) * 314
	god := 0xe76a + int(int16(c.D[0]))
	category, err := m.Read16(0xf3a)
	if err != nil {
		return p, err
	}
	c.Word(0, category)
	c.D[0] = uint32(uint16(c.D[0])) * 3
	flags := god + 0x70 + int(int16(c.D[0]))
	c.Word(0, uint16(c.D[0])*2)
	table := 0x21102 + int(int16(c.D[0]))
	c.D[3] = 4
	for index := 0; index < 5; index++ {
		x, err := r.Render.word(0x2114a + index*4)
		if err != nil {
			return p, err
		}
		y, err := r.Render.word(0x2114c + index*4)
		if err != nil {
			return p, err
		}
		c.Word(0, x)
		c.Word(1, y)
		offset, err := r.Render.word(table + index*2)
		if err != nil {
			return p, err
		}
		c.Word(2, offset)
		flag, err := m.Read8(flags + index)
		if err != nil {
			return p, err
		}
		descriptor := 0x214b2
		if int8(flag) > 0 {
			descriptor += int(int16(c.D[2]))
		}
		icon, ok := r.Icons[descriptor]
		if !ok {
			return p, fmt.Errorf("native panel descriptor %#x missing", descriptor)
		}
		target, err := m.Read32(0x22)
		if err != nil {
			return p, err
		}
		bitmap, err := cb.Bitmap(target)
		if err != nil {
			return p, err
		}
		height, err := r.Render.word(descriptor + 6)
		if err != nil {
			return p, err
		}
		c.Word(2, height)
		sprite := NativePresentationSprite{X: int16(c.D[0]), Y: int16(c.D[1]), HalfWidth: 16, Height: int16(height), Routine: 0xf3a0}
		p.Descriptors = append(p.Descriptors, descriptor)
		p.Sprites = append(p.Sprites, sprite)
		saved3, ownershipD := c.D[3], c.D
		if err := cb.Ownership(true, c); err != nil {
			return p, err
		}
		c.D = ownershipD
		if err := r.Render.primitiveRegisters(0xf3a0, c); err != nil {
			return p, err
		}
		if err := icon.paintRows(sprite, bitmap, int(height)); err != nil {
			return p, err
		}
		ownershipD = c.D
		if err := cb.Ownership(false, c); err != nil {
			return p, err
		}
		c.D = ownershipD
		c.D[3] = saved3
		c.Word(3, uint16(c.D[3])-1)
	}
	target, err := m.Read32(0x1e)
	if err != nil {
		return p, err
	}
	bitmap, err := cb.Bitmap(target)
	if err != nil {
		return p, err
	}
	p.HUD, err = r.Render.HUD(NativeRenderFrameCallbacks{Memory: m, Frame: c, Image: cb.Image, Sprite: cb.Sprite, Bitmap: bitmap}, true)
	return p, err
}
