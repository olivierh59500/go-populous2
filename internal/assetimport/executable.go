package assetimport

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

const (
	FrenchExecutableSHA256   = "c148b9bdcc543d89bf788d86886eb5ac9320ad66b09ef92792ec24314bfee803"
	DiskExecutableSHA256     = "f5fa1a02d6fe7bd4f298498d0bcbcc3ebc03b83b102dbe170781114bfcdfa271"
	RepairedExecutableSHA256 = "4fa6fe0ff7960100a3a007eef2d8ad6ab3f86e67adf480cf6ddfc0fdc5b73a45"
)

// prepareExecutable accepts only identified original revisions. The recognized
// boot-disk file omits HUNK_END after its CODE relocation groups; restore that
// structural separator locally, without supplying code, text or graphics bytes.
func prepareExecutable(data []byte) ([]byte, error) {
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	switch digest {
	case FrenchExecutableSHA256, RepairedExecutableSHA256:
		if _, err := amiga.ParseExecutable(data); err != nil {
			return nil, err
		}
		return data, nil
	case DiskExecutableSHA256:
		repaired, err := insertHunkEnd(data, 0x45030)
		if err != nil {
			return nil, err
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(repaired)); got != RepairedExecutableSHA256 {
			return nil, fmt.Errorf("boot executable repair produced an unexpected fingerprint %s", got)
		}
		if _, err := amiga.ParseExecutable(repaired); err != nil {
			return nil, err
		}
		return repaired, nil
	default:
		return nil, fmt.Errorf("unsupported populous.ii revision (SHA-256 %s); use the identified original boot disk or compatible French executable listed in docs/ASSET_SETUP.md", digest)
	}
}

func insertHunkEnd(data []byte, at int) ([]byte, error) {
	if at < 0 || at%4 != 0 || at > len(data)-4 || binary.BigEndian.Uint32(data[at:]) != amiga.HunkBSS {
		return nil, fmt.Errorf("missing expected BSS boundary for HUNK_END repair")
	}
	var separator [4]byte
	binary.BigEndian.PutUint32(separator[:], 1010) // HUNK_END.
	repaired := make([]byte, 0, len(data)+4)
	repaired = append(repaired, data[:at]...)
	repaired = append(repaired, separator[:]...)
	repaired = append(repaired, data[at:]...)
	return repaired, nil
}
