package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativePresentationContextRules owns the post-HUD selected-follower pane.
// Raw low-memory pointers and countdowns remain caller-owned; an absolute
// pointer uses AddressBase, while the Go runtime's BSS-relative convention is0.
type NativePresentationContextRules struct {
	code                         []byte
	HUD                          NativeHUDRules
	Walking                      NativeWallRules
	Town                         NativeTownCenterArt
	SelectionRect                [4]uint16
	FollowerAnchor, WeaponAnchor [2]int16
	DigitAnchors                 [8][2]int16
}
type NativePresentationContextCallbacks struct {
	Memory      FollowerCleanupMemory
	AddressBase uint32
	EditOverlay func(*NativeHUDRegisters) error // Original$346a, only on actual edit flag.
	SelectHit   func(*NativeHUDRegisters) error // Original$2914, only on actual cursor-hit branch.
}
type NativePresentationSprite struct {
	Sprite                  int
	X, Y, HalfWidth, Height int16
	Routine                 uint32
}
type NativeSelectedPresentation struct {
	SelectedAddress uint32
	Rendered        bool
	Sprites         []NativePresentationSprite
}

func DecodeNativePresentationContextRules(exe *amiga.Executable) (NativePresentationContextRules, error) {
	var r NativePresentationContextRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x26956 {
		return r, fmt.Errorf("native selected presentation tables missing")
	}
	r.code = exe.Hunks[0].Data
	var err error
	r.HUD, err = DecodeNativeHUDRules(exe)
	if err != nil {
		return r, err
	}
	r.Walking, err = DecodeNativeWallRules(exe)
	if err != nil {
		return r, err
	}
	r.Town, err = DecodeNativeTownCenterArt(exe)
	if err != nil {
		return r, err
	}
	for i := range r.SelectionRect {
		r.SelectionRect[i] = binary.BigEndian.Uint16(r.code[0x21292+i*2:])
	}
	for i := range 2 {
		r.FollowerAnchor[i] = int16(binary.BigEndian.Uint16(r.code[0x2128a+i*2:]))
		r.WeaponAnchor[i] = int16(binary.BigEndian.Uint16(r.code[0x2128e+i*2:]))
	}
	for i := range r.DigitAnchors {
		r.DigitAnchors[i] = [2]int16{int16(binary.BigEndian.Uint16(r.code[0x2129a+i*4:])), int16(binary.BigEndian.Uint16(r.code[0x2129c+i*4:]))}
	}
	return r, nil
}

func (r *NativePresentationContextRules) word(at int) (uint16, error) {
	if r == nil || at < 0 || at&1 != 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native presentation word outside retained CODE")
	}
	return binary.BigEndian.Uint16(r.code[at:]), nil
}
func (r *NativePresentationContextRules) sprites(layers []SpriteLayer, x, y int16, c *NativeHUDRegisters) ([]NativePresentationSprite, error) {
	result := []NativePresentationSprite{}
	for _, layer := range layers {
		a := 0x21626 + layer.Sprite*12
		if a < 0 || a+12 > len(r.code) {
			return nil, fmt.Errorf("native presentation descriptor outside CODE")
		}
		half, height := int16(binary.BigEndian.Uint16(r.code[a+4:])), int16(binary.BigEndian.Uint16(r.code[a+6:]))
		procedure := binary.BigEndian.Uint32(r.code[a+8:])
		if procedure != 0xf0ee && procedure != 0xf3a0 {
			return nil, fmt.Errorf("native presentation sprite register body$%x needs its own continuation", procedure)
		}
		p := NativePresentationSprite{Sprite: layer.Sprite, X: int16(uint16(x) + uint16(int16(layer.X)) - uint16(half)), Y: int16(uint16(y) + uint16(int16(layer.Y)) - uint16(height)), HalfWidth: half, Height: height, Routine: procedure}
		result = append(result, p)
		// Both supported original primitives assign the source-plane stride
		// to D7.W before any clipping. They preserve D4/D5, including when
		// a sprite is entirely outside its target buffer.
		c.D7 = hudWord(c.D7, uint16(height*half/4))
	}
	return result, nil
}
func (r *NativePresentationContextRules) image(offset uint16) ([]SpriteLayer, error) {
	word, err := r.word(0x23d1a + int(int16(offset)))
	if err != nil {
		return nil, err
	}
	if int16(word) < 0 {
		return nil, fmt.Errorf("native presentation image is an animation marker")
	}
	return decodeImageLayers(r.code, word)
}

