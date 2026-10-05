package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type campaignResultFixture struct {
	Input struct {
		Name, Mode                                   string
		Selected, Eliminated, GameMode, World, Score uint16
		Ticks                                        uint32
		Local, Opponent                              CampaignResultStatistics
	}
	Score, Ratio, World, Bolts uint16
	Overflow, Trap             bool
	Stop, Hash                 string
	Changes                    []nativeHeroChange
}

type campaignResultMemory [0xeb50]byte

func (m *campaignResultMemory) read16(a int) (uint16, error) {
	return binary.BigEndian.Uint16(m[a : a+2]), nil
}
func (m *campaignResultMemory) read32(a int) (uint32, error) {
	return binary.BigEndian.Uint32(m[a : a+4]), nil
}
func (m *campaignResultMemory) write16(a int, v uint16) error {
	binary.BigEndian.PutUint16(m[a:a+2], v)
	return nil
}
func (m *campaignResultMemory) write32(a int, v uint32) { binary.BigEndian.PutUint32(m[a:a+4], v) }

func campaignResultFixtureMemory(f campaignResultFixture) *campaignResultMemory {
	m := &campaignResultMemory{}
	local, opponent := 1, 2
	if f.Input.Selected == 2 {
		local, opponent = 2, 1
	}
	for owner, s := range map[int]CampaignResultStatistics{local: f.Input.Local, opponent: f.Input.Opponent} {
		a := 0xe76a + owner*314
		for _, x := range []struct {
			Offset int
			Value  uint32
		}{{4, s.Population}, {0x3c, s.PeakPopulation}, {0x40, s.PeakMana}} {
			m.write32(a+x.Offset, x.Value)
		}
		for _, x := range []struct {
			Offset int
			Value  uint16
		}{{0x18, s.Identity}, {0x44, s.Metric}, {0x46, s.LeaderLosses}, {0x48, s.BattleWins}, {0x4a, s.ScenarioOptions}, {0x138, s.WeightedPowerUse}, {0x58, s.Bolts}} {
			_ = m.write16(a+x.Offset, x.Value)
		}
	}
	_ = m.write16(0xeb42, f.Input.Selected)
	_ = m.write16(0xeb44, f.Input.GameMode)
	_ = m.write16(0xeb46, f.Input.World)
	m.write32(0xf40, f.Input.Ticks)
	_ = m.write16(0xdd0, f.Input.Score)
	return m
}

func TestCampaignResultAgainstOriginalScoreAndProgression(t *testing.T) {
	data, err := os.ReadFile("testdata/campaign_result_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []campaignResultFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1096 {
		t.Fatal("native campaign result fixture catalog incomplete")
	}
	r, err := DecodeCampaignResultRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	if r.ScoreBase != 5000 || r.PowerWeight != 150 || r.BoltDivisor != 13007 || r.WorldDivisor != 6000 || r.BoltCap != 5 || r.WorldStepCap != 6 || r.LastWorld != 999 {
		t.Fatal("native campaign result constants differ")
	}
	scores, progressions, traps, overflows := 0, 0, 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := campaignResultFixtureMemory(f)
			before := *m
			memory := FollowerCleanupMemory{Read16: m.read16, Read32: m.read32, Write16: m.write16}
			selected := uint8(f.Input.Selected)
			local, err := ReadCampaignResultStatistics(selected, memory)
			if err != nil {
				t.Fatal(err)
			}
			other := uint8(3 - f.Input.Selected)
			opponent, err := ReadCampaignResultStatistics(other, memory)
			if err != nil {
				t.Fatal(err)
			}
			if local != f.Input.Local || opponent != f.Input.Opponent {
				t.Fatal("native raw result statistics decode differs")
			}
			if f.Input.Mode == "score" {
				scores++
				score, err := r.Score(f.Input.Ticks, local, opponent)
				if f.Trap {
					traps++
					if err == nil {
						t.Fatal("native divide-zero fault was replaced")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if score.Value != f.Score || score.RatioWord != f.Ratio || score.DivideOverflow != f.Overflow {
						t.Fatalf("native score result differs: %+v want%d ratio%d overflow%v", score, f.Score, f.Ratio, f.Overflow)
					}
					if score.DivideOverflow {
						overflows++
					}
					_ = m.write16(0xdd0, score.Value)
				}
			} else {
				progressions++
				p, err := r.Progress(f.Input.GameMode, f.Input.Selected, f.Input.Eliminated, f.Input.World, f.Input.Score, memory)
				if err != nil {
					t.Fatal(err)
				}
				stop := "reload"
				if p.OpenDeity {
					stop = "deity"
				}
				if p.Complete {
					stop = "complete"
				}
				if p.NextWorld != f.World || stop != f.Stop {
					t.Fatalf("native progression branch differs: %+v wantworld%d stop%s", p, f.World, f.Stop)
				}
				_ = m.write16(0xeb46, p.NextWorld)
				bolts, _ := m.read16(0xe76a + int(f.Input.Selected)*314 + 0x58)
				if bolts != f.Bolts {
					t.Fatal("native bolt balance differs")
				}
			}
			if fmt.Sprintf("%x", sha256.Sum256(m[:])) != f.Hash {
				t.Fatal("complete native campaign BSS differs")
			}
			changes := []nativeHeroChange{}
			for i, b := range before {
				if b != m[i] {
					changes = append(changes, nativeHeroChange{Address: i, Value: m[i]})
				}
			}
			if !reflect.DeepEqual(changes, f.Changes) {
				t.Fatal("native campaign changed byte ranges differ")
			}
		})
	}
	if scores != 200 || progressions != 896 || traps != 40 || overflows == 0 {
		t.Fatalf("native result coverage differs: %d/%d traps%d overflows%d", scores, progressions, traps, overflows)
	}
}

func TestCampaignResultUsesZeroPopulationAndNativeIdentity(t *testing.T) {
	r := CampaignResultRules{}
	sides := [3]CampaignResultStatistics{{}, {Population: 1, Identity: 2}, {Population: 0, Identity: 1}}
	identity, ended := r.Detect(2, sides)
	if !ended || identity != 1 {
		t.Fatal("swapped identity not preserved")
	}
	sides[1].Population = 0
	identity, ended = r.Detect(2, sides)
	if !ended || identity != 2 {
		t.Fatal("original first-zero side order changed")
	}
	if _, ended := r.Detect(8, sides); ended {
		t.Fatal("editor received automatic result")
	}
	sides[1].Population = 0xffffffff
	sides[2].Population = 1
	if _, ended := r.Detect(2, sides); ended {
		t.Fatal("signed-negative sum was treated as original zero")
	}
}
