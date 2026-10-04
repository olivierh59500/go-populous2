package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativeWinInput struct {
	Name                                              string
	WinnerOwner, WinnerKind, WinnerFlags, WinnerStage uint8
	LoserKind, LoserFlags, LoserStage, LoserState     uint8
	WinnerHero, LoserHero                             uint16
	WinnerPopulation, WinnerMana, LoserMana           uint32
	Wins                                              uint16
	FullPool, Blocked, OriginalLoser                  bool
}

type nativeWinFixture struct {
	Input        nativeWinInput
	Hash         string
	ChangedBytes []struct {
		Address int
		Value   uint8
	}
	Actors []struct {
		Slot int
		Raw  string
	}
	WinnerMana, LoserMana uint32
	Wins                  uint16
	RNG                   uint32
}

func winFixtureMemory(input nativeWinInput) *cleanupMemory {
	m := &cleanupMemory{}
	m.putWord(0xf42, 123)
	for i := range 4096 {
		m.putByte(0xf44+i*4, 0xa8)
		m.putByte(0xf44+i*4+1, 15)
		m.putByte(0x4f44+i, 0x77)
	}
	if input.FullPool {
		for i := 1; i < 400; i++ {
			m.putByte(0x76c0+i*52+12, 1)
		}
	}
	if input.Blocked {
		m.putWord(0xdc2, 1)
	}
	for _, record := range []struct {
		address                          int
		owner, kind, flags, stage, state uint8
		hero                             uint16
		population                       uint32
	}{
		{0x76f4, input.WinnerOwner, input.WinnerKind, input.WinnerFlags, input.WinnerStage, 14, input.WinnerHero, input.WinnerPopulation},
		{0x7728, 3 - input.WinnerOwner, input.LoserKind, input.LoserFlags, input.LoserStage, input.LoserState, input.LoserHero, 0},
	} {
		a := record.address
		m.putByte(a, record.kind)
		m.putByte(a+1, record.stage)
		m.putByte(a+12, record.owner)
		m.putByte(a+13, record.flags)
		m.putWord(a+6, 8320)
		m.putWord(a+8, 8320)
		m.putWord(a+10, 0x1c8)
		m.putWord(a+14, 17)
		m.putWord(a+16, 23)
		m.putByte(a+18, 16)
		m.putByte(a+19, 9)
		m.putWord(a+20, 23)
		m.putByte(a+22, record.state)
		_ = m.write32(a+26, record.population)
		m.putWord(a+40, record.hero)
		m.putWord(a+50, 4)
		if record.kind == 4 {
			tile := uint8(47)
			if record.owner == 2 {
				tile = 63
			}
			for y := 29; y <= 35; y++ {
				for x := 29; x <= 35; x++ {
					m.putByte(0xf44+(x+y*64)*4+1, tile)
				}
			}
		}
	}
	m.putWord(0x76f4+2, 104)
	m.putWord(0x7728+4, 52)
	m.putWord(0xf44+(32+32*64)*4+2, 52)
	for owner := uint8(1); owner <= 2; owner++ {
		god := 0xe76a + int(owner)*314
		marker := 0xe740 + int(owner)*14
		reference := uint16(marker - 0x76c0)
		m.putWord(god+10, reference)
		m.putByte(marker+6, 10+owner)
		m.putByte(marker+7, 128)
		m.putByte(marker+8, 10)
		m.putByte(marker+9, 128)
		m.putByte(marker+12, owner)
		m.putWord(0xf44+(int(10+owner)+10*64)*4+2, reference)
	}
	wg, lg := 0xe76a+int(input.WinnerOwner)*314, 0xe76a+int(3-input.WinnerOwner)*314
	_ = m.write32(wg, input.WinnerMana)
	_ = m.write32(lg, input.LoserMana)
	m.putWord(wg+0x48, input.Wins)
	if input.WinnerFlags&1 != 0 {
		m.putWord(wg+8, 52)
	}
	if input.LoserFlags&1 != 0 {
		m.putWord(lg+8, 104)
	}
	return m
}

func winEntryRead(m *cleanupMemory, reference NativeRecordReference) (FollowerEntryActor, error) {
	var temporary entryFixtureMemory
	at := cleanupRecordAddress(reference)
	if err := m.runtime().Read(at, temporary.bytes[at:at+52]); err != nil {
		return FollowerEntryActor{}, err
	}
	return temporary.read(reference)
}

func winEntryWrite(m *cleanupMemory, reference NativeRecordReference, actor FollowerEntryActor) error {
	var temporary entryFixtureMemory
	at := cleanupRecordAddress(reference)
	if err := m.runtime().Read(at, temporary.bytes[at:at+52]); err != nil {
		return err
	}
	if err := temporary.write(reference, actor); err != nil {
		return err
	}
	_, err := m.runtime().Patch(at, temporary.bytes[at:at+52])
	return err
}

