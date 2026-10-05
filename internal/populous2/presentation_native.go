package populous2

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	"go-populous2/internal/amiga"
)

type NativeMenuTemplate uint16

const (
	NativeMenuSpellHelp NativeMenuTemplate = 0x5530
	NativeMenuZeus      NativeMenuTemplate = 0x6e02
	NativeMenuOpponent  NativeMenuTemplate = 0x7208
	NativeMenuDeity     NativeMenuTemplate = 0x760e
	NativeMenuPaint     NativeMenuTemplate = 0x7a14
	NativeMenuFiles     NativeMenuTemplate = 0x7af2
	NativeMenuResult    NativeMenuTemplate = 0x7db2
	NativeMenuWorld     NativeMenuTemplate = 0x809c
	NativeMenuStartup   NativeMenuTemplate = 0x8854
	NativeMenuInGame    NativeMenuTemplate = 0x88e4
	NativeMenuOptions   NativeMenuTemplate = 0x8c42
	NativeMenuSerial    NativeMenuTemplate = 0x901e
	NativeMenuMessage   NativeMenuTemplate = 0x91d0
	NativeMenuOverwrite NativeMenuTemplate = 0x930c
)

// NativePresentation keeps the supplied executable's byte-coded menu layouts,
// original glyphs and embedded startup screen. QAZ.PAK is the game interface;
// it is not the bitmap copied by the startup routine at $3b64.
type NativePresentation struct {
	Font             *NativeMenuFont
	Requesters       NativeRequesterRules
	Templates        map[NativeMenuTemplate][]byte
	StartupRequester *NativeRequester
	StartupPixels    []byte
	StartupPalette   [16]color.RGBA
	EndingText       []byte
	Widgets          NativeMenuWidgetRules
	winners          [2][]byte
	daySuffix        []byte
	decimalDivisors  [10]uint32
}

func DecodeNativePresentation(exe *amiga.Executable) (*NativePresentation, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x3c548 {
		return nil, fmt.Errorf("native presentation data missing")
	}
	code := exe.Hunks[0].Data
	p := &NativePresentation{Templates: make(map[NativeMenuTemplate][]byte)}
	var err error
	if p.Font, err = DecodeNativeMenuFont(exe); err != nil {
		return nil, err
	}
	if p.Requesters, err = DecodeNativeRequesterRules(exe); err != nil {
		return nil, err
	}
	if p.Widgets, err = DecodeNativeMenuWidgetRules(exe); err != nil {
		return nil, err
	}
	for _, template := range []NativeMenuTemplate{NativeMenuSpellHelp, NativeMenuZeus, NativeMenuOpponent, NativeMenuDeity, NativeMenuPaint, NativeMenuFiles, NativeMenuResult, NativeMenuWorld, NativeMenuStartup, NativeMenuInGame, NativeMenuOptions, NativeMenuSerial, NativeMenuMessage, NativeMenuOverwrite} {
		if p.Templates[template], err = NativeRequesterTemplate(exe, int(template)); err != nil {
			return nil, err
		}
	}
	if p.StartupRequester, err = p.Requesters.Startup(exe); err != nil {
		return nil, err
	}
	if p.StartupPalette, err = NativeStartupPalette(exe); err != nil {
		return nil, err
	}
	if p.StartupPixels, err = nativeScreenIndices(code[0x34828 : 0x34828+32000]); err != nil {
		return nil, err
	}
	readText := func(start int) ([]byte, error) {
		for end := start; end < len(code) && end-start < 4096; end++ {
			if code[end] == 0 {
				return append([]byte(nil), code[start:end]...), nil
			}
		}
		return nil, fmt.Errorf("native presentation text has no terminator")
	}
	if p.EndingText, err = readText(0xaa24); err != nil {
		return nil, err
	}
	for index, offset := range []int{0x9670, 0x9675} {
		if p.winners[index], err = readText(offset); err != nil {
			return nil, err
		}
	}
	if p.daySuffix, err = readText(0xa96c); err != nil {
		return nil, err
	}
	for index := range p.decimalDivisors {
		p.decimalDivisors[index] = binary.BigEndian.Uint32(code[0x1006a+index*4:])
		if p.decimalDivisors[index] == 0 {
			return nil, fmt.Errorf("native decimal divisor missing")
		}
	}
	return p, nil
}

// Compile exposes the real layouts and field truncation. Caller-specific
// checkbox, face and experience overlays are separate from template text.
func (p *NativePresentation) Compile(template NativeMenuTemplate, parameters [][]byte) (*NativeRequester, error) {
	if p == nil || p.Templates[template] == nil {
		return nil, fmt.Errorf("native presentation template missing")
	}
	return p.Requesters.Compile(p.Templates[template], parameters)
}

