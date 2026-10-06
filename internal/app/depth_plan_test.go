package app

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"go-populous2/internal/engine"
	"os"
	"reflect"
	"testing"
)

func TestDepthPlanUsesSourceRowsAndMixedTailToHead(t *testing.T) {
	w := &engine.World{}
	old := engine.ActorRef{Kind: engine.ActorScenery, Index: 0}
	newer := engine.ActorRef{Kind: engine.ActorFollower, Index: 1}
	newest := engine.ActorRef{Kind: engine.ActorWall, Index: 0}
	w.Actors.Link(old, 12*256+128, 12*256+128)
	w.Actors.Link(newer, 12*256+128, 12*256+128)
	w.Actors.Link(newest, 12*256+128, 12*256+128)
	plan := sourceRenderPlan(w, 12, 12, nil)
	if len(plan) != 67 || plan[0].Actor.Kind != engine.ActorNone || plan[1].Actor != old || plan[2].Actor != newer || plan[3].Actor != newest {
		t.Fatal("mixed depth order differs", plan[:4])
	}
	if plan[4].X != 13 || plan[4].Y != 12 || plan[11].X != 12 || plan[11].Y != 13 {
		t.Fatal("terrain traversal is not the source row-major walk")
	}
}

func TestDepthPlanDoesNotReorderActorsWhenFamilyChanges(t *testing.T) {
	w := &engine.World{}
	a := engine.ActorRef{Kind: engine.ActorEffect, Index: 0}
	b := engine.ActorRef{Kind: engine.ActorFollower, Index: 2}
	w.Actors.Link(a, 20*256+128, 20*256+128)
	w.Actors.Link(b, 20*256+128, 20*256+128)
	first := sourceRenderPlan(w, 20, 20, nil)
	w.Fire.Columns[0] = engine.FireEffect{Active: true, Frame: 1}
	second := sourceRenderPlan(w, 20, 20, nil)
	if first[1].Actor != a || first[2].Actor != b || second[1].Actor != a || second[2].Actor != b {
		t.Fatal("render plan sorted actors by family instead of retained list membership")
	}
}

// The captured corpus includes independent original row/column and mixed-list
// order. Legacy record widths are confined to this comparison fixture adapter.
func TestDepthPlanMatchesOriginalWorldTraversalCorpus(t *testing.T) {
	data, err := os.ReadFile("../populous2/testdata/render_frame_world_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name    string
				Initial []struct {
					Address, Width int
					Value          uint32
				}
			}
			Actors []uint16
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 340 {
		t.Fatal("original traversal corpus missing", err)
	}
	ref := func(token uint16) engine.ActorRef {
		if token == 0 {
			return engine.ActorRef{}
		}
		at := 0x76c0 + int(int16(token))
		for _, pool := range []struct {
			start, stride, count int
			kind                 engine.ActorKind
		}{
			{0x76c0, 52, 400, engine.ActorFollower}, {0x6bd0, 14, 200, engine.ActorScenery},
			{0x5f50, 16, 200, engine.ActorWall}, {0xc800, 32, 250, engine.ActorEffect}, {0xe74e, 14, 2, engine.ActorMagnet},
		} {
			delta := at - pool.start
			if delta >= 0 && delta%pool.stride == 0 && delta/pool.stride < pool.count {
				return engine.ActorRef{Kind: pool.kind, Index: uint16(delta / pool.stride)}
			}
		}
		t.Fatalf("unknown valid fixture actor %x", token)
		return engine.ActorRef{}
	}
	actorCases := 0
	for _, f := range catalog.Cases {
		raw := make([]byte, 0x11280)
		for _, p := range f.Input.Initial {
			switch p.Width {
			case 1:
				raw[p.Address] = byte(p.Value)
			case 2:
				binary.BigEndian.PutUint16(raw[p.Address:], uint16(p.Value))
			case 4:
				binary.BigEndian.PutUint32(raw[p.Address:], p.Value)
			}
		}
		w := &engine.World{}
		for y := 20; y < 28; y++ {
			for x := 20; x < 28; x++ {
				head := binary.BigEndian.Uint16(raw[0xf46+(x+y*64)*4:])
				w.Actors.Heads[x+y*64] = ref(head)
				for token, steps := head, 0; token != 0 && steps < 1052; steps++ {
					r := ref(token)
					at := 0x76c0 + int(int16(token))
					link := engine.ActorLink{Next: ref(binary.BigEndian.Uint16(raw[at+2:])), Previous: ref(binary.BigEndian.Uint16(raw[at+4:])), Linked: true, X: x*256 + 128, Y: y*256 + 128}
					switch r.Kind {
					case engine.ActorFollower:
						w.Actors.Followers[r.Index] = link
					case engine.ActorScenery:
						w.Actors.Scenery[r.Index] = link
					case engine.ActorWall:
						w.Actors.Walls[r.Index] = link
					case engine.ActorEffect:
						w.Actors.Effects[r.Index] = link
					case engine.ActorMagnet:
						w.Actors.Magnets[r.Index] = link
					}
					token = binary.BigEndian.Uint16(raw[at+2:])
				}
			}
		}
		got, want := []engine.ActorRef{}, []engine.ActorRef{}
		for _, command := range sourceRenderPlan(w, 20, 20, nil) {
			if command.Actor.Kind != engine.ActorNone {
				got = append(got, command.Actor)
			}
		}
		for _, token := range f.Actors {
			want = append(want, ref(token))
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal(fmt.Sprintf("original mixed depth order differs for %s: got%v want%v", f.Input.Name, got, want))
		}
		if len(want) > 0 {
			actorCases++
		}
	}
	if actorCases != 86 {
		t.Fatal("original actor overlap cases missing", actorCases)
	}
}
