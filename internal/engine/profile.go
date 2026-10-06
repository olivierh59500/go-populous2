package engine

import (
	"encoding/binary"
	"fmt"
)

type Deity struct {
	Name       string
	FaceParts  [3]uint8
	Experience [6]uint8
	Bolts      uint16
}

// NewDeity begins with five unspent bolts and the first face variants.
func NewDeity(name string) Deity {
	return Deity{Name: name, Bolts: 5}
}

// AllocateBolt adds one experience point when a bolt and room are available.
func (d *Deity) AllocateBolt(element Element) bool {
	if d == nil || element > Water || d.Bolts == 0 || d.Experience[element] == 255 {
		return false
	}
	d.Experience[element]++
	d.Bolts--
	return true
}

func (d *Deity) AwardCampaignBolts(score uint16) uint16 {
	if d == nil {
		return 0
	}
	award := min(score/13007, uint16(5))
	d.Bolts += award
	return award
}

// CampaignWorldStep returns the score-based advance, between one and six worlds.
func CampaignWorldStep(score uint16) int {
	return min(int(score)/6000+1, 6)
}

// CycleFace selects the previous or next of eight variants for one face part.
func (d *Deity) CycleFace(part, direction int) bool {
	if d == nil || part < 0 || part >= len(d.FaceParts) ||
		(direction != -1 && direction != 1) || d.FaceParts[part] > 7 {
		return false
	}
	d.FaceParts[part] = uint8((int(d.FaceParts[part]) + direction + 8) & 7)
	return true
}

// Password encodes appearances, experience and unspent bolts as sixteen letters.
func (d Deity) Password() (string, error) {
	for _, part := range d.FaceParts {
		if part > 7 {
			return "", fmt.Errorf("deity face variant %d outside 0..7", part)
		}
	}
	if d.Bolts > 7 {
		return "", fmt.Errorf("deity bolt balance %d exceeds password range 0..7", d.Bolts)
	}
	return d.encodePassword(), nil
}

func (d Deity) encodePassword() string {
	payload := [8]byte{d.FaceParts[0]<<4 | d.FaceParts[1], d.FaceParts[2]<<4 | uint8(d.Bolts)}
	copy(payload[2:], d.Experience[:])
	payload = transposeDeityBits(payload)
	binary.BigEndian.PutUint32(payload[:4], binary.BigEndian.Uint32(payload[:4])^0xec89bb22)
	binary.BigEndian.PutUint32(payload[4:], binary.BigEndian.Uint32(payload[4:])^0x137644dd)
	var letters [16]byte
	for word := 0; word < 4; word++ {
		value := uint32(binary.BigEndian.Uint16(payload[word*2:])) * 3
		for digit := 3; digit >= 0; digit-- {
			letters[word*4+digit] = byte('A' + value%26)
			value /= 26
		}
	}
	transposeDeityLetters(&letters)
	return string(letters[:])
}

// DecodeDeityPassword validates and decodes the sixteen-letter profile format.
func DecodeDeityPassword(name, password string) (Deity, error) {
	if len(password) != 16 {
		return Deity{}, fmt.Errorf("deity password must contain sixteen letters")
	}
	var letters [16]byte
	copy(letters[:], password)
	for _, letter := range letters {
		if letter < 'A' || letter > 'Z' {
			return Deity{}, fmt.Errorf("deity password requires uppercase A..Z")
		}
	}
	transposeDeityLetters(&letters)
	var payload [8]byte
	for word := 0; word < 4; word++ {
		var value uint32
		for _, letter := range letters[word*4 : word*4+4] {
			value = value*26 + uint32(letter-'A')
		}
		if value%3 != 0 || value/3 > 65535 {
			return Deity{}, fmt.Errorf("invalid deity password word %d", word)
		}
		binary.BigEndian.PutUint16(payload[word*2:], uint16(value/3))
	}
	binary.BigEndian.PutUint32(payload[:4], binary.BigEndian.Uint32(payload[:4])^0xec89bb22)
	binary.BigEndian.PutUint32(payload[4:], binary.BigEndian.Uint32(payload[4:])^0x137644dd)
	payload = transposeDeityBits(payload)
	if payload[0]&0x88 != 0 || payload[1]&0x88 != 0 {
		return Deity{}, fmt.Errorf("deity password has invalid face or bolt bits")
	}
	d := Deity{Name: name, FaceParts: [3]uint8{payload[0] >> 4, payload[0] & 15, payload[1] >> 4}, Bolts: uint16(payload[1] & 15)}
	copy(d.Experience[:], payload[2:])
	return d, nil
}

// SetPassword imports a profile atomically while retaining its separately
// entered name. An invalid code cannot partially replace experience or faces.
func (d *Deity) SetPassword(password string) error {
	if d == nil {
		return fmt.Errorf("nil deity profile")
	}
	decoded, err := DecodeDeityPassword(d.Name, password)
	if err != nil {
		return err
	}
	*d = decoded
	return nil
}

func transposeDeityBits(input [8]byte) (output [8]byte) {
	for bit := 0; bit < 8; bit++ {
		for row, value := range input {
			output[bit] |= ((value >> bit) & 1) << row
		}
	}
	return output
}

func transposeDeityLetters(letters *[16]byte) {
	for row := 0; row < 4; row++ {
		for column := row + 1; column < 4; column++ {
			a, b := row*4+column, column*4+row
			letters[a], letters[b] = letters[b], letters[a]
		}
	}
}
