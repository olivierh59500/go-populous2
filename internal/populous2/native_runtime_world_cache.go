package populous2

import (
	"bytes"
	"fmt"
)

// RefreshWorldCaches updates host-side decoded rules after actual startup or
// loading. It never generates terrain, actors, deity records or CODE data.
// Invoke while the session is idle and the source's initialized raw memory
// remains authoritative.
func (h *NativeRuntimeHost) RefreshWorldCaches() error {
	if h == nil || h.World == nil || h.Session == nil || h.Memory == nil || h.World.nativeCallDepth != 0 || h.Session.world != nil {
		return fmt.Errorf("native world cache refresh requires idle ownership")
	}
	world, err := h.Memory.BSS.Read16(0xeb46)
	if err != nil {
		return err
	}
	land, err := h.Memory.BSS.Read16(0xeb22)
	if err != nil {
		return err
	}
	mode, err := h.Memory.BSS.Read16(0xeb44)
	if err != nil {
		return err
	}
	profile, err := h.Memory.BSS.Read16(0xeb42)
	if err != nil {
		return err
	}
	if int(world) >= len(h.Bundle.Levels) || int(land) >= len(h.Bundle.Landscapes) {
		return fmt.Errorf("native initialized world/LAND index unavailable")
	}
	data := make([]byte, LandDataSize)
	for i := range data {
		value, err := h.Memory.Code.Read8(0x3365a + i)
		if err != nil {
			return err
		}
		data[i] = value
	}
	if !bytes.Equal(data, h.Bundle.Raw[fmt.Sprintf("land%d.dat", land)]) {
		return fmt.Errorf("native cache refresh requires the actual loaded LAND bank")
	}
	followers, err := DecodeNativeFollowerFrameRules(h.Bundle, int(land))
	if err != nil {
		return err
	}
	landscape, err := DecodeLandscape(data)
	if err != nil {
		return err
	}
	towns, err := DecodeTownRules(h.Bundle.Executable, landscape)
	if err != nil {
		return err
	}
	economy, err := DecodeNativeTownEconomyRules(landscape)
	if err != nil {
		return err
	}
	win, err := DecodeFollowerWinRules(h.Bundle.Executable, landscape)
	if err != nil {
		return err
	}
	h.World.Level = h.Bundle.Levels[world]
	h.World.Level.Terrain = int(land)
	h.World.NativeGameMode, h.World.NativeProfileSide = mode, uint8(profile)
	h.World.Custom = mode != 2
	h.World.Core.Level.Terrain = uint8(land)
	h.World.Landscape = landscape
	h.World.Core.OlympianTowns = towns
	h.World.TownEconomy, h.World.FollowerWin = economy, win
	h.Session.followerRules = followers
	h.Session.land = append(h.Session.land[:0], data...)
	return nil
}
