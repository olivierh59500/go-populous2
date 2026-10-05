package populous2

import (
	"fmt"
	"math"
)

// NativePaulaDMAConfig gives the raster geometry and its phase relative to
// audio color-clock zero. Fetch slots14/16/18/20 include the refresh cycle
// omitted by the original manual's Figure6-9. Request-latch phase is explicit:
// CPU register traces alone cannot establish that phase on an original board.
type NativePaulaDMAConfig struct {
	ScanlineClocks                [2]uint16
	RequestClock                  uint16
	FetchClocks                   [4]uint16
	FirstScanlineClock            uint64
	InitialDMA, InitialInterrupts uint16
}

func NativePALPaulaDMAConfig(requestClock uint16) NativePaulaDMAConfig {
	return NativePaulaDMAConfig{ScanlineClocks: [2]uint16{227, 227}, RequestClock: requestClock, FetchClocks: [4]uint16{14, 16, 18, 20}, InitialDMA: 0x200}
}
func NativeNTSCPaulaDMAConfig(requestClock uint16) NativePaulaDMAConfig {
	return NativePaulaDMAConfig{ScanlineClocks: [2]uint16{227, 228}, RequestClock: requestClock, FetchClocks: [4]uint16{14, 16, 18, 20}, InitialDMA: 0x200}
}

type NativePaulaDMAChannelSnapshot struct {
	State                             uint8
	Location, Pointer                 uint32
	Length, WorkingLength             uint16
	Period                            uint32
	Volume, OutputVolume              uint16
	Holding, OutputWord               uint16
	Sample                            int8
	Request, Restart, InterruptOnWord bool
	PeriodRemaining                   uint32
	InterruptCheck                    int8
	DataWritten                       bool
}
type NativePaulaDMASnapshot struct {
	Clock                            uint64
	DMA, Interrupts, LatchedRequests uint16
	Channels                         [4]NativePaulaDMAChannelSnapshot
}
type nativePaulaDMAChannel struct {
	NativePaulaDMAChannelSnapshot
	next uint64
}
type nativePaulaDMAEvent struct {
	clock   uint64
	channel int
	kind    uint8
	data    uint32
}

// NativePaulaDMAState models the documented001/101/010/011 transport. A word
// fetched by Agnus first enters a holding latch. Output alternates its high and
// low bytes at raw AUDPER intervals; without a new word, BOTH bytes repeat.
// This is a digital DMA transport, not an analog filter or complete-chip model.
type NativePaulaDMAState struct {
	Config          NativePaulaDMAConfig
	ReadWord        func(uint32) (uint16, error)
	Clock           uint64
	DMA, Interrupts uint16
	channels        [4]nativePaulaDMAChannel
	events          []nativePaulaDMAEvent
	packet          uint16
	hpos            uint16
	line            uint64
	started         bool
	failed          error
}

