package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportResult(source *populous2.Bundle, output string) error {
	source, err := interfaceSource(source)
	if err != nil {
		return err
	}
	presentation, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		return err
	}
	markers := []NamedRequesterField{{"winner", "WINR"}, {"elapsed", "ELAPSEDABCDE"}, {"local-people", "LOCALPOP"}, {"opponent-people", "OTHERPOP"}, {"local-mana", "LOCALMAN"}, {"opponent-mana", "OTHERMAN"}, {"local-wins", "WINAA"}, {"opponent-wins", "WINBB"}, {"local-leaders", "LEAAA"}, {"opponent-leaders", "LEBBB"}, {"score", "SCRZZ"}}
	parameters := make([][]byte, len(markers))
	for i, m := range markers {
		parameters[i] = []byte(m.Marker)
	}
	requester, err := presentation.Compile(populous2.NativeMenuResult, parameters)
	if err != nil {
		return err
	}
	layout, err := decodeNamedRequester("result", presentation.Requesters, requester, source.Landscapes[0].Palettes[0], map[int]string{2: "continue"}, markers)
	if err != nil {
		return err
	}
	for i := range layout.Fields {
		layout.Fields[i].Padding = ' '
	}
	descriptor := visualassets.ResultDescriptor{Version: 1, Layout: *layout}
	for winner := 0; winner < 2; winner++ {
		result, err := presentation.Result(1, uint16(2-winner), 0, populous2.CampaignResultStatistics{}, populous2.CampaignResultStatistics{}, 0)
		if err != nil {
			return err
		}
		descriptor.Winners[winner] = string(result.Parameters[0])
		descriptor.DaySuffix = strings.TrimPrefix(string(result.Parameters[1]), "0")
	}
	data, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, visualassets.ResultFile), append(data, '\n'), 0644)
}
