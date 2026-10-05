package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeAudioDevice retains the initialized driver's actual mutable CODE.
// Four primary records and four secondary effect records share four Paula
// channels; additional effects are not independent additive PCM voices.
type NativeAudioDevice struct {
	Code                   []byte
	FX                     []byte
	CodeBase, ResourceBase uint32
	TimerLow               uint8 // The executable writes only the $19 high latch byte.
	Hardware               []NativeFrameHardwareWrite
	// A native null envelope can read actual absolute low memory. The host
	// must provide that backing explicitly; it is not a fabricated zero bank.
	ReadAbsolute8 func(uint32) (uint8, error)
}

func NewNativeAudioDevice(exe *amiga.Executable, fx []byte, codeBase, resourceBase uint32, timerLow uint8) (*NativeAudioDevice, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x3e40c {
		return nil, fmt.Errorf("native audio driver CODE missing")
	}
	if _, err := DecodeAudioBank(exe, fx); err != nil {
		return nil, err
	}
	d := &NativeAudioDevice{Code: append([]byte(nil), exe.Hunks[0].Data...), FX: append([]byte(nil), fx...), CodeBase: codeBase, ResourceBase: resourceBase, TimerLow: timerLow}
	for _, rel := range exe.Hunks[0].Relocations {
		if rel.Target != 0 {
			continue
		}
		for _, offset := range rel.Offsets {
			binary.BigEndian.PutUint32(d.Code[offset:], binary.BigEndian.Uint32(d.Code[offset:])+codeBase)
		}
	}
	return d, nil
}

func (d *NativeAudioDevice) word(at int) uint16       { return binary.BigEndian.Uint16(d.Code[at:]) }
func (d *NativeAudioDevice) long(at int) uint32       { return binary.BigEndian.Uint32(d.Code[at:]) }
func (d *NativeAudioDevice) putWord(at int, v uint16) { binary.BigEndian.PutUint16(d.Code[at:], v) }
func (d *NativeAudioDevice) putLong(at int, v uint32) { binary.BigEndian.PutUint32(d.Code[at:], v) }
func (d *NativeAudioDevice) codeAt(address uint32, size int) (int, error) {
	at := int(int64(address) - int64(d.CodeBase))
	if at < 0 || size < 0 || at > len(d.Code)-size {
		return 0, fmt.Errorf("native audio address %x outside retained CODE", address)
	}
	return at, nil
}

// Initialize translates $18ada/$18ee4 and score-zero $1926c. The portable
// backend retains the same interrupt descriptor and timer-register requests;
// installing a host audio stream remains the caller's explicit operation.
func (d *NativeAudioDevice) Initialize() ([]NativeFrameHardwareWrite, error) {
	if d == nil || len(d.Code) < 0x3e40c {
		return nil, fmt.Errorf("native audio initialization missing")
	}
	if d.ResourceBase == 0 {
		return nil, nil
	}
	d.putLong(0x18ec6, d.ResourceBase+4)
	d.putLong(0x18eca, d.CodeBase+0x3e3f8)
	d.putLong(0x18b2a, d.CodeBase+0x18dea)
	for i, offset := range []int{0x1988c, 0x198e6, 0x1989c, 0x19932} {
		d.putLong(0x18b36+i*4, d.CodeBase+uint32(offset))
	}
	for _, p := range []struct{ At, Value int }{{0x18b58, 0x19258}, {0x18b5e, 0x1924a}, {0x18b64, 0x19262}} {
		d.putLong(p.At, d.CodeBase+uint32(p.Value))
	}
	for i := 0; i < 8; i++ {
		at := 0x18b6a + i*80
		parent := at
		if i >= 4 {
			parent -= 320
		}
		d.putLong(at+4, d.CodeBase+uint32(parent))
		d.putLong(at+16, d.CodeBase+uint32(at+48))
		d.putLong(at+20, d.CodeBase+uint32(at+64))
		env := 0x18dea
		if i >= 4 {
			env = 0x18df2
		}
		d.putLong(at+24, d.CodeBase+uint32(env))
		if i < 4 {
			d.putLong(at+28, d.CodeBase+uint32(at+320))
		}
	}
	d.Code[0x18ece+9], d.Code[0x18ece+8] = 0xd8, 2
	d.putLong(0x18ece+18, d.CodeBase+0x1942a)
	if err := d.loadScoreZero(); err != nil {
		return nil, err
	}
	return []NativeFrameHardwareWrite{{0x1909a, 0xbfdd00, 0x81, 1}, {0x190a2, 0xbfd500, 0x19, 1}, {0x190aa, 0xbfde00, 1, 1}}, nil
}

// InitializeWithFrame retains $18ada's caller continuation: D0 is the
// resource pointer plus its four-byte header, while D1-D7 survive unchanged.
func (d *NativeAudioDevice) InitializeWithFrame(c *NativeFrameRegisterContext) ([]NativeFrameHardwareWrite, error) {
	if d == nil || c == nil {
		return nil, fmt.Errorf("native audio initialization frame missing")
	}
	writes, err := d.Initialize()
	if err == nil {
		c.D[0] = d.ResourceBase
		if d.ResourceBase != 0 {
			c.D[0] += 4
		}
	}
	return writes, err
}

