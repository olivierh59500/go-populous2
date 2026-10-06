package gamcodec

import (
	"fmt"
	"go-populous2/internal/engine"
)

// These are file-format drawing identities, not controller addresses. The
// runtime stores named classes so retained slot identities remain meaningful.
var inspectionFileKinds = [...]uint8{0, 32, 34, 36, 38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58}

func decodeInspectionClass(kind uint8) (engine.EffectInspectionClass, error) {
	for class, value := range inspectionFileKinds {
		if value == kind {
			return engine.EffectInspectionClass(class), nil
		}
	}
	return engine.InspectUnspecified, fmt.Errorf("unsupported effect inspection class in GAM")
}

func encodeInspectionClass(class engine.EffectInspectionClass) (uint8, error) {
	if int(class) >= len(inspectionFileKinds) {
		return 0, fmt.Errorf("invalid effect inspection class")
	}
	return inspectionFileKinds[class], nil
}
