package app

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestOverviewEffectsRespectLocalDisasterVisibility(t *testing.T) {
	w := &engine.World{}
	w.Fire.Columns[0] = engine.FireEffect{Active: true, X: 20*256 + 128, Y: 18*256 + 128}
	visual := &visualassets.Bundle{}
	visual.Palettes[0][5] = color.RGBA{R: 255, A: 255}
	g := &Game{World: w, Assets: &Assets{Visual: visual}, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	w.Level.Players[0].Scenario.HideDisasters = true
	g.drawOverviewEffects(0)
	if g.framebuffer.RGBAAt(70, 23).A != 0 {
		t.Fatal("hidden disaster was painted on local overview")
	}
	w.Level.Players[0].Scenario.HideDisasters = false
	g.drawOverviewEffects(0)
	if g.framebuffer.RGBAAt(70, 23).R != 255 {
		t.Fatal("visible disaster lost original palette color/coordinates")
	}
}

func TestPrivateOverviewFXTailMatchesOriginalNormalCoordinates(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original data directory for overview-tail comparison")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeCommandRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []bool{false, true} {
		raw := make([]byte, 0x11280)
		binary.BigEndian.PutUint16(raw[0xf0c:], 8)
		binary.BigEndian.PutUint16(raw[0xeb42:], 1)
		if hidden {
			binary.BigEndian.PutUint16(raw[0xeb2c:], 0x100)
		}
		raw[0xc800+12], raw[0xc800+6], raw[0xc800+8], raw[0xc800+22] = 1, 20, 18, 2
		span := func(at, n int) error {
			if at < 0 || at > len(raw)-n {
				return fmt.Errorf("private overview backing invalid")
			}
			return nil
		}
		memory := populous2.FollowerCleanupMemory{Read8: func(at int) (uint8, error) {
			if e := span(at, 1); e != nil {
				return 0, e
			}
			return raw[at], nil
		}, Read16: func(at int) (uint16, error) {
			if e := span(at, 2); e != nil {
				return 0, e
			}
			return binary.BigEndian.Uint16(raw[at:]), nil
		}, Read32: func(at int) (uint32, error) {
			if e := span(at, 4); e != nil {
				return 0, e
			}
			return binary.BigEndian.Uint32(raw[at:]), nil
		}, Write8: func(at int, v uint8) error {
			if e := span(at, 1); e != nil {
				return e
			}
			raw[at] = v
			return nil
		}, Write16: func(at int, v uint16) error {
			if e := span(at, 2); e != nil {
				return e
			}
			binary.BigEndian.PutUint16(raw[at:], v)
			return nil
		}, Write32: func(at int, v uint32) error {
			if e := span(at, 4); e != nil {
				return e
			}
			binary.BigEndian.PutUint32(raw[at:], v)
			return nil
		}}
		frame := populous2.NativeFrameRegisterContext{}
		calls := 0
		if err := rules.TickFrameFX(&frame, populous2.NativeFrameFXCallbacks{Memory: memory, Tick: func(populous2.NativeRecordReference, *populous2.NativeFrameRegisterContext) (populous2.NativeFrameFXStep, error) {
			return populous2.NativeFrameFXStep{Draw: true, Color: 5}, nil
		}, MapPoint: func(c *populous2.NativeFrameRegisterContext) error {
			calls++
			if uint16(c.D[0]) != 70 || uint16(c.D[1]) != 23 || uint16(c.D[2]) != 5 {
				t.Fatal("original overview FX coordinates/color differ")
			}
			return nil
		}}); err != nil {
			t.Fatal(err)
		}
		if hidden && calls != 0 || !hidden && calls != 1 {
			t.Fatal("original local disaster visibility differs", hidden, calls)
		}
	}
}

func TestWaitingMeteorKeepsSourceOverviewMarker(t *testing.T) {
	w := &engine.World{}
	w.Fire.Rain[0] = engine.FireEffect{Active: true, Phase: engine.MeteorWaiting, X: 32 * 256, Y: 32 * 256}
	if _, visible := activeOverviewEffect(w, 0); !visible {
		t.Fatal("waiting meteor lost its source effect marker")
	}
	w.Fire.Rain[0].Phase = engine.MeteorFalling
	if point, visible := activeOverviewEffect(w, 0); !visible || point.X != 32 || point.Y != 32 {
		t.Fatal("active meteor overview point missing")
	}
}

func TestInvisibleEnemyHeadDoesNotHideLocalFollowerBehind(t *testing.T) {
	w := &engine.World{}
	w.Level.Players[0].Scenario.HideEnemy = true
	w.Occupants[20+20*engine.MapSize] = 2
	w.Followers[2] = engine.Follower{Owner: 1, State: engine.Walking, X: 20, Y: 20, NextFollower: 1}
	w.Followers[1] = engine.Follower{Owner: 0, State: engine.Walking, X: 20, Y: 20}
	if f, visible := visibleOverviewFollower(w, 0, 20, 20); !visible || f.Owner != 0 {
		t.Fatal("hidden enemy head suppressed local group behind it")
	}
	if f, visible := visibleOverviewFollower(w, 1, 20, 20); !visible || f.Owner != 1 {
		t.Fatal("opponent observer lost its own marker")
	}
}

func TestOverviewIncludesUnmappedControllersAndSkipsFungusGeneration(t *testing.T) {
	w := &engine.World{}
	w.Water.Basalt[0] = engine.BasaltEffect{Active: true, X: 12, Y: 13}
	if point, shown := activeOverviewEffect(w, 0); !shown || point.X != 12 || point.Y != 13 {
		t.Fatal("unmapped basalt controller missing from overview")
	}
	w.Water.Basalt[0].Active = false
	w.Nature.Fungi[0] = engine.FungusController{Active: true, Collecting: true, MinX: 12, MinY: 13, Period: 10, Wait: 100}
	if _, shown := activeOverviewEffect(w, 0); !shown {
		t.Fatal("collecting fungus marker missing")
	}
	w.Nature.Fungi[0].Collecting = false
	w.Nature.Fungi[0].Wait = 10
	if _, shown := activeOverviewEffect(w, 0); shown {
		t.Fatal("fungus generation pass received source-omitted overview marker")
	}
}