// Compose replaces only the glyph cells written by $509a. Pixels outside the
// requester retain the supplied background; index zero inside glyphs is opaque.
func (p *NativePresentation) Compose(background []byte, requester *NativeRequester, palette [16]color.RGBA) (*image.Paletted, error) {
	if p == nil || requester == nil || len(background) != 320*200 {
		return nil, fmt.Errorf("native requester composition input missing")
	}
	pixels := append([]byte(nil), background...)
	if err := p.Font.DrawIndices(pixels, string(requester.Text), requester.Column, requester.Row); err != nil {
		return nil, err
	}
	return nativeIndexedImage(pixels, palette)
}

func (p *NativePresentation) StartupImage() (*image.Paletted, error) {
	if p == nil {
		return nil, fmt.Errorf("native startup presentation missing")
	}
	return p.Compose(p.StartupPixels, p.StartupRequester, p.StartupPalette)
}

type NativeResultPresentation struct {
	Requester       *NativeRequester
	Parameters      [11][]byte
	DisplayedMetric [2]uint16
	AudioDescriptor uint16
	WaitVBlanks     int
}

// decimal follows $10024's signed-negative test and byte-sized digit counter.
// It deliberately preserves odd strings from negative wrapped input instead
// of replacing the native arithmetic with a signed decimal formatter.
func (p *NativePresentation) decimal(value uint32) []byte {
	if value == 0 {
		return []byte{'0'}
	}
	result := []byte{}
	for _, divisor := range p.decimalDivisors {
		digit := byte('/')
		for {
			digit++
			value -= divisor
			if int32(value) < 0 {
				break
			}
		}
		value += divisor
		if len(result) != 0 || digit != '0' {
			result = append(result, digit)
		}
	}
	return result
}

func nativeResultMetric(dividend uint32, bonus bool) (uint32, uint16) {
	quotient, remainder := int32(dividend)/4, int32(dividend)%4
	if quotient >= -32768 && quotient <= 32767 {
		dividend = uint32(uint16(remainder))<<16 | uint32(uint16(quotient))
	}
	if bonus {
		dividend = dividend&0xffff0000 | uint32(uint16(dividend)+50)
	}
	if int16(dividend) <= 0 {
		dividend = 1
	} else if int16(dividend) > 99 {
		dividend = 99
	}
	return dividend, uint16(dividend)
}

// Result translates $3878..$39e8's presentation boundary. Score is supplied
// by the separately verified CampaignResultRules.Score. Metric division keeps
// D0's remainder high word when loading the second metric, including DIVS
// overflow. The elapsed DIVU similarly preserves its dividend on overflow.
func (p *NativePresentation) Result(selected, eliminated uint16, ticks uint32, local, opponent CampaignResultStatistics, score uint16) (NativeResultPresentation, error) {
	var r NativeResultPresentation
	if p == nil || selected < 1 || selected > 2 || eliminated < 1 || eliminated > 2 {
		return r, fmt.Errorf("native result profile/outcome outside 1/2")
	}
	r.AudioDescriptor, r.WaitVBlanks = 0x168, 101
	winner := p.winners[0]
	if eliminated == 1 {
		r.AudioDescriptor, winner = 0x500, p.winners[1]
	}
	r.Parameters[0] = append([]byte(nil), winner...)
	d0, metric := nativeResultMetric(uint32(local.Metric), eliminated == 1)
	r.DisplayedMetric[0] = metric
	_, r.DisplayedMetric[1] = nativeResultMetric(d0&0xffff0000|uint32(opponent.Metric), eliminated == 2)
	elapsed := ticks
	if ticks/50 <= 0xffff {
		elapsed = ticks/50 | ticks%50<<16
	}
	elapsed = uint32(int32(int16(elapsed)))
	// $3afc..$3b62 is one shared text buffer, not independent strings.
	// A ten-digit peak can overlap the next eight-byte field; subsequent
	// formatting then overwrites its tail before requester substitution.
	var scratch [0x66]byte
	for index := range scratch {
		scratch[index] = ' '
	}
	offsets := [10]int{0, 14, 22, 30, 38, 62, 70, 78, 86, 94}
	values := [10][]byte{
		append(p.decimal(elapsed), p.daySuffix...),
		p.decimal(local.PeakPopulation), p.decimal(opponent.PeakPopulation),
		p.decimal(local.PeakMana), p.decimal(opponent.PeakMana),
		p.decimal(uint32(int32(int16(local.BattleWins)))), p.decimal(uint32(int32(int16(opponent.BattleWins)))),
		p.decimal(uint32(int32(int16(local.LeaderLosses)))), p.decimal(uint32(int32(int16(opponent.LeaderLosses)))),
		p.decimal(uint32(score)),
	}
	for index, value := range values {
		copy(scratch[offsets[index]:], value)
		scratch[offsets[index]+len(value)] = 0
	}
	for index, offset := range offsets {
		end := offset
		for scratch[end] != 0 {
			end++
		}
		r.Parameters[index+1] = append([]byte(nil), scratch[offset:end]...)
	}
	parameters := make([][]byte, len(r.Parameters))
	copy(parameters, r.Parameters[:])
	var err error
	r.Requester, err = p.Compile(NativeMenuResult, parameters)
	return r, err
}