// Selected translates$1e18..$1f5c, retaining the actual no-selection/dead-owner
// and delayed-selection transitions. Cursor selection and edit overlays are
// explicit original callback boundaries, not silent no-op or zero-state paths.
func (r *NativePresentationContextRules) Selected(cb NativePresentationContextCallbacks, c *NativeHUDRegisters) (NativeSelectedPresentation, error) {
	plan := NativeSelectedPresentation{Sprites: []NativePresentationSprite{}}
	if r == nil || c == nil || !winMemoryValid(cb.Memory) {
		return plan, fmt.Errorf("native selected presentation memory/context missing")
	}
	m := cb.Memory
	edit, err := m.Read16(0xf0e)
	if err != nil {
		return plan, err
	}
	if edit != 0 {
		if cb.EditOverlay == nil {
			return plan, fmt.Errorf("native edit overlay continuation missing")
		}
		return plan, cb.EditOverlay(c)
	}
	hit, err := m.Read16(0x146)
	if err != nil {
		return plan, err
	}
	if hit != 0 {
		x, err := m.Read16(0x134)
		if err != nil {
			return plan, err
		}
		y, err := m.Read16(0x136)
		if err != nil {
			return plan, err
		}
		dx, dy := uint16(x-r.SelectionRect[0]), uint16(y-r.SelectionRect[1])
		if int16(dx) >= 0 && int16(dy) >= 0 && int16(uint16(dx-r.SelectionRect[2])) < 0 && int16(uint16(dy-r.SelectionRect[3])) < 0 {
			if err := m.Write16(0x142, 0); err != nil {
				return plan, err
			}
			if cb.SelectHit == nil {
				return plan, fmt.Errorf("native selected-hit continuation missing")
			}
			if err := cb.SelectHit(c); err != nil {
				return plan, err
			}
		}
	}
	timer, err := m.Read16(0xf30)
	if err != nil {
		return plan, err
	}
	backup := func() error {
		p, err := m.Read32(0xf32)
		if err != nil {
			return err
		}
		return m.Write32(0xf36, p)
	}
	if timer != 0 {
		timer--
		if err := m.Write16(0xf30, timer); err != nil {
			return plan, err
		}
		if timer == 0 {
			if err := backup(); err != nil {
				return plan, err
			}
		}
	}
	var selected uint32
	for attempts := 0; attempts < 2; attempts++ {
		selected, err = m.Read32(0xf36)
		if err != nil {
			return plan, err
		}
		if selected == 0 {
			return plan, nil
		}
		address := int(int64(selected) - int64(cb.AddressBase))
		owner, err := m.Read8(address + 12)
		if err != nil {
			return plan, err
		}
		if owner != 0 {
			break
		}
		if err := m.Write32(0xf36, 0); err != nil {
			return plan, err
		}
		timer, err = m.Read16(0xf30)
		if err != nil {
			return plan, err
		}
		if timer == 0 {
			return plan, nil
		}
		if err := m.Write16(0xf30, 0); err != nil {
			return plan, err
		}
		if err := backup(); err != nil {
			return plan, err
		}
	}
	plan.SelectedAddress = selected
	plan.Rendered = true
	address := int(int64(selected) - int64(cb.AddressBase))
	kind, err := m.Read8(address)
	if err != nil {
		return plan, err
	}
	state, err := m.Read8(address + 22)
	if err != nil {
		return plan, err
	}
	owner, err := m.Read8(address + 12)
	if err != nil {
		return plan, err
	}
	population, err := m.Read32(address + 26)
	if err != nil {
		return plan, err
	}
	ref := NativeRecordReference(uint16(address - 0x76c0))
	var layers []SpriteLayer
	markers := true
	if kind == 4 && state == 6 {
		stage, err := m.Read8(address + 1)
		if err != nil {
			return plan, err
		}
		clock, err := m.Read16(0xf42)
		if err != nil {
			return plan, err
		}
		frame, ok := r.Town.Frame(int(stage), owner, population, clock)
		if !ok {
			return plan, fmt.Errorf("native selected town stage outside decoded artwork")
		}
		layers = frame.Layers
	} else if kind <= 18 {
		target, e := r.word(0xe4ba + int(state))
		if e != nil {
			return plan, e
		}
		target += 0xe4ba
		switch target {
		case 0xe624:
			frame, _, e := r.Walking.Frame(ref, m)
			if e != nil {
				return plan, e
			}
			layers = frame.Layers
		case 0xe506: // Receiving state draws no body for nontowns.
		case 0xe5ee, 0xe8e2:
			animation, e := m.Read16(address + 10)
			if e != nil {
				return plan, e
			}
			layers, e = r.image(animation)
			if e != nil {
				return plan, e
			}
			if target == 0xe8e2 {
				markers = false
				for i := range layers {
					layers[i].Y += 8
				}
			}
		default:
			banks := map[uint16]int{0xe514: 0x209a0, 0xe51e: 0x208c0, 0xe528: 0x20880, 0xe532: 0x209c0, 0xe556: 0x208a0, 0xe560: 0x208e0, 0xe56a: 0x20900, 0xe574: 0x20920, 0xe57e: 0x20940, 0xe588: 0x20980, 0xe592: 0x20960}
			bank, ok := banks[target]
			if !ok {
				return plan, fmt.Errorf("native selected body dispatcher$%x unsupported", target)
			}
			animation, e := m.Read16(address + 10)
			if e != nil {
				return plan, e
			}
			flags, e := m.Read8(address + 13)
			if e != nil {
				return plan, e
			}
			if flags&2 != 0 {
				layers, e = r.image(animation)
			} else {
				variant, x := m.Read16(address + 50)
				if x != nil {
					return plan, x
				}
				if owner != 1 {
					bank += 16
				}
				base, x := r.word(bank + int(int16(variant)))
				if x != nil {
					return plan, x
				}
				layers, e = r.image(base + animation)
			}
			if e != nil {
				return plan, e
			}
		}
	} else {
		return plan, fmt.Errorf("native selected kind$%x/state$%x requires its own draw dispatcher", kind, state)
	}
	pose, err := r.sprites(layers, r.FollowerAnchor[0], r.FollowerAnchor[1], c)
	if err != nil {
		return plan, err
	}
	flags, err := m.Read8(address + 13)
	if err != nil {
		return plan, err
	}
	appendMarkers := func(x, y int16) {
		for _, bit := range []uint8{1, 2} {
			if flags&bit != 0 {
				at := 0x21a82
				if owner != 1 {
					at = 0x21a8e
				}
				marker := NativePresentationSprite{Sprite: (at - 0x21626) / 12, X: x - 4, Y: y - 8, HalfWidth: int16(binary.BigEndian.Uint16(r.code[at+4:])), Height: int16(binary.BigEndian.Uint16(r.code[at+6:])), Routine: binary.BigEndian.Uint32(r.code[at+8:])}
				// $eee2 saves/restores all registers around this direct descriptor.
				plan.Sprites = append(plan.Sprites, marker)
			}
		}
	}
	if kind == 4 && state == 6 {
		stage, _ := m.Read8(address + 1)
		for i, p := range pose {
			if r.Town.Frames[stage].Layers[i].Sprite == 89 && i > 0 {
				appendMarkers(int16(uint16(r.FollowerAnchor[0])+uint16(int16(r.Town.Frames[stage].Layers[i].X))), pose[i-1].Y)
			}
			plan.Sprites = append(plan.Sprites, p)
		}
	} else {
		plan.Sprites = append(plan.Sprites, pose...)
		if markers && len(pose) > 0 {
			appendMarkers(r.FollowerAnchor[0], pose[len(pose)-1].Y)
		}
	}
	weapon, err := m.Read8(address + 25)
	if err != nil {
		return plan, err
	}
	base, err := r.word(0x20b60 + int(weapon)*2)
	if err != nil {
		return plan, err
	}
	layers, err = r.image(base + 0x140)
	if err != nil {
		return plan, err
	}
	weaponPlan, err := r.sprites(layers, r.WeaponAnchor[0], r.WeaponAnchor[1], c)
	if err != nil {
		return plan, err
	}
	plan.Sprites = append(plan.Sprites, weaponPlan...)
	for digit, remaining := 0, population; remaining != 0 && digit < len(r.DigitAnchors); digit++ {
		at := 0x21626 + 0x84c
		half, height := int16(binary.BigEndian.Uint16(r.code[at+4:])), int16(binary.BigEndian.Uint16(r.code[at+6:]))
		procedure := binary.BigEndian.Uint32(r.code[at+8:])
		if procedure != 0xf0ee && procedure != 0xf3a0 {
			return plan, fmt.Errorf("native population digit primitive unsupported")
		}
		x, y := r.DigitAnchors[digit][0], int16(uint16(r.DigitAnchors[digit][1])-uint16(remaining&15))
		plan.Sprites = append(plan.Sprites, NativePresentationSprite{Sprite: 0x84c / 12, X: x, Y: y, HalfWidth: half, Height: height, Routine: procedure})
		c.D7 = hudWord(c.D7, uint16(height*half/4))
		remaining >>= 4
	}
	return plan, nil
}

