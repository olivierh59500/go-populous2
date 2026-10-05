package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// The native reference stops at10FC before072E. This test exercises the real
// session-owned physics callbacks and scoped terrain sink to that same cut;
// it does not claim a completed main renderer or initialized audio device.
func TestNativeFrameSessionPhysicsAgainstOriginalMain(t *testing.T) {
	data, err := os.ReadFile("testdata/complete_physics_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []completePhysicsFixture }
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 168 {
		t.Fatal("native session physics corpus incomplete")
	}
	b := testBundle(t)
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := frameContextInitial(f.Input.frameContextInput)
			w := aiFixtureWorld(t, initial)
			w.Landscape = b.Landscapes[f.Input.Landscape]
			w.FollowerWin, err = DecodeFollowerWinRules(b.Executable, w.Landscape)
			if err != nil {
				t.Fatal(err)
			}
			s, err := NewNativeFrameSession(b, f.Input.Landscape, 0x500000, 0x400000)
			if err != nil {
				t.Fatal(err)
			}
			m := s.Presentation.Memory(w.nativeCleanupMemory())
			for at := 0; at < 0xdc2; at++ {
				if err := m.Write8(at, initial[at]); err != nil {
					t.Fatal(err)
				}
			}
			bitmap, err := s.Presentation.BackBuffer()
			if err != nil {
				t.Fatal(err)
			}
			terrain := s.Presentation.Chip[0x408 : 0x408+32000]
			for i := range bitmap {
				bitmap[i] = byte((i*73 + int(f.Input.ScreenSeed)*19) % 256)
				terrain[i] = byte((i*31 + int(f.Input.ScreenSeed)*43) % 256)
			}
			for _, p := range f.Input.Code {
				if p.Address >= 0x185a8 && p.Address < 0x18ada {
					at := p.Address - 0x185a8
					switch p.Width {
					case 1:
						s.Audio.Entries[at] = byte(p.Value)
					case 2:
						binary.BigEndian.PutUint16(s.Audio.Entries[at:], uint16(p.Value))
					case 4:
						binary.BigEndian.PutUint32(s.Audio.Entries[at:], p.Value)
					}
				} else if p.Address >= 0x18426 && p.Address < 0x1842e {
					s.Audio.Channels[(p.Address-0x18426)/2] = uint16(p.Value)
				}
			}
			s.Image.AudioBank = s.Audio.Entries
			// The reference begins with already loaded authoritative raw bytes.
			w.nativeCallDepth++
			if err := s.Begin(w, NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}); err != nil {
				t.Fatal(err)
			}
			defer func() { s.finish(nil); w.nativeCallDepth-- }()
			s.Phase = NativeFrameSessionPhysics
			iterations := f.Input.Frames
			if iterations == 0 {
				iterations = 1
			}
			for iteration := 0; iteration < iterations; iteration++ {
				s.Pass = NativeFramePassState{}
				s.Audio.Entries = s.Image.AudioBank
				callbacks := s.physicsCallbacks(NativeFrameSessionCallbacks{Audio: NativeFrameAudioCallbacks{Command: func(_, _ uint16, input uint32) (uint32, error) { return input, nil }}}, bitmap)
				callbacks.Swap = func(*NativeFrameRegisterContext) (bool, error) { return false, nil }
				done, err := s.Pass.TickFramePass(&s.Frame, callbacks)
				if err != nil || done || s.Pass.Stage != NativeFrameSwap {
					t.Fatal("session missed original before-swap cut", done, err)
				}
				var want *struct {
					Changes                       []nativeHeroChange
					Iteration                     int
					Routine                       uint32
					D                             [8]uint32
					Hash, ScreenHash, TerrainHash string
					Audio                         []byte
					Channels                      [4]uint16
					RNG                           uint32
				}
				for i := range f.Frames {
					if f.Frames[i].Iteration == iteration {
						want = &f.Frames[i]
					}
				}
				if want != nil {
					if s.Frame.D != want.D || !reflect.DeepEqual(s.Audio.Entries[:], want.Audio) || s.Audio.Channels != want.Channels {
						t.Fatal("session fullD/audio differs from original")
					}
					all := aiFixtureWorldBytes(w, initial)
					copy(all[0xeb90:], w.NativeRedrawBytes[:])
					for at := 0; at < 0xdc2; at++ {
						v, err := m.Read8(at)
						if err != nil {
							t.Fatal(err)
						}
						all[at] = v
					}
					for name, pair := range map[string]struct {
						Bytes []byte
						Hash  string
					}{"BSS": {all, want.Hash}, "overview1E": {bitmap, want.ScreenHash}, "terrain22": {terrain, want.TerrainHash}} {
						if got := fmt.Sprintf("%x", sha256.Sum256(pair.Bytes)); got != pair.Hash {
							t.Fatalf("iteration%d native session%s differs: %s/%s", iteration, name, got, pair.Hash)
						}
					}
				}
				s.Image.AudioBank = s.Audio.Entries
				clock, _ := m.Read32(0xf40)
				_ = m.Write32(0xf40, clock+1)
			}
			if f.ErrorPC != 0 || s.Frame.D != f.D {
				t.Fatal("session final native register continuation differs")
			}
		})
	}
}