func NewNativePaulaDMAState(config NativePaulaDMAConfig, readWord func(uint32) (uint16, error)) (*NativePaulaDMAState, error) {
	if readWord == nil || config.ScanlineClocks[0] == 0 || config.ScanlineClocks[1] == 0 {
		return nil, fmt.Errorf("native Paula DMA backing/raster missing")
	}
	for _, n := range config.ScanlineClocks {
		if config.RequestClock >= n {
			return nil, fmt.Errorf("native Paula request phase outside scanline")
		}
		for _, fetch := range config.FetchClocks {
			if fetch >= n || fetch <= config.RequestClock {
				return nil, fmt.Errorf("native Paula fetch before request latch/outside scanline")
			}
		}
	}
	s := &NativePaulaDMAState{Config: config, ReadWord: readWord, DMA: config.InitialDMA, Interrupts: config.InitialInterrupts}
	for i := range s.channels {
		s.channels[i].Period = 65536
		s.channels[i].next = math.MaxUint64
	}
	return s, nil
}
func (s *NativePaulaDMAState) enabled(n int) bool { return s.DMA&0x200 != 0 && s.DMA&(1<<uint(n)) != 0 }
func (s *NativePaulaDMAState) deferEvent(n int, kind uint8, data uint32) {
	s.events = append(s.events, nativePaulaDMAEvent{s.Clock + 1, n, kind, data})
}
func (s *NativePaulaDMAState) irq(n int) { s.deferEvent(n, 1, 0) }
func (s *NativePaulaDMAState) request(n int, startup bool) {
	if s.DMA&0x200 == 0 {
		return
	}
	c := &s.channels[n]
	if !startup && c.WorkingLength == 1 {
		s.deferEvent(n, 2, 0)
	} else {
		c.Request = true
	}
}
func (s *NativePaulaDMAState) idle(n int) {
	c := &s.channels[n]
	c.State = 0
	c.InterruptCheck = 0
	c.next = math.MaxUint64
}
func (s *NativePaulaDMAState) output(n int, high bool) {
	c := &s.channels[n]
	if high {
		c.Sample = int8(c.OutputWord >> 8)
	} else {
		c.Sample = int8(c.OutputWord)
	}
	if high || s.enabled(n) {
		c.OutputVolume = c.Volume
	}
	c.next = s.Clock + uint64(c.Period)
	if !high && !s.enabled(n) {
		if c.Period == 1 {
			if s.Interrupts&(0x80<<uint(n)) != 0 {
				c.InterruptCheck = 1
			} else {
				c.InterruptCheck = -1
			}
		} else {
			c.State = 0x13
			c.next--
		}
	}
}
func (s *NativePaulaDMAState) settle(n int) {
	c := &s.channels[n]
	on := s.enabled(n)
	switch c.State {
	case 0:
		if on {
			c.State = 1
			c.next = math.MaxUint64
			c.WorkingLength = c.Length
			s.request(n, true)
			c.Restart = true
			if c.InterruptOnWord {
				s.irq(n)
				c.InterruptOnWord = false
			}
		} else if c.DataWritten && s.Interrupts&(0x80<<uint(n)) == 0 {
			// A latched DMA word can arrive just after disabling DMA. The
			// documented CPU-fed path then starts the same two-byte loop.
			s.irq(n)
			c.OutputWord = c.Holding
			c.State = 2
			s.output(n, true)
		}
	case 1:
		if !on {
			s.idle(n)
		} else if c.DataWritten {
			s.irq(n)
			s.request(n, false)
			if c.WorkingLength != 1 {
				c.WorkingLength--
			}
			c.State = 5
		}
	case 5:
		if !on {
			s.idle(n)
		} else if c.DataWritten {
			c.OutputWord = c.Holding
			s.request(n, false)
			c.State = 2
			s.output(n, true)
		}
	}
}
func (s *NativePaulaDMAState) period(n int) {
	c := &s.channels[n]
	switch c.State {
	case 2:
		c.State = 3
		c.InterruptCheck = 0
		s.output(n, false)
	case 0x13:
		c.State = 3
		c.next = s.Clock + 1
		if !s.enabled(n) && s.Interrupts&(0x80<<uint(n)) != 0 {
			c.InterruptCheck = 1
		} else {
			c.InterruptCheck = -1
		}
	case 3:
		if !s.enabled(n) {
			if c.InterruptCheck == 0 && s.Interrupts&(0x80<<uint(n)) != 0 {
				c.InterruptCheck = 1
			}
			s.irq(n)
			if c.InterruptCheck > 0 {
				s.idle(n)
				return
			}
		}
		c.OutputWord = c.Holding
		if s.enabled(n) {
			s.request(n, false)
			if c.InterruptOnWord {
				s.irq(n)
				c.InterruptOnWord = false
			}
		}
		c.State = 2
		s.output(n, true)
	}
}
func (s *NativePaulaDMAState) arrive(n int, data uint32) {
	c := &s.channels[n]
	on := data&0x10000 != 0
	if c.State == 2 || c.State == 3 || c.State == 0x13 {
		if on {
			if c.WorkingLength == 1 {
				c.WorkingLength = c.Length
				if data&0x20000 == 0 {
					c.InterruptOnWord = true
				}
			} else {
				c.WorkingLength--
			}
		} else {
			c.Holding = uint16(data)
		}
	} else {
		c.Holding = uint16(data)
		c.DataWritten = true
		s.settle(n)
	}
	c.DataWritten = false
}
func (s *NativePaulaDMAState) fetch(n int) error {
	c := &s.channels[n]
	address := c.Pointer &^ 1
	word, e := s.ReadWord(address)
	if e != nil {
		return e
	}
	c.Holding = word
	c.DataWritten = true
	data := uint32(word) | 0x10000
	if c.State == 2 || c.State == 3 || c.State == 0x13 {
		data |= 0x20000
		if c.WorkingLength == 1 {
			c.InterruptOnWord = true
		}
	}
	s.deferEvent(n, 3, data)
	c.Pointer = address + 2
	if s.packet&(1<<uint(n*2)) != 0 {
		c.Pointer = c.Location
	}
	return nil
}
func (s *NativePaulaDMAState) tick() error {
	for n := range s.channels {
		if s.channels[n].next == s.Clock {
			s.period(n)
		}
	}
	for i := 0; i < len(s.events); {
		e := s.events[i]
		if e.clock != s.Clock {
			i++
			continue
		}
		s.events = append(s.events[:i], s.events[i+1:]...)
		switch e.kind {
		case 1:
			s.Interrupts |= 0x80 << uint(e.channel)
		case 2:
			s.channels[e.channel].Restart = true
		case 3:
			s.arrive(e.channel, e.data)
		}
	}
	if s.Clock >= s.Config.FirstScanlineClock {
		if s.hpos == s.Config.RequestClock {
			s.packet = 0
			for n := range s.channels {
				c := &s.channels[n]
				if c.Request || c.Restart {
					s.packet |= 2 << uint(n*2)
				}
				if c.Restart {
					s.packet |= 1 << uint(n*2)
				}
				c.Request, c.Restart = false, false
			}
		}
		for n, fetch := range s.Config.FetchClocks {
			if s.hpos == fetch && s.packet&(2<<uint(n*2)) != 0 && s.enabled(n) {
				if e := s.fetch(n); e != nil {
					return e
				}
			}
		}
		s.hpos++
		if s.hpos == s.Config.ScanlineClocks[s.line&1] {
			s.hpos = 0
			s.line++
		}
	}
	return nil
}
func (s *NativePaulaDMAState) AdvanceTo(clock uint64) error {
	if s == nil {
		return fmt.Errorf("native Paula DMA state missing")
	}
	if s.failed != nil {
		return s.failed
	}
	if s.started && clock < s.Clock {
		return fmt.Errorf("native Paula DMA clock cannot reverse")
	}
	if !s.started {
		s.started = true
		s.Clock = 0
		if e := s.tick(); e != nil {
			s.failed = e
			return e
		}
	}
	for s.Clock < clock {
		next := clock
		for _, c := range s.channels {
			if c.next > s.Clock && c.next < next {
				next = c.next
			}
		}
		for _, e := range s.events {
			if e.clock > s.Clock && e.clock < next {
				next = e.clock
			}
		}
		if s.Clock < s.Config.FirstScanlineClock {
			if s.Config.FirstScanlineClock < next {
				next = s.Config.FirstScanlineClock
			}
		} else {
			length := s.Config.ScanlineClocks[s.line&1]
			boundary := s.Clock + uint64(length-s.hpos) + 1
			if boundary < next {
				next = boundary
			}
			for _, phase := range [5]uint16{s.Config.RequestClock, s.Config.FetchClocks[0], s.Config.FetchClocks[1], s.Config.FetchClocks[2], s.Config.FetchClocks[3]} {
				if phase >= s.hpos {
					event := s.Clock + uint64(phase-s.hpos) + 1
					if event < next {
						next = event
					}
				}
			}
			// Every skipped clock has no period, latch or transfer event.
			// Stop at the next line boundary so NTSC's alternating length
			// cannot drift while processing long quiet/zero-period spans.
			s.hpos += uint16(next - s.Clock - 1)
			if s.hpos == length {
				s.hpos = 0
				s.line++
			}
		}
		s.Clock = next
		if e := s.tick(); e != nil {
			s.failed = e
			return e
		}
	}
	return nil
}