// AlternateClear$ c826 and AlternateView$c204 preserve the complete three
// consumed registers. c204 restores all data registers after its internal
// render loop; this contract does not replace that drawing with a generic map.
func (*NativePresentationContextRules) AlternateClear(*NativeHUDRegisters) {}
func (*NativePresentationContextRules) AlternateView(*NativeHUDRegisters)  {}

// CountdownText$501a/$509a and Portrait$bbe0 likewise restore these registers
// around all glyph/face child routines and the OS synchronization wrappers.
func (*NativePresentationContextRules) CountdownText(*NativeHUDRegisters) {}
func (*NativePresentationContextRules) Portrait(*NativeHUDRegisters)      {}

// CameraMarker is original$1062..$10aa. View8 draws its descriptor even
// without an under-map cursor; the source plane stride survives in D7.W.
func (r *NativePresentationContextRules) CameraMarker(m FollowerCleanupMemory, c *NativeHUDRegisters) ([]NativePresentationSprite, error) {
	if r == nil || c == nil || !winMemoryValid(m) {
		return nil, fmt.Errorf("native camera-marker context missing")
	}
	view, err := m.Read16(0xf0c)
	if err != nil {
		return nil, err
	}
	if view != 8 {
		return []NativePresentationSprite{}, nil
	}
	x, err := m.Read16(0x5f44)
	if err != nil {
		return nil, err
	}
	y, err := m.Read16(0x5f46)
	if err != nil {
		return nil, err
	}
	px := int16(uint16(68 + x - y))
	py := int16(uint16((x+y+6)>>1) + 4)
	a := 0x219da
	half, height := int16(binary.BigEndian.Uint16(r.code[a+4:])), int16(binary.BigEndian.Uint16(r.code[a+6:]))
	procedure := binary.BigEndian.Uint32(r.code[a+8:])
	if procedure != 0xf0ee {
		return nil, fmt.Errorf("native camera-marker primitive needs own continuation")
	}
	c.D7 = hudWord(c.D7, uint16(half*height/4))
	return []NativePresentationSprite{{Sprite: (a - 0x21626) / 12, X: px, Y: py, HalfWidth: half, Height: height, Routine: procedure}}, nil
}

