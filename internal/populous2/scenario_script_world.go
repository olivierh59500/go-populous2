package populous2

import "fmt"

// tickNativeScenarioScript runs after the native scenery pass, at the original
// $17e9a position. It executes at most one due event before normal commands.
func (w *World) tickNativeScenarioScript() error {
	return w.runNativeFollowerCall(func() error {
		if err := w.nativeCleanupMemory().Write16(0xeb44, w.NativeGameMode); err != nil {
			return err
		}
		_, err := TickScenarioScript(ScenarioScriptCallbacks{Memory: w.nativeCleanupMemory(), Execute: w.executeNativeScriptCommand})
		return err
	})
}

// The original conquest resources use these command bodies, including six
// neutral inventions at90..100. Neutral events bypass player affordability,
// availability and weighted debit; their raw owner/register contexts survive.
func (w *World) executeNativeScriptCommand(caller int) error {
	m := w.nativeCleanupMemory()
	owner, err := m.Read8(caller)
	if err != nil {
		return err
	}
	command, err := m.Read8(caller + 1)
	if err != nil {
		return err
	}
	x, err := m.Read8(caller + 2)
	if err != nil {
		return err
	}
	y, err := m.Read8(caller + 3)
	if err != nil {
		return err
	}
	switch command {
	case 0:
		return nil
	case 6:
		_, err = w.PrimitiveCreators.CreateFireColumn(uint16(owner), x, y, w.nativePrimitiveCallbacks())
	case 22:
		_, err = w.PrimitiveCreators.CreateWhirlwind(uint16(owner), x, y, w.nativePrimitiveCallbacks())
	case 40:
		strength := w.EarthquakeRules.BaseStrength
		if owner <= 2 {
			xp, e := m.Read8(primitiveDeityAddress(uint16(owner)) + 0x54)
			if e != nil {
				return e
			}
			strength += uint16(xp)
		}
		step, e := w.EarthquakeRules.Create(owner, x&63, y, w.EarthquakeRules.Directions[(x>>6)&3], strength, 0, w.quakeCallbacks())
		err = e
		if step.Allocated && step.Active {
			w.NativeEnvironment[(step.Address-0xc800)/32] = NativeEnvironmentQuake
		}
	case 46:
		_, err = w.ForestNative.Cast(owner, x, y, w.nativeForestCallbacks())
	case 62:
		ref, created, e := w.VolcanoRules.Create(owner, x, y, w.volcanoCallbacks())
		err = e
		if created {
			loc, _ := LocateNativeRecord(ref)
			w.NativeEnvironment[loc.Index] = NativeEnvironmentVolcano
		}
	case 64:
		_, err = w.StormRules.Create(uint16(owner), x, y, caller, w.stormCallbacks())
	case 38:
		step, e := w.FireRainRules.Create(uint16(owner), x, y, w.fireRainCallbacks())
		err = e
		for _, ref := range step.References {
			loc, _ := LocateNativeRecord(ref)
			w.NativeEnvironment[loc.Index] = NativeEnvironmentFireRain
		}
	case 90, 92, 94, 96, 98, 100:
		_, err = CreateNativeNeutral(FollowerCleanupRegisters{D0: uint32(x), D1: uint32(y), D2: uint32(command - 88)}, NativeNeutralCallbacks{Memory: m, Insert: w.nativeRuntimeInsert})
	default:
		return fmt.Errorf("native scenario command%d requires its original body", command)
	}
	return err
}