// Apply consumes ordered native register writes at the settled current clock.
// Period values are retained exactly, including16 and zero(65536 clocks).
// No PAL/NTSC minimum-period clamp is applied.
func (s *NativePaulaDMAState) Apply(writes []NativeFrameHardwareWrite) error {
	if s == nil {
		return fmt.Errorf("native Paula DMA state missing")
	}
	if s.failed != nil {
		return s.failed
	}
	for _, w := range writes {
		switch w.Address {
		case 0xdff096:
			if w.Value&0x8000 != 0 {
				s.DMA |= uint16(w.Value) & 0x7fff
			} else {
				s.DMA &^= uint16(w.Value)
			}
			for n := range s.channels {
				s.settle(n)
			}
			continue
		case 0xdff09c:
			if w.Value&0x8000 != 0 {
				s.Interrupts |= uint16(w.Value) & 0x7fff
			} else {
				s.Interrupts &^= uint16(w.Value)
			}
			continue
		}
		if w.Address < 0xdff0a0 || w.Address >= 0xdff0e0 {
			continue
		}
		n := (w.Address - 0xdff0a0) / 16
		c := &s.channels[n]
		switch (w.Address - 0xdff0a0) % 16 {
		case 0:
			if w.Width == 4 {
				c.Location = w.Value &^ 1
			} else {
				c.Location = c.Location&0xffff | w.Value<<16
			}
		case 2:
			c.Location = c.Location&0xffff0000 | w.Value&0xfffe
		case 4:
			c.Length = uint16(w.Value)
		case 6:
			c.Period = uint32(uint16(w.Value))
			if c.Period == 0 {
				c.Period = 65536
			}
		case 8:
			c.Volume = uint16(w.Value) & 127
			if c.Volume > 64 {
				c.Volume = 64
			}
		case 10:
			data := w.Value & 0xffff
			if s.enabled(int(n)) {
				c.Holding, c.DataWritten = uint16(data), true
				data |= 0x10000
				if c.State == 2 || c.State == 3 || c.State == 0x13 {
					data |= 0x20000
					if c.WorkingLength == 1 {
						c.InterruptOnWord = true
					}
				}
				s.deferEvent(int(n), 3, data)
			} else {
				s.arrive(int(n), data)
			}
		}
	}
	return nil
}
func (s *NativePaulaDMAState) Snapshot() NativePaulaDMASnapshot {
	out := NativePaulaDMASnapshot{Clock: s.Clock, DMA: s.DMA, Interrupts: s.Interrupts, LatchedRequests: s.packet}
	for n, c := range s.channels {
		out.Channels[n] = c.NativePaulaDMAChannelSnapshot
		out.Channels[n].PeriodRemaining = math.MaxUint32
		if c.next != math.MaxUint64 {
			out.Channels[n].PeriodRemaining = uint32(c.next - s.Clock)
		}
	}
	return out
}