// CursorNonEdit is$1be8's surviving normal-play register contract. Its
// pointer-image dispatch/admission query preserve these registers. Editor
// preview draws$ee32 and explicitly requires its own image continuation.
func (*NativePresentationContextRules) CursorNonEdit(m FollowerCleanupMemory, c *NativeHUDRegisters) error {
	if c == nil || m.Read16 == nil {
		return fmt.Errorf("native cursor context missing")
	}
	edit, err := m.Read16(0xf0e)
	if err != nil {
		return err
	}
	if edit != 0 {
		return fmt.Errorf("native editor cursor image continuation required")
	}
	return nil
}

type NativeFrameInputPresentation struct {
	HUD       NativeHUDPlan
	Selected  NativeSelectedPresentation
	Camera    []NativePresentationSprite
	MapCursor []NativePresentationSprite
}

// MapCursor retains the surviving$ f12..$1062 overlay register effect. Border
// pixels use$e17a and preserve these registers; the pointer's own descriptor
// assigns D7.W before clipping in every view. Command-pointer updates remain
// the owning input dispatcher's separate state operation.
func (r *NativePresentationContextRules) MapCursor(m FollowerCleanupMemory, c *NativeHUDRegisters) ([]NativePresentationSprite, error) {
	if r == nil || c == nil || m.Read16 == nil {
		return nil, fmt.Errorf("native map-cursor context missing")
	}
	x, err := m.Read16(0x5f48)
	if err != nil {
		return nil, err
	}
	y, err := m.Read16(0x5f4a)
	if err != nil {
		return nil, err
	}
	if y == 0 {
		return []NativePresentationSprite{}, nil
	}
	a := 0x219da
	half, height := int16(binary.BigEndian.Uint16(r.code[a+4:])), int16(binary.BigEndian.Uint16(r.code[a+6:]))
	proc := binary.BigEndian.Uint32(r.code[a+8:])
	if proc != 0xf0ee {
		return nil, fmt.Errorf("native map-cursor primitive needs own continuation")
	}
	c.D7 = hudWord(c.D7, uint16(half*height/4))
	return []NativePresentationSprite{{Sprite: (a - 0x21626) / 12, X: int16((x & 0x1f0) - 3), Y: int16(y + 1), HalfWidth: half, Height: height, Routine: proc}}, nil
}

