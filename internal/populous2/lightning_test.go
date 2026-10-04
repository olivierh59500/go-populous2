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

type nativeLightningSnapshot struct {
	MarkerReference uint16
	Actors          []struct {
		Slot int
		Raw  string
	}
	RNG         uint32
	TilesSHA256 string
	Tick        int
}

func TestLightningLifecycleAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/lightning_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			Name    string
			XP      uint8
			X, Y    int
			Initial nativeLightningSnapshot
			Trace   []nativeLightningSnapshot
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 5 {
		t.Fatal("native lightning lifecycle catalog incomplete")
	}
	rules, err := DecodeLightningRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			var pool [NativeEffectCapacity]NativeEffectActor
			var state LightningState
			var links [NativeEffectCapacity][2]NativeRecordReference
			var cells NativeOccupancyState
			for i := range cells.Cells {
				cells.Cells[i].Tile = 15
			}
			if fixture.Name == "full-pool" {
				for i := 1; i < len(pool); i++ {
					pool[i].Active = true
					pool[i].Player = 2
				}
			}
			seed := uint32(4311)
			access := NativeRecordAccess{
				Record: func(ref NativeRecordReference) (NativeOccupancyRecord, bool) {
					slot, ok := lightningSlot(ref)
					if !ok {
						return NativeOccupancyRecord{}, false
					}
					a := pool[slot]
					return NativeOccupancyRecord{Next: links[slot][0], Previous: links[slot][1], X: uint16(a.X), Y: uint16(a.Y)}, true
				},
				SetLinks: func(ref, next, prev NativeRecordReference) {
					slot, _ := lightningSlot(ref)
					links[slot] = [2]NativeRecordReference{next, prev}
				},
				SetPosition: func(ref NativeRecordReference, x, y uint16) {
					slot, _ := lightningSlot(ref)
					pool[slot].X, pool[slot].Y = int16(x), int16(y)
				},
			}
			cb := LightningCallbacks{
				Random: func() int {
					if seed == 0 {
						seed = 0x00bc614e
					}
					seed *= 0xbb40e62d
					return int(seed >> 8 & 0x7fff)
				},
				Insert: func(ref NativeRecordReference) error {
					slot, _ := lightningSlot(ref)
					a := pool[slot]
					return cells.Insert(ref, int(a.X)>>8, int(a.Y)>>8, access)
				},
				Move: func(ref NativeRecordReference, x, y uint16) error {
					_, err := cells.Move(ref, x, y, access)
					return err
				},
				Unlink: func(ref NativeRecordReference) error { return cells.Remove(ref, access) },
				Head:   func(x, y int) NativeRecordReference { return cells.Cells[x+y*64].Head },
				Record: func(ref NativeRecordReference) (LightningVictim, bool) {
					slot, ok := lightningSlot(ref)
					if !ok {
						return LightningVictim{}, false
					}
					a := pool[slot]
					return LightningVictim{Kind: a.Kind, Owner: a.Player + 1, State: a.State, Next: links[slot][0]}, true
				},
				SetRecord: func(NativeRecordReference, LightningVictim) {
					t.Fatal("effect-only reference was changed as a follower")
				},
				Burn: func(x, y int) {
					if cells.Cells[x+y*64].Tile == 15 {
						cells.Cells[x+y*64].Tile = 95
					}
				},
			}
			assert := func(want nativeLightningSnapshot) {
				t.Helper()
				for _, reference := range want.Actors {
					slot := reference.Slot
					a := pool[slot]
					var raw [32]byte
					raw[0] = a.Kind
					binary.BigEndian.PutUint16(raw[2:], uint16(links[slot][0]))
					binary.BigEndian.PutUint16(raw[4:], uint16(links[slot][1]))
					binary.BigEndian.PutUint16(raw[6:], uint16(a.X))
					binary.BigEndian.PutUint16(raw[8:], uint16(a.Y))
					binary.BigEndian.PutUint16(raw[10:], uint16(a.Animation))
					if a.Active {
						raw[12] = a.Player + 1
					}
					binary.BigEndian.PutUint16(raw[14:], uint16(a.VX))
					binary.BigEndian.PutUint16(raw[16:], uint16(a.VY))
					raw[18] = a.Speed
					binary.BigEndian.PutUint16(raw[20:], uint16(a.Timer))
					raw[22] = a.State
					binary.BigEndian.PutUint16(raw[24:], uint16(a.Life))
					binary.BigEndian.PutUint16(raw[26:], uint16(state.Word26[slot]))
					binary.BigEndian.PutUint16(raw[28:], uint16(state.Word28[slot]))
					binary.BigEndian.PutUint16(raw[30:], state.RandomWords[slot])
					if got := hex.EncodeToString(raw[:]); got != reference.Raw {
						t.Fatalf("tick%d slot%d: Go%s native%s", want.Tick, slot, got, reference.Raw)
					}
				}
				if uint16(state.Markers[0]) != want.MarkerReference || seed != want.RNG {
					t.Fatalf("tick%d marker/RNG mismatch", want.Tick)
				}
				var tiles [4096]byte
				for i, c := range cells.Cells {
					tiles[i] = c.Tile
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(tiles[:])); got != want.TilesSHA256 {
					t.Fatalf("tick%d scorch map mismatch", want.Tick)
				}
			}
			if ok, err := rules.Place(&state, &pool, 0, fixture.X, fixture.Y, cb); err != nil || !ok {
				t.Fatal(err)
			}
			if _, err := rules.Activate(&state, &pool, 0, fixture.XP, cb); err != nil {
				t.Fatal(err)
			}
			assert(fixture.Initial)
			for _, step := range fixture.Trace {
				if fixture.Name == "center-xp32" && step.Tick == 30 {
					if _, err := rules.Place(&state, &pool, 0, 35, 34, cb); err != nil {
						t.Fatal(err)
					}
				}
				for slot := range pool {
					if err := rules.Tick(&state, &pool, slot, cb); err != nil {
						t.Fatal(err)
					}
				}
				assert(step)
			}
		})
	}
}

func TestLightningBeamAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/lightning_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		BeamFixtures []struct {
			From, To     [2]int16
			Random, Tick uint16
			Segments     [][5]int16
		}
		ProjectionFixtures []struct {
			Marker, Camera [2]int
			BoltHeader     uint8
			Endpoint       [2]int16
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.BeamFixtures) != 16 || len(catalog.ProjectionFixtures) != 12 {
		t.Fatal("native lightning artwork fixtures incomplete")
	}
	for _, fixture := range catalog.BeamFixtures {
		segments := LightningBeamSegments(LightningPoint{fixture.From[0], fixture.From[1]}, LightningPoint{fixture.To[0], fixture.To[1]}, fixture.Random, fixture.Tick)
		if len(fixture.Segments) != len(segments) {
			t.Fatal("native beam segment count differs")
		}
		for index, segment := range segments {
			got := [5]int16{segment.From.X, segment.From.Y, segment.To.X, segment.To.Y, int16(segment.PaletteIndex)}
			if got != fixture.Segments[index] {
				t.Fatalf("beam segment %d: Go %v; native %v", index, got, fixture.Segments[index])
			}
		}
	}
	for _, fixture := range catalog.ProjectionFixtures {
		p := LightningMarkerEndpoint(fixture.Marker[0], fixture.Marker[1], fixture.Camera[0], fixture.Camera[1], fixture.BoltHeader)
		if [2]int16{p.X, p.Y} != fixture.Endpoint {
			t.Fatalf("marker endpoint %+v; native %v", p, fixture.Endpoint)
		}
	}
}
