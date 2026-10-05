package populous2

import (
	"encoding/binary"
	"fmt"
)

// NativeFollowerFrameRules collects every active source state family. It
// retains one LAND bank; the enclosing frame owns clock, drawing and audio.
type NativeFollowerFrameRules struct {
	pass        NativeFollowerPassRules
	aftermath   NativeFollowerAftermathFrameRules
	motion      FollowerMotionFrameRules
	entry       NativeFollowerEntryFrameRules
	combat      FollowerCombatFrameRules
	town        *NativeFollowerTownFrameRules
	decision    FollowerDecisionFrameRules
	hero        NativeFollowerHeroFrameRules
	magnet      NativeFollowerMagnetFrameRules
	terrain     NativeFollowerTerrainFrameRules
	waitContact NativeFollowerWaitContactFrameRules
	siege       NativeFollowerSiegeFrameRules
	retained    NativeFollowerRetainedFrameRules
	neutral     NativeFollowerNeutralFrameRules
	landscape   Landscape
	initial     NativeFollowerFrameState
}

type NativeFollowerFrameState struct {
	Pass NativeFollowerPassState
	Town NativeTownFrameState
}

type NativeFollowerFrameOutputs struct {
	MapPoint func(uint16, *NativeFrameRegisterContext) error
	Result   func(uint16, *NativeFrameRegisterContext) error
	Commands NativeCommandWorldBindings
}

func DecodeNativeFollowerFrameRules(bundle *Bundle, landIndex int) (*NativeFollowerFrameRules, error) {
	if bundle == nil || bundle.Executable == nil || landIndex < 0 || landIndex >= len(bundle.Landscapes) {
		return nil, fmt.Errorf("native follower frame resources/LAND missing")
	}
	r := &NativeFollowerFrameRules{landscape: bundle.Landscapes[landIndex]}
	exe := bundle.Executable
	var err error
	if r.pass, err = DecodeNativeFollowerPassRules(exe); err != nil {
		return nil, err
	}
	if r.aftermath, err = DecodeNativeFollowerAftermathFrameRules(exe); err != nil {
		return nil, err
	}
	if r.motion, err = DecodeFollowerMotionFrameRules(exe); err != nil {
		return nil, err
	}
	if r.entry, err = DecodeNativeFollowerEntryFrameRules(exe); err != nil {
		return nil, err
	}
	if r.combat, err = DecodeFollowerCombatFrameRules(exe); err != nil {
		return nil, err
	}
	if r.town, err = DecodeNativeFollowerTownFrameRules(exe, bundle.Raw[fmt.Sprintf("land%d.dat", landIndex)]); err != nil {
		return nil, err
	}
	if r.decision, err = DecodeFollowerDecisionFrameRules(exe); err != nil {
		return nil, err
	}
	if r.hero, err = DecodeNativeFollowerHeroFrameRules(exe); err != nil {
		return nil, err
	}
	if r.magnet, err = DecodeNativeFollowerMagnetFrameRules(exe); err != nil {
		return nil, err
	}
	if r.terrain, err = DecodeNativeFollowerTerrainFrameRules(exe); err != nil {
		return nil, err
	}
	if r.waitContact, err = DecodeNativeFollowerWaitContactFrameRules(exe); err != nil {
		return nil, err
	}
	if r.siege, err = DecodeNativeFollowerSiegeFrameRules(exe); err != nil {
		return nil, err
	}
	if r.retained, err = DecodeNativeFollowerRetainedFrameRules(exe); err != nil {
		return nil, err
	}
	if r.neutral, err = DecodeNativeFollowerNeutralFrameRules(exe); err != nil {
		return nil, err
	}
	code := exe.Hunks[0].Data
	r.initial.Town.Property13550 = binary.BigEndian.Uint16(code[0x13550:])
	for i := range r.initial.Town.Scratch136E8 {
		r.initial.Town.Scratch136E8[i] = binary.BigEndian.Uint16(code[0x136e8+i*2:])
	}
	return r, nil
}

// NewState creates a distinct mutable CODE cache/scratch for one session.
func (r *NativeFollowerFrameRules) NewState() *NativeFollowerFrameState {
	if r == nil {
		return nil
	}
	s := r.initial
	return &s
}

func (r *NativeFollowerFrameRules) bindings(state *NativeFollowerFrameState, outputs NativeFollowerFrameOutputs) NativeFollowerFrameBindings {
	return NativeFollowerFrameBindings{Aftermath: &r.aftermath, Motion: &r.motion, Entry: &r.entry, Combat: &r.combat, Town: r.town, TownState: &state.Town, Decision: &r.decision, Hero: &r.hero, Magnet: &r.magnet, Terrain: &r.terrain, WaitContact: &r.waitContact, Siege: &r.siege, Retained: &r.retained, Neutral: &r.neutral, Commands: outputs.Commands, MapPoint: outputs.MapPoint, Result: outputs.Result}
}

// Tick composes the complete $11252 pass. Nested calls borrow the parent's
// authoritative raw frame; standalone calls reconcile once and hydrate after
// all actors, never between a prepass and its direct child continuation.
// A missing drawing/result boundary is reported only when the source uses it.
func (r *NativeFollowerFrameRules) Tick(w *World, frame *NativeFrameRegisterContext, state *NativeFollowerFrameState, outputs NativeFollowerFrameOutputs) error {
	if r == nil || w == nil || frame == nil || state == nil {
		return fmt.Errorf("native follower frame session missing")
	}
	if w.Landscape != r.landscape {
		return fmt.Errorf("native follower frame LAND differs from World")
	}
	return w.runNativeFollowerCall(func() error {
		return w.tickNativeFollowerFrame(&r.pass, frame, &state.Pass, r.bindings(state, outputs))
	})
}
