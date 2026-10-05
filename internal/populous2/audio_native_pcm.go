package populous2

import (
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
)

type NativeAudioTiming struct {
	SampleRate     int
	CIAHz, PaulaHz uint32
	TimerLow       uint8
}

func NativePALAudioTiming(sampleRate int, timerLow uint8) NativeAudioTiming {
	return NativeAudioTiming{sampleRate, 709379, 3546895, timerLow}
}
func NativeNTSCAudioTiming(sampleRate int, timerLow uint8) NativeAudioTiming {
	return NativeAudioTiming{sampleRate, 715909, 3579545, timerLow}
}

type nativePaulaPCMChannel struct {
	location               uint32
	length, period, volume uint16
	dma                    bool
	current                uint32
	remaining              uint32
	value                  int8
	next                   uint64
}

// NativeAudioPCM is a four-channel Paula reader driven by the proven native
// CIA device. Programmed pointers/lengths remain distinct from a running DMA
// block; loop writes take effect when that block ends. Integer color-clock
// deadlines avoid a simulation-rate or floating-point sequencer substitute.
type NativeAudioPCM struct {
	mu                                 sync.Mutex
	device                             *NativeAudioDevice
	dma                                *NativePaulaDMAState
	timing                             NativeAudioTiming
	channels                           [4]nativePaulaPCMChannel
	frame                              NativeFrameRegisterContext
	clock, nextCIA, ciaPeriod, samples uint64
	pending                            [4]byte
	pendingAt, pendingLen              int
	events                             []NativeAudioTimedEvent
}

// NativeAudioTimedEvent places a native command immediately before its CIA
// update, for deterministic frame/audio synchronization and source replay.
type NativeAudioTimedEvent struct {
	CIAUpdate     uint64
	Kind          string
	Control, Data uint16
}