func (d *NativeAudioDevice) loadScoreZero() error {
	base := 0x3e3f8 + int(int16(d.word(0x3e3f8)))
	if base < 0 || base+18 > len(d.Code) {
		return fmt.Errorf("native score header outside CODE")
	}
	d.Code[0x18b30] = 0
	d.putLong(0x18b22, d.CodeBase+uint32(base))
	tempo := d.word(base)
	d.Code[0x18b31], d.Code[0x18b32] = uint8(tempo), uint8(tempo)
	d.putLong(0x18b1a, d.CodeBase+uint32(base+int(int16(d.word(base+2)))))
	d.putLong(0x18b1e, d.CodeBase+uint32(base+int(int16(d.word(base+4)))))
	for i := 0; i < 4; i++ {
		d.putLong(0x18b6a+i*80, d.CodeBase+uint32(base+int(int16(d.word(base+6+i*2)))))
	}
	d.putLong(0x18b26, d.CodeBase+uint32(base+16))
	d.Code[0x18b33] = 63
	for i := 0; i < 4; i++ {
		at := 0x18b6a + i*80
		d.putWord(at+38, 0x40)
		d.Code[at+43], d.Code[at+47] = 63, 63
	}
	return nil
}

// Command is initialized $190e4. Stack arguments are represented explicitly;
// the original restores D1-D7 and returns only its real D0/status long.
func (d *NativeAudioDevice) Command(control, data uint16, input uint32) (uint32, error) {
	if d == nil {
		return 0, fmt.Errorf("native audio device missing")
	}
	if int32(d.ResourceBase) <= 0 {
		return input, nil
	}
	status := uint32(0xffff)
	mask := control & 15
	if mask == 0 {
		mask = 15
	}
	for channel := 0; channel < 4; channel++ {
		if mask&(1<<uint(channel)) == 0 {
			continue
		}
		primary := 0x18b6a + channel*80
		secondary, err := d.codeAt(d.long(primary+28), 80)
		if err != nil {
			return status, err
		}
		if control&0x8000 != 0 {
			d.Code[secondary+47] = uint8(data)
		}
		if control&0x2000 != 0 {
			d.Code[secondary+43], d.Code[secondary+47] = uint8(data), uint8(data)
		}
		if control&0x0100 != 0 {
			d.Code[secondary+43] = 0
			d.Code[secondary+47] = d.Code[0x18b33]
			status = status&0xffffff00 | uint32(d.Code[0x18b33])
		}
		if control&0x0200 != 0 {
			d.putWord(secondary+38, d.word(secondary+38)|0x0200)
		}
		if control&0x0010 != 0 {
			status = 0xffffffff
			if d.Code[primary+39]&2 != 0 {
				status = 1
			}
		}
		if control&0x0040 != 0 {
			d.putWord(secondary+38, 0x40)
			d.Code[primary+39] &^= 2
			if d.Code[primary+39]&0x40 != 0 {
				d.Hardware = append(d.Hardware, NativeFrameHardwareWrite{0x1987a, uint32(0xdff0a8 + channel*16), 0, 2}, NativeFrameHardwareWrite{0x19880, 0xdff096, uint32(1 << uint(channel)), 2})
			}
		}
		if control&0x0080 != 0 && (control&0x4000 == 0 || d.Code[primary+39]&2 == 0) {
			d.Code[secondary+45] = uint8(data)
			d.putLong(secondary, d.CodeBase+uint32(secondary+45))
			d.Code[secondary+39] &^= 0x60
			d.putWord(secondary+38, d.word(secondary+38)|0x83)
			d.putWord(primary+38, d.word(primary+38)|2)
			d.Code[secondary+43], d.Code[secondary+47] = 63, 63
			d.Code[0x18b30] = 0xff
		}
		if control&0x0020 != 0 {
			d.putWord(secondary+38, d.word(secondary+38)|0x20)
		}
	}
	return status, nil
}

// DirectCue is complete $184f6's device-call order, not a software flag
// increment. The first start has native channel-mask four; a linked cue uses
// its own channel byte. The outer MOVEM preserves the entire caller frame.
func (d *NativeAudioDevice) DirectCue(offset uint16, c *NativeFrameRegisterContext) error {
	if d == nil || c == nil {
		return fmt.Errorf("native direct cue device/frame missing")
	}
	read := func(raw uint16) (int, error) {
		at := 0x185a8 + int(int16(raw))
		if at&1 != 0 || at < 0 || at+10 > len(d.Code) {
			return 0, fmt.Errorf("native direct cue descriptor %x outside aligned CODE", raw)
		}
		return at, nil
	}
	at, err := read(offset)
	if err != nil {
		return err
	}
	control := (uint16(4) | d.word(at+2)) &^ 0x20
	if _, err = d.Command(control, d.word(at+6), uint32(control)); err != nil {
		return err
	}
	channel := d.Code[at+4]
	if _, err = d.Command(uint16(1)<<uint(channel&63)|0x2000, uint16(d.Code[at+5]), uint32(d.Code[at+5])); err != nil {
		return err
	}
	linked := d.word(at + 8)
	if linked == 0 {
		return nil
	}
	at, err = read(linked)
	if err != nil {
		return err
	}
	channel = d.Code[at+4]
	control = (uint16(1)<<uint(channel&63) | d.word(at+2)) &^ 0x20
	if _, err = d.Command(control, d.word(at+6), uint32(control)); err != nil {
		return err
	}
	_, err = d.Command(uint16(1)<<uint(channel&63)|0x2000, uint16(d.Code[at+5]), uint32(d.Code[at+5]))
	return err
}
