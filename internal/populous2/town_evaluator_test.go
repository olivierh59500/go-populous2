package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativeTownFixtureCell struct {
	Index                 int
	Header, Tile, Overlay uint8
	Head                  uint16
}

type nativeTownFixtureRecord struct {
	Slot                             int
	Kind, Owner, Stage, State, Flags uint8
	X, Y                             int
	Animation                        uint16
	Population                       uint32
	Next, Previous                   uint16
}

type nativeTownFixtureInput struct {
	Name, Mode                string
	X, Y                      int
	Owner, Stage, DefaultTile uint8
	Clock                     uint16
	Support                   int
	Cells                     []nativeTownFixtureCell
	Records                   []nativeTownFixtureRecord
	Replacement               uint8
}

type nativeTownFixture struct {
	Input                                    nativeTownFixtureInput
	ResultStage                              int16
	StoredStage                              uint8
	GridSHA256, OverlaySHA256, RecordsSHA256 string
	Changes                                  []nativeTownFixtureCell
	Records                                  []struct {
		Slot int
		Raw  string
	}
	RNG uint32
}

type nativeTownTestMemory struct {
	grid     [16384]byte
	overlays [4096]byte
	records  [20800]byte
}

func (m *nativeTownTestMemory) callbackRecord(reference NativeRecordReference) (NativeTownRecord, bool) {
	at := int(reference)
	if at <= 0 || at%52 != 0 || at+52 > len(m.records) {
		return NativeTownRecord{}, false
	}
	b := m.records[at : at+52]
	return NativeTownRecord{Kind: b[0], Stage: b[1], Next: NativeRecordReference(binary.BigEndian.Uint16(b[2:])), X: binary.BigEndian.Uint16(b[6:]), Y: binary.BigEndian.Uint16(b[8:]), Animation: binary.BigEndian.Uint16(b[10:]), Owner: b[12], State: b[22]}, true
}

func (m *nativeTownTestMemory) callbacks() NativeTownCallbacks {
	return NativeTownCallbacks{
		Record: m.callbackRecord,
		SetRecord: func(reference NativeRecordReference, r NativeTownRecord) {
			b := m.records[int(reference) : int(reference)+52]
			b[0], b[1], b[12], b[22] = r.Kind, r.Stage, r.Owner, r.State
			binary.BigEndian.PutUint16(b[2:], uint16(r.Next))
			binary.BigEndian.PutUint16(b[6:], r.X)
			binary.BigEndian.PutUint16(b[8:], r.Y)
			binary.BigEndian.PutUint16(b[10:], r.Animation)
		},
		Head: func(x, y int) NativeRecordReference {
			return NativeRecordReference(binary.BigEndian.Uint16(m.grid[(x+y*64)*4+2:]))
		},
		ReadTile:     func(x, y int) uint8 { return m.grid[(x+y*64)*4+1] },
		WriteTile:    func(x, y int, tile uint8) { m.grid[(x+y*64)*4+1] = tile },
		WriteOverlay: func(x, y int, overlay uint8) { m.overlays[x+y*64] = overlay },
	}
}

