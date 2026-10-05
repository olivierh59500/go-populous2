package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeFrameQuakeRegistersAgainstFullFXPass(t *testing.T) {
	data, e := os.ReadFile("testdata/frame_quake_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []frameContextFixture }
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, f := range catalog.Cases {
		if len(f.Input.Stages) != 1 || f.Input.Stages[0] != 0x1482e {
			continue
		}
		count++
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := frameContextInitial(f.Input)
			w := aiFixtureWorld(t, initial)
			w.nativeCallDepth++
			c := NativeFrameRegisterContext{D: f.Input.D}
			audio, e := DecodeNativeFrameAudioState(testBundle(t).Executable)
			if e != nil {
				t.Fatal(e)
			}
			e = w.tickNativeFrameFX(&c, NativeFrameWorldBindings{Audio: &audio})
			w.nativeCallDepth--
			if e != nil {
				t.Fatal(e)
			}
			want := f.Frames[0]
			if !reflect.DeepEqual(audio.Entries[:], want.Audio) || audio.Channels != want.Channels {
				t.Fatal("native Whirlpool audio descriptor differs")
			}
			if c.D != want.D {
				t.Fatalf("FX registers got%08x native%08x", c.D, want.D)
			}
			b := aiFixtureWorldBytes(w, initial)
			copy(b[0xeb90:], w.NativeRedrawBytes[:])
			if fmt.Sprintf("%x", sha256.Sum256(b)) != want.Hash {
				t.Fatal("FX complete BSS/RNG differs")
			}
		})
	}
	if count != 984 {
		t.Fatalf("full whirlwind frame coverage%d", count)
	}
}