func (p *NativeAudioPCM) ScheduleCIAEvent(event NativeAudioTimedEvent) error {
	if p == nil || event.CIAUpdate == 0 {
		return fmt.Errorf("native timed audio event invalid")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if event.CIAUpdate*p.ciaPeriod < p.clock {
		return fmt.Errorf("native timed audio event precedes current stream")
	}
	p.events = append(p.events, event)
	sort.SliceStable(p.events, func(i, j int) bool { return p.events[i].CIAUpdate < p.events[j].CIAUpdate })
	return nil
}

func NewNativeAudioPCM(device *NativeAudioDevice, timing NativeAudioTiming) (*NativeAudioPCM, error) {
	if device == nil || timing.SampleRate < 8000 || timing.SampleRate > 192000 || timing.CIAHz == 0 || timing.PaulaHz == 0 || timing.PaulaHz%timing.CIAHz != 0 {
		return nil, fmt.Errorf("native PCM device/rational clocks invalid")
	}
	device.TimerLow = timing.TimerLow
	period := (uint64(0x1900|uint16(timing.TimerLow)) + 1) * uint64(timing.PaulaHz/timing.CIAHz)
	return &NativeAudioPCM{device: device, timing: timing, nextCIA: period, ciaPeriod: period}, nil
}

// NewNativeAudioPCMWithDMA selects the raster DMA transport. The existing
// constructor remains the byte-stream reference for CPU register traces.
// Beam/request phase is supplied explicitly; IRQ writes still arrive at the
// configured CIA boundary, rather than claiming captured68000 bus timestamps.
func NewNativeAudioPCMWithDMA(device *NativeAudioDevice, timing NativeAudioTiming, config NativePaulaDMAConfig) (*NativeAudioPCM, error) {
	p, err := NewNativeAudioPCM(device, timing)
	if err != nil {
		return nil, err
	}
	p.dma, err = NewNativePaulaDMAState(config, func(address uint32) (uint16, error) {
		base := device.long(0x18ec6) - 4
		at := int(int64(address) - int64(base))
		if at >= 0 && at <= len(device.FX)-2 {
			return binary.BigEndian.Uint16(device.FX[at:]), nil
		}
		// Startup discards a real fetch from the previous pointer, often0.
		// It must read configured backing; an invented zero word is not used.
		if device.ReadAbsolute8 == nil {
			return 0, fmt.Errorf("native DMA physical word %x backing missing", address)
		}
		h, err := device.ReadAbsolute8(address)
		if err != nil {
			return 0, err
		}
		l, err := device.ReadAbsolute8(address + 1)
		return uint16(h)<<8 | uint16(l), err
	})
	return p, err
}

func (p *NativeAudioPCM) DMASnapshot() (NativePaulaDMASnapshot, bool) {
	if p == nil {
		return NativePaulaDMASnapshot{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dma == nil {
		return NativePaulaDMASnapshot{}, false
	}
	return p.dma.Snapshot(), true
}

func (p *NativeAudioPCM) sample(address uint32) (int8, error) {
	base := p.device.long(0x18ec6) - 4
	at := int(int64(address) - int64(base))
	if at < 4 || at >= len(p.device.FX) {
		return 0, fmt.Errorf("native DMA sample address %x outside retained FX", address)
	}
	return int8(p.device.FX[at]), nil
}

func nativePaulaPeriod(v uint16) uint64 {
	if v == 0 {
		return 65536
	}
	return uint64(v)
}
func nativePaulaLength(v uint16) uint32 {
	if v == 0 {
		return 131072
	}
	return uint32(v) * 2
}
func nativePaulaVolume(v uint16) int32 {
	if v&64 != 0 {
		return 64
	}
	return int32(v & 63)
}

func (p *NativeAudioPCM) reload(channel *nativePaulaPCMChannel) error {
	channel.current = channel.location
	channel.remaining = nativePaulaLength(channel.length)
	value, err := p.sample(channel.current)
	if err != nil {
		return err
	}
	channel.value = value
	channel.current++
	channel.remaining--
	return nil
}

func (p *NativeAudioPCM) apply(writes []NativeFrameHardwareWrite) error {
	if p.dma != nil {
		return p.dma.Apply(writes)
	}
	for _, w := range writes {
		if w.Address == 0xdff096 {
			for i := range p.channels {
				if w.Value&(1<<uint(i)) == 0 {
					continue
				}
				c := &p.channels[i]
				if w.Value&0x8000 == 0 {
					c.dma = false
					continue
				}
				if !c.dma {
					c.dma = true
					if err := p.reload(c); err != nil {
						return err
					}
					c.next = p.clock + nativePaulaPeriod(c.period)
				}
			}
			continue
		}
		if w.Address < 0xdff0a0 || w.Address >= 0xdff0e0 {
			continue
		}
		channel := (w.Address - 0xdff0a0) / 16
		offset := (w.Address - 0xdff0a0) % 16
		c := &p.channels[channel]
		switch offset {
		case 0:
			if w.Width == 4 {
				c.location = w.Value
			} else {
				c.location = c.location&0xffff | w.Value<<16
			}
		case 2:
			c.location = c.location&0xffff0000 | w.Value&0xffff
		case 4:
			c.length = uint16(w.Value)
		case 6:
			c.period = uint16(w.Value)
		case 8:
			c.volume = uint16(w.Value)
		}
	}
	return nil
}

func (p *NativeAudioPCM) runUntil(timeNumerator uint64) error {
	if p.dma != nil {
		return p.runDMAUntil(timeNumerator)
	}
	rate := uint64(p.timing.SampleRate)
	for {
		next := p.nextCIA
		for i := range p.channels {
			c := &p.channels[i]
			if c.dma && c.next < next {
				next = c.next
			}
		}
		if next*rate > timeNumerator {
			return nil
		}
		p.clock = next
		// CIA writes are applied in original source order before same-time
		// sample transitions. Raster DMA arbitration is a separate boundary.
		if next == p.nextCIA {
			if err := p.tickCIA(); err != nil {
				return err
			}
			p.nextCIA += p.ciaPeriod
		}
		for i := range p.channels {
			c := &p.channels[i]
			if !c.dma || c.next != next {
				continue
			}
			if c.remaining == 0 {
				if err := p.reload(c); err != nil {
					return err
				}
			} else {
				value, err := p.sample(c.current)
				if err != nil {
					return err
				}
				c.value = value
				c.current++
				c.remaining--
			}
			c.next += nativePaulaPeriod(c.period)
		}
	}
}

func (p *NativeAudioPCM) tickCIA() error {
	p.device.Hardware = nil
	update := p.nextCIA / p.ciaPeriod
	for len(p.events) > 0 && p.events[0].CIAUpdate == update {
		e := p.events[0]
		p.events = p.events[1:]
		var err error
		switch e.Kind {
		case "cue":
			p.frame.Word(0, e.Data)
			err = p.device.DirectCue(e.Data, &p.frame)
		case "music":
			p.frame.D[0], err = p.device.MusicCommand(e.Control, e.Data, p.frame.D[0])
		case "command":
			p.frame.D[0], err = p.device.Command(e.Control, e.Data, p.frame.D[0])
		default:
			return fmt.Errorf("native timed audio command kind %q unsupported", e.Kind)
		}
		if err != nil {
			return err
		}
	}
	if err := p.device.TickCIA(&p.frame); err != nil {
		return err
	}
	return p.apply(p.device.Hardware)
}

func (p *NativeAudioPCM) runDMAUntil(timeNumerator uint64) error {
	target := timeNumerator / uint64(p.timing.SampleRate)
	for p.nextCIA <= target {
		if err := p.dma.AdvanceTo(p.nextCIA); err != nil {
			return err
		}
		p.clock = p.nextCIA
		if err := p.tickCIA(); err != nil {
			return err
		}
		p.nextCIA += p.ciaPeriod
	}
	if err := p.dma.AdvanceTo(target); err != nil {
		return err
	}
	p.clock = target
	return nil
}

func (p *NativeAudioPCM) nextFrame(dst []byte) error {
	if err := p.runUntil(p.samples * uint64(p.timing.PaulaHz)); err != nil {
		return err
	}
	var left, right int32
	for i := range p.channels {
		value := int32(0)
		if p.dma != nil {
			c := p.dma.channels[i]
			value = int32(c.Sample) * int32(c.OutputVolume) * 2
		} else {
			c := p.channels[i]
			if !c.dma {
				continue
			}
			value = int32(c.value) * nativePaulaVolume(c.volume) * 2
		}
		if i == 0 || i == 3 {
			left += value
		} else {
			right += value
		}
	}
	clip := func(v int32) int16 {
		if v > 32767 {
			return 32767
		}
		if v < -32768 {
			return -32768
		}
		return int16(v)
	}
	binary.LittleEndian.PutUint16(dst, uint16(clip(left)))
	binary.LittleEndian.PutUint16(dst[2:], uint16(clip(right)))
	p.samples++
	return nil
}

func (p *NativeAudioPCM) Read(dst []byte) (int, error) {
	if p == nil {
		return 0, fmt.Errorf("native PCM reader missing")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	written := 0
	for len(dst) > 0 {
		if p.pendingLen == 0 {
			if err := p.nextFrame(p.pending[:]); err != nil {
				return written, err
			}
			p.pendingAt, p.pendingLen = 0, 4
		}
		n := copy(dst, p.pending[p.pendingAt:p.pendingAt+p.pendingLen])
		p.pendingAt += n
		p.pendingLen -= n
		written += n
		dst = dst[n:]
	}
	return written, nil
}

func (p *NativeAudioPCM) Command(control, data uint16, input uint32) (uint32, error) {
	if p == nil {
		return 0, fmt.Errorf("native PCM device missing")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.synchronizeCommands(); err != nil {
		return 0, err
	}
	p.device.Hardware = nil
	status, err := p.device.Command(control, data, input)
	if err != nil {
		return status, err
	}
	return status, p.apply(p.device.Hardware)
}

func (p *NativeAudioPCM) MusicCommand(control, data uint16, input uint32) (uint32, error) {
	if p == nil {
		return 0, fmt.Errorf("native PCM device missing")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.synchronizeCommands(); err != nil {
		return 0, err
	}
	p.device.Hardware = nil
	status, err := p.device.MusicCommand(control, data, input)
	if err != nil {
		return status, err
	}
	return status, p.apply(p.device.Hardware)
}

func (p *NativeAudioPCM) DirectCue(offset uint16, c *NativeFrameRegisterContext) error {
	if p == nil {
		return fmt.Errorf("native PCM device missing")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.synchronizeCommands(); err != nil {
		return err
	}
	p.device.Hardware = nil
	if err := p.device.DirectCue(offset, c); err != nil {
		return err
	}
	return p.apply(p.device.Hardware)
}

func (p *NativeAudioPCM) synchronizeCommands() error {
	numerator := p.samples * uint64(p.timing.PaulaHz)
	if err := p.runUntil(numerator); err != nil {
		return err
	}
	p.clock = numerator / uint64(p.timing.SampleRate)
	return nil
}