// NormalFrameInput composes the surviving$eb6..$10b6 presentation contract.
// Earlier terrain rendering determines its input. The final camera marker
// can supersede selected-pane D7 and is derived from its actual descriptor.
func (r *NativePresentationContextRules) NormalFrameInput(cb NativePresentationContextCallbacks, c *NativeHUDRegisters) (NativeFrameInputPresentation, error) {
	var plan NativeFrameInputPresentation
	if r == nil || c == nil || !winMemoryValid(cb.Memory) {
		return plan, fmt.Errorf("native normal-frame input context missing")
	}
	view, err := cb.Memory.Read16(0xf0c)
	if err != nil {
		return plan, err
	}
	if view == 8 {
		plan.HUD, err = r.HUD.Continue(cb.Memory, c, true)
		if err != nil {
			return plan, err
		}
		plan.Selected, err = r.Selected(cb, c)
		if err != nil {
			return plan, err
		}
		edit, err := cb.Memory.Read16(0xf0e)
		if err != nil {
			return plan, err
		}
		if edit == 0 {
			r.CountdownText(c)
		}
		r.Portrait(c)
	} else {
		r.AlternateClear(c)
		r.AlternateView(c)
	}
	plan.MapCursor, err = r.MapCursor(cb.Memory, c)
	if err != nil {
		return plan, err
	}
	plan.Camera, err = r.CameraMarker(cb.Memory, c)
	if err != nil {
		return plan, err
	}
	if err := r.CursorNonEdit(cb.Memory, c); err != nil {
		return plan, err
	}
	return plan, nil
}
