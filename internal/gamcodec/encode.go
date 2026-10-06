package gamcodec

import (
	"encoding/binary"
	"fmt"
	"reflect"

	"go-populous2/internal/engine"
)

func Encode(document *Document) ([]byte, error) {
	if document == nil || document.World == nil || len(document.Metadata.Original) != FileSize {
		return nil, fmt.Errorf("GAM export requires a decoded file document")
	}
	w := document.World
	snapshot := w.Snapshot()
	if _, err := snapshot.Restore(); err != nil {
		return nil, err
	}
	data := append([]byte(nil), document.Metadata.Original...)
	if reflect.DeepEqual(snapshot, document.Metadata.initial) && document.Metadata.Profile == document.Metadata.initialProfile {
		return data, nil
	}
	byteAt := func(at int, value uint8) { data[at-fileStart] = value }
	wordAt := func(at int, value uint16) { binary.BigEndian.PutUint16(data[at-fileStart:], value) }
	longAt := func(at int, value uint32) { binary.BigEndian.PutUint32(data[at-fileStart:], value) }
	profile := document.Metadata.Profile
	if len(profile.Name) > 15 || document.Metadata.ProfileSide < 0 || document.Metadata.ProfileSide > 1 {
		return nil, fmt.Errorf("GAM profile does not fit its file fields")
	}
	for _, part := range profile.FaceParts {
		if part > 7 {
			return nil, fmt.Errorf("GAM face variant is invalid")
		}
	}
	name := data[0xeb30-fileStart : 0xeb40-fileStart]
	if profile.Name != document.Metadata.initialProfile.Name {
		copy(name, profile.Name)
		name[len(profile.Name)] = 0
	}
	profileGod := 0xe76a + (document.Metadata.ProfileSide+1)*314
	wordAt(profileGod+0x58, profile.Bolts)
	for i, part := range profile.FaceParts {
		byteAt(profileGod+0x4e+i, part)
	}
	for id, reservation := range snapshot.Reservations {
		if reservation.Kind != engine.EffectNone {
			return nil, fmt.Errorf("GAM export of active effect%d (%d) is not mapped yet", id, reservation.Kind)
		}
	}
	for id, wall := range w.Earth.Walls {
		if wall.Active {
			return nil, fmt.Errorf("GAM export of active wall%d is not mapped yet", id)
		}
	}
	longAt(0xf40, uint32(w.Tick))
	longAt(0xeb24, w.Level.Seed)
	longAt(0xeb28, snapshot.Random)
	wordAt(0xeb22, uint16(w.Level.Landscape))
	wordAt(0xeb46, uint16(w.Level.Number))
	for at, cell := range w.Tiles {
		logical := w.Cell(at%64, at/64)
		byteAt(0xf44+at*4, (data[0xf44+at*4-fileStart]&0xf8)|(cell.BaseAltitude&7))
		byteAt(0xf45+at*4, logical.Code)
		byteAt(0x4f44+at, w.Pressure[at])
		head, err := fileReference(w.Actors.Heads[at])
		if err != nil {
			return nil, err
		}
		wordAt(0xf46+at*4, head)
	}
	for id, f := range w.Followers {
		if id == 0 {
			continue
		}
		at := 0x76c0 + id*52
		if f.State == engine.Inactive {
			byteAt(at+12, 0)
			continue
		}
		if f.Owner > 1 || f.Conversion.Active || f.Disease.Dying || f.Neutral.Kind != engine.NeutralNone || f.State != engine.Walking && f.State != engine.Town {
			return nil, fmt.Errorf("GAM follower%d lifecycle is not mapped yet", id)
		}
		link := w.Actors.Followers[id]
		next, err := fileReference(link.Next)
		if err != nil {
			return nil, err
		}
		previous, err := fileReference(link.Previous)
		if err != nil {
			return nil, err
		}
		wordAt(at+2, next)
		wordAt(at+4, previous)
		byteAt(at+12, f.Owner+1)
		motion := snapshot.Motion[id]
		wordAt(at+6, uint16(motion.PositionX))
		wordAt(at+8, uint16(motion.PositionY))
		wordAt(at+14, uint16(int16(motion.VelocityX)))
		wordAt(at+16, uint16(int16(motion.VelocityY)))
		byteAt(at+18, f.MovementSpeed)
		byteAt(at+25, uint8(f.Weapons))
		longAt(at+26, uint32(int32(f.Population)))
		wordAt(at+46, uint16(f.FoundedAt))
		if f.State == engine.Town {
			byteAt(at, 4)
			byteAt(at+1, f.Stage)
			byteAt(at+22, 6)
			wordAt(at+20, f.Work)
		} else {
			byteAt(at, 2)
			phase := uint8(2)
			if motion.Moving {
				phase = 4
			}
			byteAt(at+22, phase)
			wordAt(at+20, uint16(int16(motion.LegRemaining)))
		}
		flags := data[at+13-fileStart] &^ uint8(2|0x20)
		if f.IsHero() {
			flags |= 2
			wordAt(at+40, uint16(f.Hero.Kind-1)*2)
		}
		if f.Disease.Infected {
			flags |= 0x20
		}
		byteAt(at+13, flags)
		wordAt(at+10, uint16(f.Frame*4))
	}
	for id, scenery := range w.Nature.Scenery {
		at := 0x6bd0 + id*14
		if scenery.Kind == engine.SceneryNone {
			byteAt(at+12, 0)
			continue
		}
		kind := uint8(22)
		if scenery.Kind == engine.SceneryBoulder {
			kind = 24
		} else if scenery.Kind == engine.SceneryBurningTree {
			kind = 30
		}
		byteAt(at, kind)
		byteAt(at+1, uint8(scenery.Age))
		byteAt(at+6, scenery.X)
		byteAt(at+8, scenery.Y)
		byteAt(at+12, 3)
		link := w.Actors.Scenery[id]
		next, err := fileReference(link.Next)
		if err != nil {
			return nil, err
		}
		previous, err := fileReference(link.Previous)
		if err != nil {
			return nil, err
		}
		wordAt(at+2, next)
		wordAt(at+4, previous)
	}
	for owner, player := range w.Players {
		god := 0xe76a + (owner+1)*314
		longAt(god, uint32(int32(player.Mana)))
		longAt(god+20, uint32(w.Level.Players[owner].Attrition))
		wordAt(god+12, [4]uint16{16, 14, 18, 20}[player.Mode])
		leader := engine.ActorRef{}
		if player.Leader > 0 {
			leader = engine.ActorRef{Kind: engine.ActorFollower, Index: uint16(player.Leader)}
		}
		reference, err := fileReference(leader)
		if err != nil {
			return nil, err
		}
		wordAt(god+10, reference)
		for i, value := range player.Experience {
			byteAt(god+0x52+i, value)
		}
		for id, enabled := range w.Level.Players[owner].Powers {
			value := uint8(0)
			if enabled {
				value = 1
			}
			byteAt(god+0x70+id, value)
		}
		at := 0xe74e + owner*14
		wordAt(at+6, uint16(w.Magnets[owner].X))
		wordAt(at+8, uint16(w.Magnets[owner].Y))
		link := w.Actors.Magnets[owner]
		next, err := fileReference(link.Next)
		if err != nil {
			return nil, err
		}
		previous, err := fileReference(link.Previous)
		if err != nil {
			return nil, err
		}
		wordAt(at+2, next)
		wordAt(at+4, previous)
	}
	return data, nil
}
