package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"

	legacy "go-populous2/internal/legacy"
)

// ImportNativeGAM rebuilds typed views from the actual saved BSS image. The
// original file remains authoritative: fields are copied in bulk before any
// hydration, and membership follows saved mixed map chains rather than owner
// or population. Go control pointers retain the file's follower-relative form.
func ImportNativeGAM(bundle *Bundle, data []byte) (*World, error) {
	if bundle == nil {
		return nil, fmt.Errorf("native GAM asset bundle missing")
	}
	image, err := DecodeNativeGAM(data)
	if err != nil {
		return nil, err
	}
	if err := ValidateNativeGAMLayout(bundle.Executable); err != nil {
		return nil, err
	}
	session, err := image.Session()
	if err != nil {
		return nil, err
	}
	if session.World >= uint16(len(bundle.Levels)) || session.Landscape >= uint16(len(bundle.Landscapes)) || session.ProfileSide < 1 || session.ProfileSide > 2 {
		return nil, fmt.Errorf("native GAM session world/landscape/profile outside supported ranges")
	}
	switch session.GameMode {
	case 2, 4, 6, 8, 10:
	default:
		return nil, fmt.Errorf("native GAM game mode outside supported modes")
	}
	w, err := NewWorld(bundle, int(session.World), session.GameMode != 2)
	if err != nil {
		return nil, err
	}
	if session.Landscape != uint16(w.Level.Terrain) {
		w.Level.Terrain = int(session.Landscape)
		w.Core.Level.Terrain = uint8(session.Landscape)
		w.Landscape = bundle.Landscapes[session.Landscape]
		w.Core.OlympianTowns, err = DecodeTownRules(bundle.Executable, w.Landscape)
		if err != nil {
			return nil, err
		}
		w.TownEconomy, err = DecodeNativeTownEconomyRules(w.Landscape)
		if err != nil {
			return nil, err
		}
		w.FollowerWin, err = DecodeFollowerWinRules(bundle.Executable, w.Landscape)
		if err != nil {
			return nil, err
		}
	}
	view := func(start, end int) []byte { return image.Bytes[start-NativeGAMStart : end-NativeGAMStart] }
	copy(w.NativeControlBytes[NativeGAMStart-0xdc4:], view(NativeGAMStart, 0xf44))
	// DOS Read touches only DD6..EB48. Unsaved scratch-command prefix and
	// command transports/render-queue suffix keep the initialized session data.
	copy(w.NativeCommandBytes[:], view(0xeb18, NativeGAMEnd))
	copy(w.NativeViewBytes[:], view(0x5f44, 0x5f50))
	copy(w.NativeOverlays[:], view(0x4f44, 0x5f44))
	copy(w.RecordImage.Bytes[:], view(NativeRecordImageStart, NativeRecordImageEnd))
	copy(w.NativeGlobals.Bytes[:], view(NativeMagnetImageStart, NativeRuntimeImageEnd))
	w.NativeClock, w.NativeGameMode, w.NativeProfileSide = session.Clock, session.GameMode, uint8(session.ProfileSide)
	w.NativeRaiseEnabled, _ = image.Read16(0xf12)
	w.Level.RandomSeed, w.Core.GameTurn = session.Seed, int(session.Clock)
	w.Core.SetRandomState(session.Random)
	w.Random = uint16(session.Random)
	w.NativeSelected = NativeRecordReference(binary.BigEndian.Uint32(view(0xf36, 0xf3a)))
	w.effectViewX, w.effectViewY = int(int16(binary.BigEndian.Uint16(w.NativeViewBytes[:]))), int(int16(binary.BigEndian.Uint16(w.NativeViewBytes[2:])))
	w.Occupancy = NativeWorldOccupancy{}
	for pos := range w.Occupancy.Grid.Cells {
		a := view(0xf44+pos*4, 0xf48+pos*4)
		w.Occupancy.Grid.Cells[pos] = NativeOccupancyCell{Header: a[0], Tile: a[1], Head: NativeRecordReference(binary.BigEndian.Uint16(a[2:]))}
	}
	clear(w.Core.Alt[:])
	clear(w.Core.MapWho[:])
	clear(w.Core.MapBk2[:])
	clear(w.Marks[:])
	geometry := bundle.BasaltRules.Geometry
	for y := range 64 {
		for x := range 64 {
			pos := x + y*64
			cell := w.Occupancy.Grid.Cells[pos]
			base := int(cell.Header & 7)
			shape := geometry[cell.Tile] & 15
			w.Core.Alt[x+y*65] = base + int(shape&1)
			if x == 63 {
				w.Core.Alt[x+1+y*65] = base + int(shape>>1&1)
			}
			if y == 63 {
				w.Core.Alt[x+(y+1)*65] = base + int(shape>>3&1)
			}
			if x == 63 && y == 63 {
				w.Core.Alt[x+1+(y+1)*65] = base + int(shape>>2&1)
			}
			w.writeNativeTownTile(x, y, cell.Tile)
		}
	}
	// Discard generated actors before reading the saved owners and records.
	w.Core.Peeps = nil
	w.NativeFollowers = [legacy.MaxPeeps]NativeFollower{}
	w.NativeEntries = [legacy.MaxPeeps]NativeFollowerEntry{}
	w.Heroes = [legacy.MaxPeeps]Hero{}
	w.LightningVictims = [legacy.MaxPeeps]NativeLightningFollower{}
	w.FlameDeaths = nil
	clear(w.flameDeathIndex[:])
	clear(w.captiveIndex[:])
	w.NativeEnvironment = [NativeEffectCapacity]NativeEnvironmentController{}
	w.hydrateNativeRuntimeGraph()
	seen := make(map[NativeRecordReference]bool)
	for pos, cell := range w.Occupancy.Grid.Cells {
		previous := NativeRecordReference(0)
		for ref := cell.Head; ref != 0; {
			entry, ok := w.Occupancy.entry(ref)
			if !ok || seen[ref] {
				return nil, fmt.Errorf("native GAM mixed map chain has unsupported/cyclic reference %04x", uint16(ref))
			}
			seen[ref] = true
			if entry.Record.Previous != previous {
				return nil, fmt.Errorf("native GAM map chain previous reference differs at %04x", uint16(ref))
			}
			at, err := nativeOccupancyIndex(entry.Record.X, entry.Record.Y)
			if err != nil || at != pos {
				return nil, fmt.Errorf("native GAM mapped record %04x has a different parcel", uint16(ref))
			}
			entry.Linked = true
			previous, ref = ref, entry.Record.Next
		}
	}
	for player := range 2 {
		god, _ := NativeDeityAddress(uint8(player + 1))
		for n := range w.Level.Players[player].Parameters {
			w.Level.Players[player].Parameters[n], _ = image.Read16(god + 0x5a + n*2)
		}
		for n := range w.Level.Players[player].Powers {
			value, _ := image.Read8(god + 0x70 + n)
			w.Level.Players[player].Powers[n] = int8(value) > 0
		}
		w.Rules[player] = DecodeScenarioRules(session.Rules[player])
		for element := range 6 {
			w.Experience[player][element], _ = image.Read8(god + 0x52 + element)
		}
		mode, _ := image.Read16(god + 12)
		switch NativeFollowerMode(mode) {
		case NativeFollowerMagnet:
			w.Core.Magnets[player].Flags = legacy.MagnetMode
		case NativeFollowerJoin:
			w.Core.Magnets[player].Flags = legacy.JoinMode
		case NativeFollowerFight:
			w.Core.Magnets[player].Flags = legacy.FightMode
		default:
			w.Core.Magnets[player].Flags = legacy.SettleMode
		}
	}
	name := session.Name[:]
	if end := bytes.IndexByte(name, 0); end >= 0 {
		name = name[:end]
	}
	w.Deity = NewDeity(string(name))
	w.Deity.Experience = w.Experience[session.ProfileSide-1]
	god, _ := NativeDeityAddress(uint8(session.ProfileSide))
	w.Deity.Bolts, _ = image.Read16(god + 0x58)
	for part := range 3 {
		w.Deity.FaceParts[part], _ = image.Read8(god + 0x4e + part)
		if w.Deity.FaceParts[part] > 7 {
			return nil, fmt.Errorf("native GAM face part outside decoded bank")
		}
	}
	w.hydrateNativeRuntimeRecords()
	for index := range w.NativeEffects {
		ref := nativeActorReference(NativeEffectPool, index)
		owner, _ := w.RecordImage.Read8(ref, 12)
		if owner == 0 {
			continue
		}
		state, _ := w.RecordImage.Read8(ref, 22)
		switch state {
		case 0x22, 0x24, 0x26:
			w.NativeEnvironment[index] = NativeEnvironmentQuake
		case 0x28, 0x2a:
			w.NativeEnvironment[index] = NativeEnvironmentTsunami
		case 0x2c, 0x2e:
			w.NativeEnvironment[index] = NativeEnvironmentStorm
		case 0x30, 0x32:
			w.NativeEnvironment[index] = NativeEnvironmentVolcano
		case 0x34, 0x36:
			w.NativeEnvironment[index] = NativeEnvironmentLava
		case 0x3c:
			w.NativeEnvironment[index] = NativeEnvironmentHurricane
		case 0x1c, 0x1e, 0x20:
			w.NativeEnvironment[index] = NativeEnvironmentFireRain
		}
	}
	for index := range w.Core.Peeps {
		a := w.NativeEntries[index].Actor
		if a.Owner == 0 {
			continue
		}
		if a.Motion.State == 0x1c || a.Motion.State == 0x1e || a.Motion.State == 0x20 || a.Motion.State == 0x22 {
			w.LightningVictims[index] = NativeLightningFollower{Active: true, Victim: LightningVictim{Kind: a.Motion.Kind, Owner: a.Owner, Flags: a.Motion.Flags, State: a.Motion.State, Next: NativeRecordReference(a.Motion.Next), HeroType: a.Hero40, Animation: a.Motion.Animation, Population: a.Motion.Population, EffectReference: NativeRecordReference(a.Contact30)}}
		}
	}
	w.bindScenarioRuntime()
	// The saved live attrition long may differ from the template word. Native
	// follower handlers read God+20; do not rewrite that long from cached P5.
	w.Core.FollowerAttrition = func(player int, _ bool) int {
		if player < 0 || player > 1 {
			return 0
		}
		god, _ := NativeDeityAddress(uint8(player + 1))
		value, _ := w.runtimeMemory().Read32(god + 20)
		return int(value)
	}
	return w, nil
}

