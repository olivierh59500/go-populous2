package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	"go-populous2/internal/amiga"
)

type NativeInGameState struct {
	Profile, GameMode, ControlMode, PaintFlag uint16
}

type NativeInGameContinuation uint8

const (
	NativeInGameContinue NativeInGameContinuation = iota
	NativeInGameOptions
	NativeInGameSerial
	NativeInGameAbout
)

// NativeInGameAction retains menu decisions for the caller's actual command
// stage. A deferred command changes only the queue's command byte; it is not
// an immediate simulation action. Continuations require their real UI or
// transport controller, and do not imply that the operation has succeeded.
type NativeInGameAction struct {
	DeferredCommand uint8
	SwitchProfile   uint16
	Continuation    NativeInGameContinuation
	Close, Ignored  bool
	// Close requests $45b8; the caller must run $181c0 before completing
	// the return. A nonzero transport/status result redraws the menu.
	ResumeCheck uint32
}

type nativeWorldIcon struct {
	Descriptor        uint32
	SourceOffset      uint16
	HalfWidth, Height int16
	planes            []byte
}

type NativeInGameRequesterRules struct {
	Presentation *NativePresentation
	labels       [6][]byte
	humanMatchup []byte
	landscapes   [4][]byte
	opponents    [32][]byte
	syllables    [64][2]byte
	invalidCode  []byte
	about        []byte
	icons        [36]nativeWorldIcon
}

func DecodeNativeInGameRequesterRules(exe *amiga.Executable) (*NativeInGameRequesterRules, error) {
	if exe == nil || len(exe.Hunks) < 4 || len(exe.Hunks[0].Data) < 0x21626 {
		return nil, fmt.Errorf("native in-game requester tables missing")
	}
	p, err := DecodeNativePresentation(exe)
	if err != nil {
		return nil, err
	}
	r := &NativeInGameRequesterRules{Presentation: p}
	code := exe.Hunks[0].Data
	read := func(at, limit int) ([]byte, error) {
		if at < 0 || at+limit > len(code) {
			return nil, fmt.Errorf("native requester string outside CODE")
		}
		end := bytes.IndexByte(code[at:at+limit], 0)
		if end < 0 {
			return nil, fmt.Errorf("native requester string at%x lacks terminator", at)
		}
		return append([]byte(nil), code[at:at+end]...), nil
	}
	for i, at := range []int{0x9670, 0x9675, 0x967a, 0x967e, 0x9682, 0x9696} {
		r.labels[i], err = read(at, 20)
		if err != nil {
			return nil, err
		}
	}
	// The human/human label follows the six primary labels in the table.
	r.humanMatchup, err = read(0x96aa, 20)
	if err != nil {
		return nil, err
	}
	for i := range r.landscapes {
		r.landscapes[i], err = read(0xa825+i*14, 14)
		if err != nil {
			return nil, err
		}
	}
	for i := range r.opponents {
		r.opponents[i], err = read(0x96d6+i*14, 11)
		if err != nil {
			return nil, err
		}
	}
	for i := range r.syllables {
		copy(r.syllables[i][:], code[0x103f8+i*2:0x103fa+i*2])
	}
	r.invalidCode, err = read(0xa922, 26)
	if err != nil {
		return nil, err
	}
	r.about, err = NativeRequesterTemplate(exe, 0x84a2)
	if err != nil {
		return nil, err
	}
	for i := range r.icons {
		offset := binary.BigEndian.Uint16(code[0x21102+i*2:])
		at := 0x214b2 + int(int16(offset))
		if at < 0x214b2 || at+12 > 0x21626 || binary.BigEndian.Uint32(code[at+8:]) != 0xf3a0 {
			return nil, fmt.Errorf("native world icon descriptor unsupported")
		}
		source := binary.BigEndian.Uint32(code[at:])
		half, height := int16(binary.BigEndian.Uint16(code[at+4:])), int16(binary.BigEndian.Uint16(code[at+6:]))
		if half != 16 || height <= 0 || int(source)+int(half*2/8*5*height) > len(exe.Hunks[3].Data) {
			return nil, fmt.Errorf("native world icon pixels missing")
		}
		r.icons[i] = nativeWorldIcon{Descriptor: uint32(at), SourceOffset: uint16(source), HalfWidth: half, Height: height, planes: append([]byte(nil), exe.Hunks[3].Data[int(source):int(source)+int(half*2/8*5*height)]...)}
	}
	return r, nil
}

