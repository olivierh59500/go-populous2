package populous2

import (
	"errors"
	"reflect"
	"testing"
)

func TestNativeFungusGenerationCodeWritesPreserveSourcePrefix(t *testing.T) {
	rules, err := DecodeNativeFungusRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	type write struct {
		At, Width int
		Value     uint16
	}
	expected := []write{{0x150bc, 1, 255}, {0x150be, 2, 0x4100}, {0x150bd, 1, 0}, {0x150c0, 2, 0xffff}}
	for fail := 0; fail <= 4; fail++ {
		raw := make([]byte, 0x11280)
		backing := commandFrameBacking(raw)
		_ = backing.Write16(0xc800+6, 0x2000)
		_ = backing.Write16(0xc800+8, 0x2000)
		frame := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
		before := frame.D
		m := nativeWhirlwindMemory{m: backing}
		step := NativeFungusStep{}
		trace := []write{}
		failure := errors.New("explicit CODE port failure")
		record := func(at, width int, value uint16) error {
			if len(trace) < 4 && frame.D != before {
				t.Fatal("14EF0 initialization moved after register setup")
			}
			if len(trace) == fail && fail < 4 {
				return failure
			}
			trace = append(trace, write{at, width, value})
			return nil
		}
		err := rules.generation(0xc800, nativeFungusSurface{m: &m, step: &step, frame: &frame, writeCode8: func(at int, v uint8) error { return record(at, 1, uint16(v)) }, writeCode16: func(at int, v uint16) error { return record(at, 2, v) }})
		if fail < 4 {
			if !errors.Is(err, failure) || frame.D != before || !reflect.DeepEqual(trace, expected[:fail]) {
				t.Fatalf("failed native CODE prefix changed:%d %v %v", fail, trace, err)
			}
		} else if err != nil || !reflect.DeepEqual(trace, expected) {
			t.Fatal("native generation initialization order differs", trace, err)
		}
	}
}
