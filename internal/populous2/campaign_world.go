package populous2

import (
	"fmt"

	legacy "go-populous2/internal/legacy"
)

// NativeGameResult latches the original end-of-pass totals before another
// update resets them. Progress is applied once when leaving the result screen.
type NativeGameResult struct {
	Detected, Applied bool
	Eliminated        uint16
	Score             CampaignScore
	ScoreError        string
	Local, Opponent   CampaignResultStatistics
	Progress          CampaignProgress
}

func (w *World) initializeNativeCampaignStatistics() {
	m := w.runtimeMemory()
	for owner := uint8(1); owner <= 2; owner++ {
		a, _ := NativeDeityAddress(owner)
		if _, err := m.Write16(a+0x18, uint16(owner)); err != nil {
			panic(err)
		}
	}
}

// The shared $17e38 debit adds one weighted use per admitted command.
// Sculpting and lightning marker/cancel bypass that boundary entirely.
func (w *World) recordNativePowerUse(player int, id SpellID) {
	if player < 0 || player > 1 || id == RaiseLower || w.NativeFreeCommands != 0 || w.NativeGameMode == 8 {
		return
	}
	m := w.runtimeMemory()
	a, _ := NativeDeityAddress(uint8(player + 1))
	old, err := m.Read16(a + 0x138)
	if err != nil {
		panic(err)
	}
	if _, err := m.Write16(a+0x138, old+uint16(id)%6+1); err != nil {
		panic(err)
	}
}

func (w *World) nativeCampaignStatistics() ([3]CampaignResultStatistics, error) {
	var sides [3]CampaignResultStatistics
	for owner := uint8(1); owner <= 2; owner++ {
		s, err := ReadCampaignResultStatistics(owner, w.nativeCleanupMemory())
		if err != nil {
			return sides, err
		}
		sides[owner] = s
	}
	return sides, nil
}

func (w *World) detectNativeResult() {
	if w.NativeResult.Detected {
		return
	}
	sides, err := w.nativeCampaignStatistics()
	if err != nil {
		panic(err)
	}
	eliminated, ended := w.CampaignResult.Detect(w.NativeGameMode, sides)
	if !ended {
		return
	}
	if w.NativeProfileSide < 1 || w.NativeProfileSide > 2 {
		panic("native selected profile side outside1/2")
	}
	local, other := sides[w.NativeProfileSide], sides[3-w.NativeProfileSide]
	score, err := w.CampaignResult.Score(w.NativeClock, local, other)
	w.NativeResult = NativeGameResult{Detected: true, Eliminated: eliminated, Local: local, Opponent: other, Score: score}
	if err != nil {
		w.NativeResult.ScoreError = err.Error()
	}
}

func (w *World) ResultForLocalProfile() int {
	if !w.NativeResult.Detected {
		return legacy.ResultOngoing
	}
	if w.NativeResult.Eliminated == uint16(w.NativeProfileSide) {
		return legacy.ResultLost
	}
	return legacy.ResultWon
}

// AdvanceCampaign uses the verified score/outcome branch and retained bolt
// balance. Repeated UI actions or loading an applied result cannot award again.
func (w *World) AdvanceCampaign() (CampaignProgress, error) {
	r := &w.NativeResult
	if !r.Detected {
		return CampaignProgress{}, fmt.Errorf("native result is not available")
	}
	if r.Applied {
		return r.Progress, nil
	}
	if r.ScoreError != "" {
		return CampaignProgress{}, fmt.Errorf("%s", r.ScoreError)
	}
	m := w.nativeCleanupMemory()
	deity, _ := NativeDeityAddress(w.NativeProfileSide)
	if err := m.Write16(deity+0x58, w.Deity.Bolts); err != nil {
		return CampaignProgress{}, err
	}
	progress, err := w.CampaignResult.Progress(w.NativeGameMode, uint16(w.NativeProfileSide), r.Eliminated, uint16(w.Level.Number), r.Score.Value, m)
	if err != nil {
		return CampaignProgress{}, err
	}
	bolts, err := m.Read16(deity + 0x58)
	if err != nil {
		return CampaignProgress{}, err
	}
	w.Deity.Bolts = bolts
	r.Applied, r.Progress = true, progress
	return progress, nil
}