// Plan translates $4486..$45a4, including the mode indicators and the two
// radio marks. The mode labels use non-action y/z glyphs in this requester.
func (r *NativeInGameRequesterRules) Plan(s NativeInGameState) (*NativeRequester, error) {
	if r == nil || r.Presentation == nil {
		return nil, fmt.Errorf("native in-game requester missing")
	}
	parameters := [][]byte{r.labels[0], r.labels[3], r.labels[4]}
	if s.Profile != 1 {
		parameters[0] = r.labels[1]
	}
	if s.ControlMode == 18 {
		parameters[1] = r.labels[2]
	}
	if s.ControlMode != 2 && s.ControlMode != 18 {
		parameters[2] = r.humanMatchup
		if s.ControlMode == 4 {
			parameters[2] = r.labels[5]
		}
	}
	plan, err := r.Presentation.Compile(NativeMenuInGame, parameters)
	if err != nil {
		return nil, err
	}
	mode, radio := 0, 0
	for i, glyph := range plan.Text {
		switch glyph {
		case 'y':
			checked := s.GameMode == 2
			if mode != 0 {
				checked = s.GameMode == 4
			}
			mode++
			if !checked {
				plan.Text[i] = 'z'
			}
		case 'c':
			checked := s.GameMode == 6
			if radio != 0 {
				checked = s.PaintFlag == 0
			}
			radio++
			if checked {
				plan.Text[i] = 'd'
			}
		}
	}
	return plan, nil
}

// Action translates $45b0..$470c. External operations are explicit returned
// continuations so a caller cannot silently substitute an editor, transport,
// profile switch, or command execution.
func (r *NativeInGameRequesterRules) Action(s *NativeInGameState, action int) (NativeInGameAction, error) {
	var step NativeInGameAction
	if r == nil || s == nil {
		return step, fmt.Errorf("native in-game state missing")
	}
	switch action {
	case 0:
	case 2:
		if s.GameMode == 2 {
			step.Ignored = true
			break
		}
		if s.GameMode == 6 {
			step.DeferredCommand, step.Close = 124, true
			break
		}
		step.SwitchProfile = 1
		if s.Profile == 1 {
			step.SwitchProfile = 2
		}
	case 4, 6:
		value := uint16(18)
		if action == 6 {
			value = 4
		}
		if s.ControlMode == value {
			value = 2
		}
		s.ControlMode = value
	case 8, 10, 12, 14:
		step.DeferredCommand = map[int]uint8{8: 110, 10: 108, 12: 116, 14: 112}[action]
		step.Close = true
	case 16:
		step.Close = true
		if s.GameMode == 2 || s.GameMode == 6 {
			step.Ignored = true
			break
		}
		step.Continuation = NativeInGameSerial
	case 18:
		if s.GameMode == 2 {
			step.Ignored = true
			break
		}
		s.PaintFlag = ^s.PaintFlag
	case 20:
		step.Continuation, step.Close = NativeInGameOptions, true
	case 22:
		step.Continuation = NativeInGameAbout
	case 24:
		step.Close = true
	default:
		return step, fmt.Errorf("native in-game action%d outside original table", action)
	}
	if step.Close {
		step.ResumeCheck = 0x181c0
	}
	return step, nil
}

func (r *NativeInGameRequesterRules) Click(s *NativeInGameState, x, y int) (NativeInGameAction, error) {
	if s == nil {
		return NativeInGameAction{}, fmt.Errorf("native in-game state missing")
	}
	plan, err := r.Plan(*s)
	if err != nil {
		return NativeInGameAction{}, err
	}
	return r.Action(s, r.Presentation.Requesters.Click(plan, x, y))
}

type NativeWorldRequesterState struct {
	World, Landscape, RuleBits uint16
	PowerFlags                 [36]int8
}
type NativeWorldRequesterIcon struct {
	Slot                    uint8
	Descriptor              uint32
	SourceOffset            uint16
	X, Y, HalfWidth, Height int16
}
type NativeWorldRequesterPlan struct {
	Requester *NativeRequester
	Icons     []NativeWorldRequesterIcon
}

// NativeWorldCode is $103c6 with the world word passed by $3cd6. It does not
// multiply a world by the five-world campaign-record stride.
func (r *NativeInGameRequesterRules) NativeWorldCode(world uint16) []byte {
	if r == nil {
		return nil
	}
	value := uint16(uint32(world)*0x24a1+0x24df) & 0x7fff
	code := []byte{}
	for value != 0 {
		code = append(code, r.syllables[value&63][:]...)
		value >>= 6
	}
	return code
}

// FindWorldCode reproduces $3e6a..$3e9a and $1038c. Comparison is exact byte
// equality, including case and spaces; empty input chooses world0 directly.
// Duplicate syllable codes resolve to the first of the 1,000 native worlds.
func (r *NativeInGameRequesterRules) FindWorldCode(input []byte) (uint16, bool) {
	if r == nil {
		return 0, false
	}
	input = nativeFileCString(input)
	if len(input) == 0 {
		return 0, true
	}
	for world := uint16(0); world < 1000; world++ {
		if bytes.Equal(input, r.NativeWorldCode(world)) {
			return world, true
		}
	}
	return 0, false
}

