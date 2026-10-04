package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"
)

func TestSceneryAgainstOriginalInitialization(t *testing.T) {
	b := testBundle(t)
	for _, ref := range []struct {
		seed, rng uint32
		count     int
		hash      string
	}{
		{4311, 0x47813c0f, 11, "7a4adb6dc34eca0293f67250a53cc9fcab7783f9719144f1424f1acf7ee53532"},
		{5038, 0x9a2d8afe, 36, "0bd7b59d6ca8c75ef241b41e4aaed0f69efe196e422497ebadcb0446b5ad5d96"},
		{65536, 0xe2e50000, 2, "40436317e86604a0dda81492b8a154d2cf73b29e56663d93787591f3cb3072ea"},
		{777, 0x2fe72d25, 65, "0d63b96800ec7c9969df0edadd67a1e21b3b0182ecec9dd355679050dc58b74a"},
	} {
		w, err := NewWorld(b, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		w.Core.GenerateOlympianTerrain(ref.seed, b.HillParameters)
		w.Scenery = [SceneryCapacity]SceneryActor{}
		w.rebuildSceneryIndex()
		w.initializeActorGraph()
		w.initializeScenery()
		var packed bytes.Buffer
		count := 0
		for i, a := range w.Scenery {
			if !a.Active {
				continue
			}
			count++
			binary.Write(&packed, binary.BigEndian, uint16(i))
			packed.Write([]byte{byte(a.Kind), byte(a.Age), byte(a.X), byte(a.Y)})
			binary.Write(&packed, binary.BigEndian, uint16(a.Animation))
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(packed.Bytes())); got != ref.hash || count != ref.count || w.Core.Snapshot().RNG != ref.rng {
			t.Fatalf("native scenery differs for seed %d: count=%d hash=%s rng=%08x", ref.seed, count, got, w.Core.Snapshot().RNG)
		}
	}
}

func TestScenerySaveBurialAndPoolReuse(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	data := encodeSnapshot(t, w)
	restored, err := ReadSave(testBundle(t), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for range 40 {
		w.Tick()
		restored.Tick()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("save changed scenery continuation")
	}
	w = flatGroundWorld(t)
	w.Scenery = [SceneryCapacity]SceneryActor{}
	w.rebuildSceneryIndex()
	if !w.allocateScenery(SceneryBoulder, 20, 20, w.SceneryBank.Boulders.Animations[0], 0) {
		t.Fatal("boulder allocation failed")
	}
	for i := range w.Core.Alt {
		w.Core.Alt[i] = 0
	}
	for range 24 {
		w.tickScenery()
	}
	if w.Scenery[0].Active || w.sceneryAt(20+20*64) >= 0 {
		t.Fatal("submerged scenery never released its slot")
	}
	if !w.allocateScenery(SceneryTree, 22, 22, w.SceneryBank.Trees.Animations[0], 24) || !w.Scenery[0].Active {
		t.Fatal("first expired scenery slot not reused")
	}
}

func TestTreesDoNotInvalidateSettlementSupport(t *testing.T) {
	w := flatGroundWorld(t)
	w.Scenery = [SceneryCapacity]SceneryActor{}
	w.rebuildSceneryIndex()
	if !w.allocateScenery(SceneryTree, 32, 32, w.SceneryBank.Trees.Animations[0], 24) {
		t.Fatal("tree allocation failed")
	}
	if w.Core.HabitatBlocked(32 + 32*64) {
		t.Fatal("tree wrongly blocks native town support")
	}
	w.removeScenery(0)
	if !w.allocateScenery(SceneryBoulder, 32, 32, w.SceneryBank.Boulders.Animations[0], 24) || !w.Core.HabitatBlocked(32+32*64) {
		t.Fatal("boulder did not block support")
	}
}
