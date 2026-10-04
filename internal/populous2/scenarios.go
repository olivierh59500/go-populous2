package populous2

// ScenarioRules decodes the ten options displayed by the native game-options
// screen at CODE:$8c42. Raw preserves the full deity+$66 word, including bits
// not identified by that screen. These properties do not replace the separate
// sixty-byte world command/scenario parameter table.
type ScenarioRules struct {
	Raw                     uint16
	BuildAnywhere           bool
	BuildAnywhereAtSeaLevel bool
	ForbidEnemyTerrain      bool
	ForbidRaise             bool
	ForbidLower             bool
	FatalWater              bool
	HideEnemyOnMap          bool
	DisableRightClickSprog  bool
	HideDisastersOnMap      bool
	ShallowSwamps           bool
}

// DecodeScenarioRules preserves the native bit numbering and polarity. The
// options screen's handlers at CODE:$485e..$48d0 toggle bits zero through nine.
// Build permissions relax presence checks; independent prohibitions still apply.
func DecodeScenarioRules(raw uint16) ScenarioRules {
	return ScenarioRules{
		Raw:                     raw,
		BuildAnywhere:           raw&(1<<0) != 0,
		BuildAnywhereAtSeaLevel: raw&(1<<1) != 0,
		ForbidEnemyTerrain:      raw&(1<<2) != 0,
		ForbidRaise:             raw&(1<<3) != 0,
		ForbidLower:             raw&(1<<4) != 0,
		FatalWater:              raw&(1<<5) != 0,
		HideEnemyOnMap:          raw&(1<<6) != 0,
		DisableRightClickSprog:  raw&(1<<7) != 0,
		HideDisastersOnMap:      raw&(1<<8) != 0,
		ShallowSwamps:           raw&(1<<9) != 0,
	}
}

// ScenarioRules reads the word copied by CODE:$10b76/$10c14 from deity+$66.
// The fifty-eight-byte campaign template starts at deity+$5a, so this is its
// seventh big-endian word, rather than a shared world parameter.
func (p PlayerOptions) ScenarioRules() ScenarioRules {
	return DecodeScenarioRules(p.Parameters[6])
}
