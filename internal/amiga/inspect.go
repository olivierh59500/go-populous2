package amiga

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
)

type Text struct {
	FileOffset int    `json:"file_offset"`
	Value      string `json:"value"`
}

type FileReport struct {
	Path       string      `json:"path"`
	Size       int         `json:"size"`
	SHA256     string      `json:"sha256"`
	Populous1  bool        `json:"matches_populous1,omitempty"`
	Executable *Executable `json:"executable,omitempty"`
	HunkError  string      `json:"hunk_error,omitempty"`
	Strings    []Text      `json:"strings,omitempty"`
}

type Report struct {
	Input            string       `json:"input"`
	Volume           string       `json:"volume,omitempty"`
	Format           string       `json:"format"`
	SHA256           string       `json:"sha256,omitempty"`
	Identity         string       `json:"identity"`
	Evidence         []string     `json:"evidence"`
	Populous1Matches int          `json:"populous1_data_matches"`
	Files            []FileReport `json:"files"`
}

// These are hashes of the Populous 1 data supplied in previous/go-populous.
// Filenames alone cannot distinguish games or their terrain formats.
var populous1Hashes = map[string]string{
	"demo.pic":     "67884ff3c408cea654baedb0148cd1e57707d27e2958ebf9736d0515ecd3b884",
	"land0":        "4aafd9834d181e59359ec77a7ec01fb5a107738ae2a815e929641423c9c7bd19",
	"land1":        "d1232e66f77a74b06980829cb72f428c1920d1cef67b0d846c66eee803783d43",
	"land2":        "593897ae7c11450ade6e4bdb94ac7a812c8702528d7fb2e5b2f32eba71f0071b",
	"land3":        "a1c75d5248ed524c4a17e6f1168ab519a0537130bd7dc2052181552584fbe2f5",
	"font.dat":     "cd5bedb77ad978ff40ae0e6a69b636728dc8170bfae399589ad4e697a3b313db",
	"gmusic1":      "4ae1ec26ec8c76f7e8ee48f5c56c334878e5434e91e29bfd34ead2600f573efe",
	"gwords":       "8ea349d2f50bb7f34fec8831161c0b21bc506e4a79dee42968da2fdf433890c7",
	"level.dat":    "389eddc565c25d0db48e763471cefb2fd06401cc13fa768f40a4b3912dc59401",
	"load.pic":     "028bbcdf7c4aecda1bce6e1d6aec8b88eb22a98d49d91c7a6010f4a38967a1be",
	"mouths.pic":   "c22adb2ce0e1aff9b21f7fd01827493876dc099be83061a6bd186a49a7d8c368",
	"lord.pic":     "e6ed81379441a258846dd07f1fa646b01d267e1754ce61c6531cf53f148cbc3f",
	"sprites0.dat": "4b1507284aad9515203a9f0e23668368788b5d3b8da401471bee2d8fb558e36f",
	"qaz.pic":      "c30555f7b56ba69d59bf664701f5f8da6d1e11941798ee0b2a1ad37e6a0f1053",
	"spr_320.dat":  "9e7798cb386c2425884e55b8862e619769cd2fd1fda24f7fd4c485174fcd2168",
}

// InspectFile retains exact bytes in code/data hunks, and printable strings with
// file offsets for reverse engineering. Text within CODE may still be data.
func InspectFile(name string, data []byte, includeStrings bool) FileReport {
	report := FileReport{Path: name, Size: len(data), SHA256: Digest(data)}
	report.Populous1 = populous1Hashes[strings.ToLower(name)] == report.SHA256
	if IsExecutable(data) {
		exe, err := ParseExecutable(data)
		if err != nil {
			report.HunkError = err.Error()
		} else {
			report.Executable = exe
		}
	}
	if includeStrings {
		report.Strings = PrintableStrings(data, 6)
	}
	return report
}

func Digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func PrintableStrings(data []byte, minimum int) []Text {
	if minimum < 1 {
		minimum = 1
	}
	var result []Text
	for start := 0; start < len(data); {
		if data[start] < 32 || data[start] > 126 {
			start++
			continue
		}
		end := start
		for end < len(data) && data[end] >= 32 && data[end] <= 126 {
			end++
		}
		value := string(bytes.TrimSpace(data[start:end]))
		if len(value) >= minimum {
			result = append(result, Text{FileOffset: start, Value: value})
		}
		start = end
	}
	return result
}

// Identify is conservative: exact known assets identify Populous 1, while the
// Populous II executable is identified by its own title and resource table.
// This identifies the program, not completeness of an installation.
func (r *Report) Identify() {
	r.Identity = "unknown"
	r.Populous1Matches = 0
	r.Evidence = nil
	matched := make(map[string]bool)
	for _, file := range r.Files {
		if file.Populous1 {
			matched[strings.ToLower(file.Path)] = true
		}
		if file.Executable == nil {
			continue
		}
		for _, hunk := range file.Executable.Hunks {
			if bytes.Contains(hunk.Data, []byte("POPULOUS II")) && bytes.Contains(hunk.Data, []byte("BLOCK0.PAK")) && bytes.Contains(hunk.Data, []byte("LAND0.DAT")) {
				r.Identity = "populous-2"
				r.Evidence = append(r.Evidence, file.Path+": POPULOUS II title, BLOCK0.PAK and LAND0.DAT resource table")
			}
		}
	}
	r.Populous1Matches = len(matched)
	if r.Populous1Matches == len(populous1Hashes) {
		r.Identity = "populous-1"
		r.Evidence = append(r.Evidence, fmt.Sprintf("all %d supplied Populous 1 data files match byte for byte (SHA-256)", len(populous1Hashes)))
	}
}