// WorldPlan starts after $3cba's real profile switch to side1. The caller
// supplies that side's live rules and signed availability bytes, not guessed
// campaign defaults. It must perform $11044 when LoadWorld is requested.
func (r *NativeInGameRequesterRules) WorldPlan(s NativeWorldRequesterState) (NativeWorldRequesterPlan, error) {
	var result NativeWorldRequesterPlan
	if r == nil || r.Presentation == nil || s.World >= 1000 || s.Landscape >= 4 {
		return result, fmt.Errorf("native world requester context outside campaign")
	}
	parameters := [][]byte{r.NativeWorldCode(s.World), r.Presentation.decimal(uint32(s.World)), r.landscapes[s.Landscape], r.opponents[s.World/32]}
	if len(parameters[0]) == 0 {
		parameters[0] = []byte{'k'}
	}
	plan, err := r.Presentation.Compile(NativeMenuWorld, parameters)
	if err != nil {
		return result, err
	}
	bits := s.RuleBits
	for i, glyph := range plan.Text {
		if glyph != 'y' && glyph != 'z' {
			continue
		}
		plan.Text[i] = 'z'
		if bits&1 != 0 {
			plan.Text[i] = 'y'
		}
		bits >>= 1
	}
	result.Requester, result.Icons = plan, []NativeWorldRequesterIcon{}
	for slot, enabled := range s.PowerFlags {
		if enabled <= 0 {
			continue
		}
		icon := r.icons[slot]
		row, column := int16(slot/6), int16(slot%6)
		result.Icons = append(result.Icons, NativeWorldRequesterIcon{Slot: uint8(slot), Descriptor: icon.Descriptor, SourceOffset: icon.SourceOffset, X: 30 + row*32 + column*16, Y: 30 + column*8, HalfWidth: icon.HalfWidth, Height: icon.Height})
	}
	return result, nil
}

func (r *NativeInGameRequesterRules) WorldIcon(slot int, palette [16]color.RGBA) (*image.RGBA, error) {
	if r == nil || slot < 0 || slot >= len(r.icons) {
		return nil, fmt.Errorf("native world icon outside descriptor table")
	}
	icon := r.icons[slot]
	return DecodeNativeMaskedPlanes(icon.planes, int(icon.HalfWidth)*2, int(icon.Height), palette)
}

// WorldSpellHit preserves the signed-word isometric hit geometry at $3df2.
// Its returned word is the original $21102 table offset, not a Go spell ID.
func NativeWorldSpellHit(x, y uint16) (uint16, bool) {
	dx, dy := int16(x-126)>>1, int16(y+10)
	column, row := int16(uint16(dx)+uint16(dy))>>4, int16(uint16(dy)-uint16(dx))>>4
	if row < 0 {
		return 0, false
	}
	column = int16(uint16(column) - 5 + uint16(row))
	if column < 0 || column >= 5 || row >= 6 {
		return 0, false
	}
	return uint16((5-row)*6+column) * 2, true
}

type NativeWorldRequesterAction struct {
	EditCode, ShowOpponent, Proceed, Cancel bool
	InvalidCode                             bool
	World                                   uint16
	LoadWorld                               bool
	ShowSpellHelp                           bool
	SpellTableOffset                        uint16
}

func (r *NativeInGameRequesterRules) WorldAction(action int) (NativeWorldRequesterAction, error) {
	var step NativeWorldRequesterAction
	if r == nil {
		return step, fmt.Errorf("native world requester missing")
	}
	switch action {
	case 0:
	case 2:
		step.EditCode = true
	case 4:
		step.ShowOpponent = true
	case 6:
		step.Proceed = true
	case 8:
		step.Cancel = true
	default:
		return step, fmt.Errorf("native world action%d outside original table", action)
	}
	return step, nil
}

func (r *NativeInGameRequesterRules) FinishWorldCode(input []byte) NativeWorldRequesterAction {
	world, ok := r.FindWorldCode(input)
	return NativeWorldRequesterAction{InvalidCode: !ok, World: world, LoadWorld: ok}
}

func (r *NativeInGameRequesterRules) InvalidWorldCodePlan() (*NativeRequester, error) {
	if r == nil || r.Presentation == nil {
		return nil, fmt.Errorf("native world requester missing")
	}
	return r.Presentation.Compile(NativeMenuMessage, [][]byte{r.invalidCode})
}

func (r *NativeInGameRequesterRules) AboutPlan() (*NativeRequester, error) {
	if r == nil || r.Presentation == nil {
		return nil, fmt.Errorf("native in-game requester missing")
	}
	return r.Presentation.Requesters.Compile(r.about, nil)
}

// WorldClick first applies the original icon-region probe, which precedes
// ordinary glyph dispatch. Showing help remains an explicit continuation.
func (r *NativeInGameRequesterRules) WorldClick(s NativeWorldRequesterState, x, y uint16) (NativeWorldRequesterAction, error) {
	if offset, hit := NativeWorldSpellHit(x, y); hit {
		return NativeWorldRequesterAction{ShowSpellHelp: true, SpellTableOffset: offset}, nil
	}
	plan, err := r.WorldPlan(s)
	if err != nil {
		return NativeWorldRequesterAction{}, err
	}
	return r.WorldAction(r.Presentation.Requesters.Click(plan.Requester, int(x), int(y)))
}
