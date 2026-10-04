package populous2

import "fmt"

type HeroLinkCleanupStep struct {
	Patches                                             []NativeRecordImagePatch
	CaptivesVisited                                     int
	OutgoingCleared, TargetBackCleared, IncomingCleared bool
}

// ClearEntryHeroLinks translates $14654 only in Entry's A0=A2=source context
// ($12914/$12916). General $124a2 cleanup can leave A0 pointing at a deity or
// another actor and requires a different explicit register-context adapter.
//
// The routine follows raw word aliases, including inactive/reused records.
// It neither checks population/owner nor changes map membership. Patches keep
// original assignment order so the caller can hydrate every affected slot.
func ClearEntryHeroLinks(image *NativeRecordImage, source NativeRecordReference) (HeroLinkCleanupStep, error) {
	var step HeroLinkCleanupStep
	write16 := func(reference NativeRecordReference, offset int, value uint16) error {
		patch, err := image.Write16(reference, offset, value)
		if err == nil {
			step.Patches = append(step.Patches, patch)
		}
		return err
	}
	write8 := func(reference NativeRecordReference, offset int, value uint8) error {
		patch, err := image.Write8(reference, offset, value)
		if err == nil {
			step.Patches = append(step.Patches, patch)
		}
		return err
	}
	flags, err := image.Read8(source, 0x0d)
	if err != nil {
		return step, err
	}
	if flags&2 != 0 {
		association, err := image.Read16(source, 0x22)
		if err != nil {
			return step, err
		}
		if association != 0 {
			target := NativeRecordReference(association)
			back, err := image.Read16(target, 0x24)
			if err != nil {
				return step, err
			}
			if back == uint16(source) {
				if err := write16(target, 0x24, 0); err != nil {
					return step, err
				}
				step.TargetBackCleared = true
			}
			if err := write16(source, 0x22, 0); err != nil {
				return step, err
			}
			step.OutgoingCleared = true
		}
		hero, err := image.Read16(source, 0x28)
		if err != nil {
			return step, err
		}
		if hero == 10 {
			head, err := image.Read16(source, 0x2a)
			if err != nil {
				return step, err
			}
			for head != 0 {
				// Native cycles normally terminate because the next word is
				// cleared before revisiting it. Preserve repeated visits rather
				// than rejecting the first repeat or rebuilding a Go captive list.
				if step.CaptivesVisited >= NativeRecordImageSize*2 {
					return step, fmt.Errorf("native captive cleanup exceeds bounded iteration window")
				}
				reference := NativeRecordReference(head)
				next, err := image.Read16(reference, 0x2a)
				if err != nil {
					return step, err
				}
				if err := write16(reference, 0x2a, 0); err != nil {
					return step, err
				}
				flags, err := image.Read8(reference, 0x0d)
				if err != nil {
					return step, err
				}
				if err := write8(reference, 0x0d, flags&^8); err != nil {
					return step, err
				}
				if err := write8(reference, 0x16, 2); err != nil {
					return step, err
				}
				step.CaptivesVisited++
				head = next
			}
		}
	}
	back, err := image.Read16(source, 0x24)
	if err != nil {
		return step, err
	}
	if back != 0 {
		reference := NativeRecordReference(back)
		forward, err := image.Read16(reference, 0x22)
		if err != nil {
			return step, err
		}
		if forward == uint16(source) {
			if err := write16(reference, 0x22, 0); err != nil {
				return step, err
			}
			step.IncomingCleared = true
		}
	}
	return step, nil
}
