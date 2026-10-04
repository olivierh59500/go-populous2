package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
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

var ElementNames = [6]string{"PEUPLE", "VEGETATION", "TERRE", "AIR", "FEU", "EAU"}

type SpellID uint8

// Spell IDs retain the original six-slot categories, including unused slots.
const (
	RaiseLower  SpellID = 0
	PapalMagnet SpellID = 1
	Perseus     SpellID = 2
	Plague      SpellID = 3
	Armageddon  SpellID = 4
	Trees       SpellID = 6
	Flowers     SpellID = 7
	Swamp       SpellID = 8
	Fungus      SpellID = 9
	Adonis      SpellID = 10
	Road        SpellID = 12
	Wall        SpellID = 13
	Earthquake  SpellID = 14
	Batholith   SpellID = 15
	Heracles    SpellID = 16
	Lightning   SpellID = 18
	Whirlwind   SpellID = 19
	Storm       SpellID = 20
	Odysseus    SpellID = 21
	Wind        SpellID = 22
	FireColumn  SpellID = 24
	FireRain    SpellID = 25
	Volcano     SpellID = 26
	Achilles    SpellID = 27
	Whirlpool   SpellID = 30
	Basalt      SpellID = 31
	Baptism     SpellID = 32
	Helen       SpellID = 33
	Tsunami     SpellID = 34
)

type Aim uint8

const (
	AimPoint Aim = iota
	AimLeader
	AimGlobal
	AimLine
	AimDirection
)

type Spell struct {
	ID      SpellID
	Name    string
	Element Element
	Cost    int
	Aim     Aim
	Help    string
}

var spellDefinitions = []Spell{
	{ID: RaiseLower, Name: "LEVER / BAISSER", Aim: AimPoint, Help: "Clic gauche: lever. Clic droit: baisser ou faire sortir un adorateur."},
	{ID: PapalMagnet, Name: "AIMANT PAPAL", Aim: AimPoint, Help: "Place l'aimant et rassemble les adorateurs autour du chef."},
	{ID: Perseus, Name: "PERSEE", Aim: AimLeader, Help: "Convertit le chef en heros, habile au combat."},
	{ID: Plague, Name: "PESTE", Aim: AimPoint, Help: "Infecte les habitations et les adorateurs ennemis."},
	{ID: Armageddon, Name: "ARMAGEDDON", Aim: AimGlobal, Help: "Rassemble les deux peuples au centre pour la bataille finale."},
	{ID: Trees, Name: "ARBRES", Aim: AimPoint, Help: "Plante des arbres autour des habitations."},
	{ID: Flowers, Name: "FLEURS", Aim: AimPoint, Help: "Cultive des fleurs pour augmenter les offrandes."},
	{ID: Swamp, Name: "MARAIS", Aim: AimPoint, Help: "Cree un marecage mortel pour les adorateurs."},
	{ID: Fungus, Name: "CHAMPIGNON", Aim: AimPoint, Help: "Seme un champignon qui se propage sur le terrain."},
	{ID: Adonis, Name: "ADONIS", Aim: AimLeader, Help: "Un heros dont les victoires multiplient la destruction."},
	{ID: Road, Name: "ROUTES", Aim: AimPoint, Help: "Maintiens le clic gauche pour poser une route. Clic droit: enlever."},
	{ID: Wall, Name: "MURS", Aim: AimLine, Help: "Choisis deux extremites pour proteger les habitations."},
	{ID: Earthquake, Name: "SEISME", Aim: AimDirection, Help: "Secoue le sol dans la direction choisie."},
	{ID: Batholith, Name: "BATHOLITE", Aim: AimPoint, Help: "Fait surgir une masse de roche sur les terres."},
	{ID: Heracles, Name: "HERACLES", Aim: AimLeader, Help: "Convertit le chef en heros d'une force exceptionnelle."},
	{ID: Lightning, Name: "FOUDRE", Aim: AimPoint, Help: "Frappe les habitations et les adorateurs ennemis."},
	{ID: Whirlwind, Name: "TORNADE", Aim: AimPoint, Help: "Un vortex traverse le terrain et detruit sur son passage."},
	{ID: Storm, Name: "TEMPETE", Aim: AimPoint, Help: "Une tempete frappe les habitations de la zone."},
	{ID: Odysseus, Name: "ULYSSE", Aim: AimLeader, Help: "Convertit le chef en un heros tres rapide."},
	{ID: Wind, Name: "VENT", Aim: AimDirection, Help: "Balaye les adorateurs dans une direction."},
	{ID: FireColumn, Name: "COLONNE DE FEU", Aim: AimPoint, Help: "Une colonne de feu mobile brule le terrain."},
	{ID: FireRain, Name: "PLUIE DE FEU", Aim: AimPoint, Help: "La pluie de feu embrase toute la zone."},
	{ID: Volcano, Name: "VOLCAN", Aim: AimPoint, Help: "Fait surgir un volcan et ravage les environs."},
	{ID: Achilles, Name: "ACHILLE", Aim: AimLeader, Help: "Convertit le chef en un heros qui incendie les terres."},
	{ID: Whirlpool, Name: "TOURBILLON", Aim: AimPoint, Help: "Cree un tourbillon mortel dans la mer."},
	{ID: Basalt, Name: "BASALTE", Aim: AimDirection, Help: "Construit un passage de basalte a travers la mer."},
	{ID: Baptism, Name: "BAPTEME", Aim: AimPoint, Help: "Convertit des adorateurs ennemis a ton camp."},
	{ID: Tsunami, Name: "TSUNAMI", Aim: AimDirection, Help: "Envoie une vague de la mer vers la terre."},
	{ID: Helen, Name: "HELENE", Aim: AimLeader, Help: "Convertit le chef en une heroine qui attire les ennemis."},
}

// DecodeSpells gets base mana costs from the actual unsigned-word table at
// 0x21238. ManaRules applies the per-deity adjustment from CODE:$14768.
func DecodeSpells(exe *amiga.Executable) ([]Spell, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x21238+72 {
		return nil, fmt.Errorf("Populous II power table missing")
	}
	data := exe.Hunks[0].Data[0x21238 : 0x21238+72]
	result := append([]Spell(nil), spellDefinitions...)
	for i := range result {
		spell := &result[i]
		spell.Element = Element(spell.ID / 6)
		spell.Cost = int(binary.BigEndian.Uint16(data[int(spell.ID)*2:]))
		if spell.Cost == 65535 {
			return nil, fmt.Errorf("spell %s points to a reserved slot", spell.Name)
		}
	}
	return result, nil
}

func SpellByID(spells []Spell, id SpellID) (Spell, bool) {
	for _, spell := range spells {
		if spell.ID == id {
			return spell, true
		}
	}
	return Spell{}, false
}

func (id SpellID) IsHero() bool {
	switch id {
	case Perseus, Adonis, Heracles, Odysseus, Achilles, Helen:
		return true
	}
	return false
}
