package populous2

import (
	"strings"
	"testing"
)

// Fixtures were produced by executing the original French Amiga CODE:$1047c
// encoder and CODE:$10564 decoder in an isolated 68000 CPU. They cover all six
// experience bytes, divisor boundaries, packed face fields, and high-bit values.
func TestDeityPasswordsMatchOriginal68000(t *testing.T) {
	tests := []struct {
		payload  [8]uint8
		password string
	}{
		{[8]uint8{0, 5, 0, 0, 0, 0, 0, 0}, "KIADKCWAZICGZOWD"},
		{[8]uint8{119, 119, 0, 31, 32, 63, 128, 255}, "DAIICSBSVUXFEOVN"},
		{[8]uint8{35, 69, 31, 0, 0, 0, 0, 0}, "KIBDHGACPXRNSEDF"},
		{[8]uint8{35, 69, 32, 0, 0, 0, 0, 0}, "KIADMCWCDICNKONF"},
		{[8]uint8{35, 69, 224, 0, 0, 0, 0, 0}, "KIACMCWXDICYKONP"},
		{[8]uint8{35, 69, 255, 0, 0, 0, 0, 0}, "KIBCHGAXPXQYSERP"},
		{[8]uint8{35, 69, 0, 31, 0, 0, 0, 0}, "KHBDDTFCAHFNEEHF"},
		{[8]uint8{35, 69, 0, 32, 0, 0, 0, 0}, "KIADMCWCDIDNKOXF"},
		{[8]uint8{35, 69, 0, 224, 0, 0, 0, 0}, "KIADMCWLDIDOKOXP"},
		{[8]uint8{35, 69, 0, 255, 0, 0, 0, 0}, "KHBDDTFLAHGOEEFP"},
		{[8]uint8{35, 69, 0, 0, 31, 0, 0, 0}, "LHADEKDCJFYNWUJF"},
		{[8]uint8{35, 69, 0, 0, 32, 0, 0, 0}, "KIADMCWCDIBNKODF"},
		{[8]uint8{35, 69, 0, 0, 224, 0, 0, 0}, "KIADMCWUDIBPKODZ"},
		{[8]uint8{35, 69, 0, 0, 255, 0, 0, 0}, "LHADEKDUJFWPWUNZ"},
		{[8]uint8{35, 69, 0, 0, 0, 31, 0, 0}, "JGCDBRGCXVMNWQFF"},
		{[8]uint8{35, 69, 0, 0, 0, 32, 0, 0}, "KIADMCVCDIZNKOHF"},
		{[8]uint8{35, 69, 0, 0, 0, 224, 0, 0}, "KIAEMCVNDIZAKOHD"},
		{[8]uint8{35, 69, 0, 0, 0, 255, 0, 0}, "JGCEBRGNXVIAWQND"},
		{[8]uint8{35, 69, 0, 0, 0, 0, 31, 0}, "HKDDRXQCSIVNIKLF"},
		{[8]uint8{35, 69, 0, 0, 0, 0, 32, 0}, "KIADMCVCDIVNKOPF"},
		{[8]uint8{35, 69, 0, 0, 0, 0, 224, 0}, "KIAAMCVHDIVNKOPJ"},
		{[8]uint8{35, 69, 0, 0, 0, 0, 255, 0}, "HKDARXQHSIONIKBJ"},
		{[8]uint8{35, 69, 0, 0, 0, 0, 0, 31}, "ECGDWNLCDMNNSKXF"},
		{[8]uint8{35, 69, 0, 0, 0, 0, 0, 32}, "KIADMCWCDIRNKOTF"},
		{[8]uint8{35, 69, 0, 0, 0, 0, 0, 224}, "KIAIMCWRDIRJKOTJ"},
		{[8]uint8{35, 69, 0, 0, 0, 0, 0, 255}, "ECGIWNMRDMCJSKRJ"},
		{[8]uint8{20, 100, 237, 206, 240, 94, 186, 127}, "EAJKOXXYPQMPFOEN"},
		{[8]uint8{33, 17, 37, 188, 230, 135, 48, 169}, "DFDDIYXRADWWTIRX"},
		{[8]uint8{18, 116, 129, 207, 144, 220, 169, 96}, "HGBKAJLLIBECYQAT"},
		{[8]uint8{20, 114, 172, 5, 21, 199, 29, 64}, "GICKMRVCXKFKLKKP"},
		{[8]uint8{82, 100, 138, 131, 92, 37, 120, 114}, "IGIGOANQSEBMAKOL"},
		{[8]uint8{33, 7, 138, 49, 226, 18, 100, 52}, "KFIADHFXHQFPLOXZ"},
		{[8]uint8{83, 5, 223, 37, 230, 1, 237, 156}, "FEGATLOTNTQJOYCF"},
		{[8]uint8{4, 49, 138, 89, 125, 207, 17, 220}, "GAILPMUAEBUVTQUR"},
	}
	for _, test := range tests {
		d := Deity{Name: "ORACLE", FaceParts: [3]uint8{test.payload[0] >> 4, test.payload[0] & 15, test.payload[1] >> 4}, Bolts: uint16(test.payload[1] & 15)}
		copy(d.Experience[:], test.payload[2:])
		before := d
		got, err := d.Password()
		if err != nil || got != test.password {
			t.Fatalf("payload %v: password %q, want %q (%v)", test.payload, got, test.password, err)
		}
		if d != before {
			t.Fatal("encoding changed the profile")
		}
		decoded, err := DecodeDeityPassword(d.Name, test.password)
		if err != nil || decoded != d {
			t.Fatalf("native password %s decoded to %+v (%v), want %+v", test.password, decoded, err, d)
		}
	}
}

