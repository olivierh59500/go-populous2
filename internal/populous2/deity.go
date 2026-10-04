package populous2

import (
	"encoding/binary"
	"fmt"
)

// Deity contains the player profile fields used by the native creation screen.
// Name is separate from the password; face parts are deity+$4e..$50,
// experience is deity+$52..$57, and unspent bolts occupy the word at +$58.
type Deity struct {
	Name       string
	FaceParts  [3]uint8
	Experience [6]uint8
	Bolts      uint16
}

// NewDeity starts with the five bolts written by CODE:$10a84. A fresh profile
// has zero experience and the first variant of each of the three face parts.
func NewDeity(name string) Deity {
	return Deity{Name: name, Bolts: 5}
}

// AllocateBolt translates CODE:$b8f8..$b92a: one bolt adds one experience unit,
// and a full experience byte or an empty bolt balance leaves the profile intact.
func (d *Deity) AllocateBolt(element Element) bool {
	if d == nil || element > Water || d.Bolts == 0 || d.Experience[element] == 255 {
		return false
	}
	d.Experience[element]++
	d.Bolts--
	return true
}

// AwardCampaignBolts translates CODE:$3a28..$3a48 for an already established
// native battle score. The caller applies the original campaign-mode check.
// The addition retains the native unsigned-word wrapping behavior.
func (d *Deity) AwardCampaignBolts(score uint16) uint16 {
	if d == nil {
		return 0
	}
	award := min(score/13007, uint16(5))
	d.Bolts += award
	return award
}

// CampaignWorldStep is the score-derived quantity computed by CODE:$3a4c:
// one plus score/6000, capped at six. Applying it requires the original result
// branch and final-world rules; this function makes no win/loss interpretation.
func CampaignWorldStep(score uint16) int {
	return min(int(score)/6000+1, 6)
}

// CycleFace translates the previous/next handlers at CODE:$b94a and $b972.
// Each part has eight variants; direction is exactly -1 or +1.
func (d *Deity) CycleFace(part, direction int) bool {
	if d == nil || part < 0 || part >= len(d.FaceParts) ||
		(direction != -1 && direction != 1) || d.FaceParts[part] > 7 {
		return false
	}
	d.FaceParts[part] = uint8((int(d.FaceParts[part]) + direction + 8) & 7)
	return true
}

// Password encodes the native sixteen-letter profile code. The imported format
// permits only three-bit face and bolt values, as checked by CODE:$ba02..$ba3c.
// Neither the profile name nor the campaign world is part of this code.
func (d Deity) Password() (string, error) {
	for _, part := range d.FaceParts {
		if part > 7 {
			return "", fmt.Errorf("deity face variant %d outside 0..7", part)
		}
	}
	if d.Bolts > 7 {
		return "", fmt.Errorf("deity bolt balance %d exceeds password range 0..7", d.Bolts)
	}
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
	return string(letters[:]), nil
}

// DecodeDeityPassword reverses CODE:$10564 and validates the packed fields as
// the creation screen does. The original code uses four base-26 words, each
// divisible by three, rather than a general checksum or a world password.
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
			return Deity{}, fmt.Errorf("invalid native deity password word %d", word)
		}
		binary.BigEndian.PutUint16(payload[word*2:], uint16(value/3))
	}
	binary.BigEndian.PutUint32(payload[:4], binary.BigEndian.Uint32(payload[:4])^0xec89bb22)
	binary.BigEndian.PutUint32(payload[4:], binary.BigEndian.Uint32(payload[4:])^0x137644dd)
	payload = transposeDeityBits(payload)
	if payload[0]&0x88 != 0 || payload[1]&0x88 != 0 {
		return Deity{}, fmt.Errorf("native deity password has invalid face or bolt bits")
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

// transposeDeityBits is the eight-by-eight bit transpose at CODE:$1052c.
func transposeDeityBits(input [8]byte) (output [8]byte) {
	for bit := 0; bit < 8; bit++ {
		for row, value := range input {
			output[bit] |= ((value >> bit) & 1) << row
		}
	}
	return output
}

// transposeDeityLetters is the four-by-four character transpose at CODE:$104d6.
func transposeDeityLetters(letters *[16]byte) {
	for row := 0; row < 4; row++ {
		for column := row + 1; column < 4; column++ {
			a, b := row*4+column, column*4+row
			letters[a], letters[b] = letters[b], letters[a]
		}
	}
}