// ExportNativeGAM reads the complete authoritative raw memory without
// regenerating reserved records or inferred membership. Go retains the two
// selected-pointer longs as follower-relative offsets, so the export base is0.
func (w *World) ExportNativeGAM() ([]byte, error) {
	if w == nil || w.Core == nil {
		return nil, fmt.Errorf("native GAM world missing")
	}
	w.syncNativeRuntimeBridge()
	w.refreshNativeRecordImage()
	image, err := CaptureNativeGAM(w.nativeCleanupMemory(), 0)
	if err != nil {
		return nil, err
	}
	// These scalar views are also changed by the Go world/menus. The file's
	// opaque session bytes survive; only their actual native fields are patched.
	for _, field := range []struct {
		address int
		value   uint16
	}{{0xeb22, uint16(w.Level.Terrain)}, {0xeb2c, w.Rules[0].Raw}, {0xeb2e, w.Rules[1].Raw}, {0xeb42, uint16(w.NativeProfileSide)}, {0xeb44, w.NativeGameMode}, {0xeb46, uint16(w.Level.Number)}} {
		if err := image.Write16(field.address, field.value); err != nil {
			return nil, err
		}
	}
	if err := image.Write32(0xeb24, w.Level.RandomSeed); err != nil {
		return nil, err
	}
	if err := image.Write32(0xeb28, w.Core.RandomState()); err != nil {
		return nil, err
	}
	god, _ := NativeDeityAddress(w.NativeProfileSide)
	for part, value := range w.Deity.FaceParts {
		if err := image.Write8(god+0x4e+part, value); err != nil {
			return nil, err
		}
	}
	name := image.Bytes[0xeb30-NativeGAMStart : 0xeb40-NativeGAMStart]
	old := name
	if end := bytes.IndexByte(old, 0); end >= 0 {
		old = old[:end]
	}
	if string(old) != w.Deity.Name {
		if len(w.Deity.Name) >= len(name) {
			return nil, fmt.Errorf("native GAM deity name exceeds its NUL-terminated16-byte field")
		}
		copy(name, []byte(w.Deity.Name))
		name[len(w.Deity.Name)] = 0
	}
	return image.MarshalBinary()
}