func winTownCallbacks(m *cleanupMemory) NativeTownCallbacks {
	return NativeTownCallbacks{
		Record: func(ref NativeRecordReference) (NativeTownRecord, bool) {
			a, err := winEntryRead(m, ref)
			return NativeTownRecord{Kind: a.Motion.Kind, Owner: a.Owner, Stage: a.Byte1, State: a.Motion.State, Animation: uint16(a.Motion.Animation), Next: NativeRecordReference(a.Motion.Next), X: uint16(a.Motion.X), Y: uint16(a.Motion.Y)}, err == nil
		},
		SetRecord: func(ref NativeRecordReference, r NativeTownRecord) {
			a, err := winEntryRead(m, ref)
			if err != nil {
				m.err = err
				return
			}
			a.Motion.Kind, a.Byte1, a.Motion.State, a.Motion.Animation = r.Kind, r.Stage, r.State, int(r.Animation)
			m.err = winEntryWrite(m, ref, a)
		},
		Head:         func(x, y int) NativeRecordReference { return NativeRecordReference(m.word(0xf44 + (x+y*64)*4 + 2)) },
		ReadTile:     func(x, y int) uint8 { return m.byte(0xf44 + (x+y*64)*4 + 1) },
		WriteTile:    func(x, y int, tile uint8) { m.putByte(0xf44+(x+y*64)*4+1, tile) },
		WriteOverlay: func(x, y int, code uint8) { m.putByte(0x4f44+x+y*64, code) },
	}
}

// Native fixtures run all of $1298c, including actual town and cleanup
// callbacks. Complete BSS comparison catches callback ordering and aliases;
// neither rewards nor town/hero effects are substituted with generic hooks.
func TestFollowerWinAgainstCompleteNativeResolution(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_win_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeWinFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 87 {
		t.Fatal("native winner catalog incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeFollowerWinRules(b.Executable, b.Landscapes[0])
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := DecodeTownCombatRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := winFixtureMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			clear := func(ref NativeRecordReference, tile uint8) error {
				return town.ClearFarms(ref, tile, winTownCallbacks(m))
			}
			cleanup := func(ref NativeRecordReference, mode uint16) error {
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: clear})
				return err
			}
			townCB := TownCombatCallbacks{Read: func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m, ref) }, Write: func(ref NativeRecordReference, a FollowerEntryActor) error { return winEntryWrite(m, ref, a) }, ClearFarms: clear, Cleanup: cleanup, EvaluateTown: func(ref NativeRecordReference) (int, error) {
				stage, err := town.Evaluate(ref, 123, winTownCallbacks(m))
				return int(stage), err
			}}
			cb := FollowerWinCallbacks{Memory: memory, Cleanup: cleanup, ClearFarms: clear, ReformTown: func(ref, original NativeRecordReference) error {
				_, err := combat.Reform(ref, original, 123, townCB)
				return err
			}, DestroyTown: func(ref NativeRecordReference) error { _, err := combat.Destroy(ref, townCB); return err }, PoolBlocked: func() bool { return m.word(0xdc2) != 0 }, Insert: m.insert}
			original := NativeRecordReference(52)
			if fixture.Input.OriginalLoser {
				original = 104
			}
			step, err := rules.Win(52, 104, original, cb)
			if err != nil {
				t.Fatal(err)
			}
			if m.err != nil {
				t.Fatal(m.err)
			}
			var image [NativeRuntimeImageEnd]byte
			copy(image[:NativeRecordImageStart], m.lower[:])
			copy(image[NativeRecordImageStart:NativeMagnetImageStart], m.records.Bytes[:])
			copy(image[NativeMagnetImageStart:], m.globals.Bytes[:])
			if got := fmt.Sprintf("%x", sha256.Sum256(image[:])); got != fixture.Hash {
				t.Fatalf("complete native BSS differs: got%s want%s (reward%d states%x/%x)", got, fixture.Hash, step.Reward, step.WinnerState, step.LoserState)
			}
			for _, change := range fixture.ChangedBytes {
				if image[change.Address] != change.Value {
					t.Fatalf("native byte%x differs", change.Address)
				}
			}
			for _, actor := range fixture.Actors {
				at := 0x76c0 + actor.Slot*52
				if got := hex.EncodeToString(image[at : at+52]); got != actor.Raw {
					t.Fatalf("native follower%d differs", actor.Slot)
				}
			}
			wg, lg := 0xe76a+int(fixture.Input.WinnerOwner)*314, 0xe76a+int(3-fixture.Input.WinnerOwner)*314
			wm, _ := m.read32(wg)
			lm, _ := m.read32(lg)
			if wm != fixture.WinnerMana || lm != fixture.LoserMana || m.word(wg+0x48) != fixture.Wins || fixture.RNG != 4311 {
				t.Fatal("native mana/victories or untouched RNG differs")
			}
		})
	}
}