func TestDeityInitialBoltsAndNativeByteAllocation(t *testing.T) {
	d := NewDeity("PLAYER")
	if d.Bolts != 5 || d.Experience != [6]uint8{} || d.FaceParts != [3]uint8{} {
		t.Fatalf("unexpected fresh deity: %+v", d)
	}
	for i := 0; i < 5; i++ {
		if !d.AllocateBolt(Fire) {
			t.Fatal("funded allocation was rejected")
		}
	}
	if d.Bolts != 0 || d.Experience[Fire] != 5 {
		t.Fatalf("one bolt must add one byte unit, got %+v", d)
	}
	before := d
	if d.AllocateBolt(Fire) || d != before {
		t.Fatal("empty bolt balance changed the deity")
	}
	for element := People; element <= Water; element++ {
		for _, xp := range []uint8{0, 1, 31, 32, 254, 255} {
			for _, bolts := range []uint16{0, 1, 5, 65535} {
				x := Deity{Bolts: bolts}
				x.Experience[element] = xp
				before := x
				applied := x.AllocateBolt(element)
				want := bolts != 0 && xp != 255
				if applied != want {
					t.Fatalf("element %d xp %d bolts %d: applied %t, want %t", element, xp, bolts, applied, want)
				}
				if !want && x != before {
					t.Fatal("rejected native allocation mutated the deity")
				}
				if want && (x.Bolts != bolts-1 || x.Experience[element] != xp+1) {
					t.Fatalf("native byte/word allocation mismatch: %+v", x)
				}
			}
		}
	}
}

func TestDeityNativeCampaignAwardThresholds(t *testing.T) {
	for _, test := range []struct {
		score uint16
		award uint16
		step  int
	}{
		{0, 0, 1}, {5999, 0, 1}, {6000, 0, 2}, {12999, 0, 3},
		{13006, 0, 3}, {13007, 1, 3}, {26013, 1, 5}, {26014, 2, 5},
		{30000, 2, 6}, {39021, 3, 6}, {52028, 4, 6}, {65035, 5, 6}, {65535, 5, 6},
	} {
		d := Deity{Bolts: 65535}
		if got := d.AwardCampaignBolts(test.score); got != test.award || d.Bolts != uint16(65535+uint32(test.award)) {
			t.Fatalf("score %d: award %d balance %d", test.score, got, d.Bolts)
		}
		if got := CampaignWorldStep(test.score); got != test.step {
			t.Fatalf("score %d: world step %d, want %d", test.score, got, test.step)
		}
	}
}

func TestDeityFacePartBoundsAndWrapping(t *testing.T) {
	d := NewDeity("FACE")
	for part := range d.FaceParts {
		if !d.CycleFace(part, -1) || d.FaceParts[part] != 7 {
			t.Fatal("previous face did not wrap zero to seven")
		}
		if !d.CycleFace(part, 1) || d.FaceParts[part] != 0 {
			t.Fatal("next face did not wrap seven to zero")
		}
		for i := 0; i < 8; i++ {
			if !d.CycleFace(part, 1) {
				t.Fatal("valid face step rejected")
			}
		}
		if d.FaceParts[part] != 0 {
			t.Fatal("face did not return after eight variants")
		}
	}
	before := d
	for _, action := range [][2]int{{-1, 1}, {3, 1}, {0, 0}, {0, 2}} {
		if d.CycleFace(action[0], action[1]) || d != before {
			t.Fatal("invalid face operation mutated the deity")
		}
	}
}

func TestDeityPasswordRejectsInvalidDataAtomically(t *testing.T) {
	d := Deity{Name: "KEEP", FaceParts: [3]uint8{3, 4, 5}, Experience: [6]uint8{2, 4, 8, 16, 32, 64}, Bolts: 7}
	before := d
	for _, password := range []string{"", "DOEGAC", strings.Repeat("Z", 16), "kiadkcwazicgzowd", "!IADKCWAZICGZOWD"} {
		if err := d.SetPassword(password); err == nil || d != before {
			t.Fatalf("invalid profile password %q changed state or was accepted", password)
		}
	}
	for part := range d.FaceParts {
		x := d
		x.FaceParts[part] = 8
		if _, err := x.Password(); err == nil {
			t.Fatal("unrepresentable face was encoded")
		}
	}
	x := d
	x.Bolts = 8
	if _, err := x.Password(); err == nil {
		t.Fatal("unrepresentable bolt balance was encoded")
	}
	password, err := NewDeity("SOURCE").Password()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetPassword(password); err != nil {
		t.Fatal(err)
	}
	if d.Name != "KEEP" || d.Experience != [6]uint8{} || d.Bolts != 5 {
		t.Fatalf("import did not preserve the separate name: %+v", d)
	}
	invalid := NewDeity("INVALID")
	if invalid.AllocateBolt(Element(6)) || invalid != NewDeity("INVALID") {
		t.Fatal("invalid element changed the profile")
	}
}
