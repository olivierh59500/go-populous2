package engine

import (
	"errors"
	"fmt"
)

type Element uint8

const (
	People Element = iota
	Plants
	Earth
	Air
	Fire
	Water
)

type PowerID uint8

const (
	RaiseLower  PowerID = 0
	PapalMagnet PowerID = 1
	Perseus     PowerID = 2
	Plague      PowerID = 3
	Armageddon  PowerID = 4
	Trees       PowerID = 6
	Flowers     PowerID = 7
	Swamp       PowerID = 8
	Fungus      PowerID = 9
	Adonis      PowerID = 10
	Road        PowerID = 12
	Wall        PowerID = 13
	Earthquake  PowerID = 14
	Batholith   PowerID = 15
	Heracles    PowerID = 16
	Lightning   PowerID = 18
	Whirlwind   PowerID = 19
	Storm       PowerID = 20
	Odysseus    PowerID = 21
	Wind        PowerID = 22
	FireColumn  PowerID = 24
	FireRain    PowerID = 25
	Volcano     PowerID = 26
	Achilles    PowerID = 27
	Basalt      PowerID = 30
	Whirlpool   PowerID = 31
	Baptism     PowerID = 32
	Helen       PowerID = 33
	Tsunami     PowerID = 34
)

type Power struct {
	ID          PowerID
	Name        string
	Element     Element
	Cost        int
	Implemented bool
}
type PowerTarget struct {
	X, Y, EndX, EndY int
	Lower            bool
	Direction        uint8
}

// ErrPowerUnavailable explicitly identifies work still to be implemented.
// An unavailable feature is never reported as cast and never consumes mana.
var ErrPowerUnavailable = errors.New("power is not implemented by this engine yet")

var Powers = []Power{
	{RaiseLower, "Raise / lower", People, 5, true}, {PapalMagnet, "Papal magnet", People, 25, true}, {Perseus, "Perseus", People, 10750, false}, {Plague, "Plague", People, 30000, false}, {Armageddon, "Armageddon", People, 65524, false},
	{Trees, "Trees", Plants, 250, true}, {Flowers, "Flowers", Plants, 500, true}, {Swamp, "Swamp", Plants, 1750, true}, {Fungus, "Fungus", Plants, 2000, true}, {Adonis, "Adonis", Plants, 11000, false},
	{Road, "Road", Earth, 25, false}, {Wall, "Wall", Earth, 62, false}, {Earthquake, "Earthquake", Earth, 10000, false}, {Batholith, "Batholith", Earth, 11250, false}, {Heracles, "Heracles", Earth, 11000, false},
	{Lightning, "Lightning", Air, 50, false}, {Whirlwind, "Whirlwind", Air, 2750, false}, {Storm, "Storm", Air, 5250, false}, {Odysseus, "Odysseus", Air, 11000, false}, {Wind, "Wind", Air, 23000, false},
	{FireColumn, "Fire column", Fire, 5625, true}, {FireRain, "Fire rain", Fire, 7500, true}, {Volcano, "Volcano", Fire, 10000, false}, {Achilles, "Achilles", Fire, 20000, false},
	{Basalt, "Basalt", Water, 250, true}, {Whirlpool, "Whirlpool", Water, 1000, false}, {Baptism, "Baptism", Water, 6250, false}, {Helen, "Helen", Water, 7500, false}, {Tsunami, "Tsunami", Water, 25000, false},
}

func PowerByID(id PowerID) (Power, bool) {
	for _, p := range Powers {
		if p.ID == id {
			return p, true
		}
	}
	return Power{}, false
}

// PowerCost converts the catalogue's quarter-mana price to ledger units and
// applies the deity's experience discount for the power's element.
func (w *World) PowerCost(owner int, id PowerID) int {
	power, ok := PowerByID(id)
	if !ok || owner < 0 || owner > 1 {
		return -1
	}
	divisors := [8]int{0, 0, 10, 9, 8, 7, 6, 5}
	divisor := divisors[w.Players[owner].Experience[power.Element]>>5]
	cost := power.Cost
	if divisor != 0 {
		cost -= cost / divisor
	}
	return cost * 4
}

func (w *World) Cast(owner int, id PowerID, target PowerTarget) error {
	p, ok := PowerByID(id)
	if !ok {
		return fmt.Errorf("unknown power %d", id)
	}
	if !p.Implemented {
		return fmt.Errorf("%s: %w", p.Name, ErrPowerUnavailable)
	}
	if owner < 0 || owner > 1 {
		return errors.New("invalid player")
	}
	if !w.Level.Players[owner].Powers[id] {
		return errors.New("power is disabled in this world")
	}
	if w.Players[owner].Mana < w.PowerCost(owner, id) {
		return errors.New("not enough mana")
	}
	switch id {
	case RaiseLower:
		var changed bool
		if target.Lower {
			changed = w.LowerAt(owner, target.X, target.Y)
		} else {
			changed = w.RaiseAt(owner, target.X, target.Y)
		}
		if !changed {
			return errors.New("terrain cannot be changed")
		}
	case PapalMagnet:
		if !w.PlaceMagnet(owner, target.X, target.Y) {
			return errors.New("invalid rally target")
		}
		w.Players[owner].Mana -= w.PowerCost(owner, id)
	case Trees, Flowers, Swamp, Fungus:
		var err error
		switch id {
		case Trees:
			err = w.CastTrees(owner, target.X, target.Y)
		case Flowers:
			err = w.CastFlowers(owner, target.X, target.Y)
		case Swamp:
			err = w.CastSwamp(owner, target.X, target.Y)
		case Fungus:
			err = w.CastFungus(owner, target.X, target.Y)
		}
		if errors.Is(err, ErrNatureNoChange) {
			return nil
		}
		if err != nil {
			return err
		}
		w.Players[owner].Mana -= w.PowerCost(owner, id)
	case FireColumn, FireRain:
		var err error
		if id == FireColumn {
			err = w.CastFireColumn(owner, target.X, target.Y)
		} else {
			err = w.CastFireRain(owner, target.X, target.Y)
		}
		if err != nil {
			return err
		}
		w.Players[owner].Mana -= w.PowerCost(owner, id)
	case Basalt:
		if err := w.CastBasalt(owner, target.X, target.Y, int(target.Direction)); err != nil {
			return err
		}
		w.Players[owner].Mana -= w.PowerCost(owner, id)
	}
	return nil
}