func nativeTownInitial(input nativeTownFixtureInput, e NativeTownEvaluator) *nativeTownTestMemory {
	m := &nativeTownTestMemory{}
	for index := range 4096 {
		m.grid[index*4], m.grid[index*4+1] = 0xa8, input.DefaultTile
		m.overlays[index] = 0x77
	}
	actor := m.records[52:104]
	actor[0], actor[1], actor[12], actor[18], actor[22] = 4, input.Stage, input.Owner, 0, 6
	binary.BigEndian.PutUint16(actor[6:], uint16(input.X*256+128))
	binary.BigEndian.PutUint16(actor[8:], uint16(input.Y*256+128))
	binary.BigEndian.PutUint16(actor[10:], 0x744)
	binary.BigEndian.PutUint16(actor[20:], 23)
	binary.BigEndian.PutUint32(actor[26:], 934)
	if input.Support >= 0 {
		origin := uint16(input.X | input.Y<<8)
		for _, offset := range e.Footprint[:min(input.Support, 49)] {
			x, y, inside := nativeTownParcel(origin, offset)
			if inside {
				m.grid[(x+y*64)*4+1] = 15
			}
		}
	}
	if inside(input.X, input.Y) {
		binary.BigEndian.PutUint16(m.grid[(input.X+input.Y*64)*4+2:], 52)
	}
	for _, record := range input.Records {
		b := m.records[record.Slot*52 : (record.Slot+1)*52]
		b[0], b[1], b[12], b[13], b[22] = record.Kind, record.Stage, record.Owner, record.Flags, record.State
		binary.BigEndian.PutUint16(b[2:], record.Next)
		binary.BigEndian.PutUint16(b[4:], record.Previous)
		binary.BigEndian.PutUint16(b[6:], uint16(record.X*256+128))
		binary.BigEndian.PutUint16(b[8:], uint16(record.Y*256+128))
		binary.BigEndian.PutUint16(b[10:], record.Animation)
		binary.BigEndian.PutUint16(b[20:], 31)
		binary.BigEndian.PutUint32(b[26:], record.Population)
	}
	for _, cell := range input.Cells {
		at := cell.Index * 4
		m.grid[at], m.grid[at+1] = cell.Header, cell.Tile
		binary.BigEndian.PutUint16(m.grid[at+2:], cell.Head)
		m.overlays[cell.Index] = cell.Overlay
	}
	return m
}

// Independent relocated 68000 runs compare all map/overlay/follower bytes,
// including pressure, stale overlays, competitor fields, and work counters.
func TestNativeTownEvaluatorAgainstOriginalCompositor(t *testing.T) {
	data, err := os.ReadFile("testdata/town_evaluator_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeTownFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 530 {
		t.Fatal("native settlement catalog is incomplete")
	}
	e, err := DecodeNativeTownEvaluator(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := nativeTownInitial(fixture.Input, e)
			callbacks := m.callbacks()
			stage := uint8(0)
			if fixture.Input.Mode == "clear" {
				err = e.ClearFarms(52, fixture.Input.Replacement, callbacks)
			} else {
				stage, err = e.Evaluate(52, fixture.Input.Clock, callbacks)
			}
			if err != nil {
				t.Fatal(err)
			}
			if fixture.Input.Mode != "clear" && int16(stage) != fixture.ResultStage {
				t.Fatalf("returned stage%d, native%d", stage, fixture.ResultStage)
			}
			if m.records[53] != fixture.StoredStage {
				t.Fatal("stored native stage differs")
			}
			for _, record := range fixture.Records {
				if got := hex.EncodeToString(m.records[record.Slot*52 : (record.Slot+1)*52]); got != record.Raw {
					t.Fatalf("native record%d differs\ngot %s\nwant%s", record.Slot, got, record.Raw)
				}
			}
			for _, hash := range []struct {
				bytes          []byte
				expected, name string
			}{{m.grid[:], fixture.GridSHA256, "map"}, {m.overlays[:], fixture.OverlaySHA256, "overlays"}, {m.records[:], fixture.RecordsSHA256, "follower pool"}} {
				if got := fmt.Sprintf("%x", sha256.Sum256(hash.bytes)); got != hash.expected {
					t.Fatalf("complete native%s hash differs: got%s want%s", hash.name, got, hash.expected)
				}
			}
			if fixture.RNG != 4311 {
				t.Fatal("native settlement compositor consumed RNG")
			}
		})
	}
}

func TestNativeTownOverlayCodesSelectOriginalCompositeImages(t *testing.T) {
	b := testBundle(t)
	e, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if e.Stages[26] != 18 || e.ClearCounters[18] != 48 || e.OverlayPointers[1] != 0x254 || e.OverlayPointers[34] != 0x2ac {
		t.Fatal("native nineteen-stage or overlay table differs")
	}
	for code := 1; code < len(e.OverlayFrames); code++ {
		frame := e.OverlayFrames[code]
		if len(frame.Layers) == 0 || frame.SoundCue < 0 || frame.SoundCue >= 133 {
			t.Fatal("native overlay descriptor missing")
		}
		for _, bank := range b.Sprites {
			for _, layer := range frame.Layers {
				if layer.Sprite < 0 || layer.Sprite >= len(bank) {
					t.Fatal("native settlement layer does not exist in landscape bank")
				}
			}
		}
	}
}
