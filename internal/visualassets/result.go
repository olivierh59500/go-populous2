package visualassets

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"strconv"
)

const ResultFile = "result-layout.json"

type ResultDescriptor struct {
	Version   int
	Layout    RequesterLayout
	Winners   [2]string
	DaySuffix string
}
type ResultStatistics struct {
	PeakPopulation, PeakMana uint32
	BattleWins, LeaderLosses uint16
}
type ResultState struct {
	Winner          int
	Ticks           uint32
	Local, Opponent ResultStatistics
	Score           uint16
}

// Values supplies ordinary Go result data to original named glyph fields.
// Scoring and campaign progression are deliberately outside this presenter.
func (d *ResultDescriptor) Values(s ResultState) map[string]string {
	number := func(n uint32) string { return strconv.FormatUint(uint64(n), 10) }
	winner := ""
	if s.Winner >= 0 && s.Winner < 2 {
		winner = d.Winners[s.Winner]
	}
	return map[string]string{
		"winner": winner, "elapsed": number(s.Ticks/50) + d.DaySuffix,
		"local-people": number(s.Local.PeakPopulation), "opponent-people": number(s.Opponent.PeakPopulation),
		"local-mana": number(s.Local.PeakMana), "opponent-mana": number(s.Opponent.PeakMana),
		"local-wins": number(uint32(s.Local.BattleWins)), "opponent-wins": number(uint32(s.Opponent.BattleWins)),
		"local-leaders": number(uint32(s.Local.LeaderLosses)), "opponent-leaders": number(uint32(s.Opponent.LeaderLosses)), "score": number(uint32(s.Score)),
	}
}
func LoadResult(files fs.FS) (*ResultDescriptor, error) {
	f, err := files.Open(ResultFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	var result ResultDescriptor
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("result layout has trailing data")
	}
	if result.Version != 1 || len(result.DaySuffix) > 12 || len(result.Winners[0]) > 8 || len(result.Winners[1]) > 8 {
		return nil, fmt.Errorf("invalid result presentation")
	}
	if err := result.Layout.Validate(); err != nil {
		return nil, err
	}
	if len(result.Layout.Fields) != 11 {
		return nil, fmt.Errorf("result fields are incomplete")
	}
	return &result, nil
}
