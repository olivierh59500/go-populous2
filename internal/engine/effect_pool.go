package engine

const EffectCapacity = 250

type EffectKind uint8

const (
	EffectNone EffectKind = iota
	EffectFireColumn
	EffectFireRain
	EffectVolcano
	EffectLava
	EffectFungus
	EffectSwamp
	EffectLightning
	EffectWhirlwind
	EffectStorm
	EffectHurricane
	EffectTidalWave
	EffectWhirlpool
	EffectBasalt
	EffectEarthquake
	EffectPlague
)

// EffectReservation gives all powers a single bounded allocation budget.
// Feature-specific state stays in named structs, indexed by the same slot.
// Previous velocity survives reuse because some original creators initialize
// direction only when their animation transitions to its active phase.
type EffectReservation struct {
	Kind                         EffectKind
	Owner                        uint8
	Generation                   uint64
	LastVelocityX, LastVelocityY int
}
type effectPool struct {
	Slots [EffectCapacity]EffectReservation
}

func (p *effectPool) allocate(kind EffectKind, owner uint8) int {
	if kind == EffectNone || owner > 2 {
		return -1
	}
	for id := range p.Slots {
		if p.Slots[id].Kind == EffectNone {
			p.Slots[id].Kind = kind
			p.Slots[id].Owner = owner
			p.Slots[id].Generation++
			return id
		}
	}
	return -1
}
func (p *effectPool) free(id int) {
	if id >= 0 && id < len(p.Slots) {
		p.Slots[id].Kind = EffectNone
	}
}
func (w *World) allocateEffect(kind EffectKind, owner uint8) int {
	return w.effects.allocate(kind, owner)
}
func (w *World) releaseEffect(id int) {
	w.Actors.Unlink(ActorRef{Kind: ActorEffect, Index: uint16(id)})
	w.effects.free(id)
}
