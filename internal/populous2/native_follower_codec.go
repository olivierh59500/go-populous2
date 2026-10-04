package populous2

// ReadFollowerEntry reads the exact 52-byte field layout used by the native
// contact handlers. Raw aliases are deliberately not restricted to aligned
// follower references; the bounded image owns that memory-safety policy.
func (image *NativeRecordImage) ReadFollowerEntry(ref NativeRecordReference) (FollowerEntryActor, error) {
	var actor FollowerEntryActor
	read8 := func(offset int, target *uint8) error {
		value, err := image.Read8(ref, offset)
		*target = value
		return err
	}
	for _, entry := range []struct {
		offset int
		target *uint8
	}{
		{0, &actor.Motion.Kind}, {1, &actor.Byte1}, {12, &actor.Owner}, {13, &actor.Motion.Flags},
		{18, &actor.Motion.Speed}, {19, &actor.Byte19}, {22, &actor.Motion.State}, {23, &actor.Motion.ReturnState}, {25, &actor.Weapon},
	} {
		if err := read8(entry.offset, entry.target); err != nil {
			return actor, err
		}
	}
	read16 := func(offset int) (uint16, error) { return image.Read16(ref, offset) }
	for _, entry := range []struct {
		offset int
		target *uint16
	}{
		{2, &actor.Motion.Next}, {4, &actor.Motion.Previous}, {30, &actor.Contact30}, {34, &actor.Association34},
		{36, &actor.AssociationBack36}, {40, &actor.Hero40}, {42, &actor.Captive42}, {44, &actor.CaptiveBack44},
		{46, &actor.Founded46}, {48, &actor.Extra48}, {50, &actor.Motion.Variant},
	} {
		value, err := read16(entry.offset)
		if err != nil {
			return actor, err
		}
		*entry.target = value
	}
	for _, entry := range []struct {
		offset int
		target *int16
	}{
		{6, &actor.Motion.X}, {8, &actor.Motion.Y}, {14, &actor.Motion.VX}, {16, &actor.Motion.VY}, {20, &actor.Motion.Timer},
	} {
		value, err := read16(entry.offset)
		if err != nil {
			return actor, err
		}
		*entry.target = int16(value)
	}
	animation, err := read16(10)
	if err != nil {
		return actor, err
	}
	actor.Motion.Animation = int(animation)
	population, err := image.Read32(ref, 26)
	if err != nil {
		return actor, err
	}
	actor.Motion.Population = int32(population)
	actor.Motion.Player = actor.Owner - 1
	return actor, nil
}

// PatchFollowerEntry writes only changed, modeled fields. It preserves the
// unmodeled byte24 and word32. Patches report physical overlap so the World
// can hydrate aliases before running the next affected controller.
func (image *NativeRecordImage) PatchFollowerEntry(ref NativeRecordReference, previous, next FollowerEntryActor) ([]NativeRecordImagePatch, error) {
	var patches []NativeRecordImagePatch
	for _, entry := range []struct {
		offset        int
		before, after uint8
	}{
		{0, previous.Motion.Kind, next.Motion.Kind}, {1, previous.Byte1, next.Byte1}, {12, previous.Owner, next.Owner}, {13, previous.Motion.Flags, next.Motion.Flags},
		{18, previous.Motion.Speed, next.Motion.Speed}, {19, previous.Byte19, next.Byte19}, {22, previous.Motion.State, next.Motion.State}, {23, previous.Motion.ReturnState, next.Motion.ReturnState}, {25, previous.Weapon, next.Weapon},
	} {
		if entry.before == entry.after {
			continue
		}
		p, err := image.Write8(ref, entry.offset, entry.after)
		if err != nil {
			return patches, err
		}
		patches = append(patches, p)
	}
	for _, entry := range []struct {
		offset        int
		before, after uint16
	}{
		{2, previous.Motion.Next, next.Motion.Next}, {4, previous.Motion.Previous, next.Motion.Previous}, {6, uint16(previous.Motion.X), uint16(next.Motion.X)},
		{8, uint16(previous.Motion.Y), uint16(next.Motion.Y)}, {10, uint16(previous.Motion.Animation), uint16(next.Motion.Animation)},
		{14, uint16(previous.Motion.VX), uint16(next.Motion.VX)}, {16, uint16(previous.Motion.VY), uint16(next.Motion.VY)}, {20, uint16(previous.Motion.Timer), uint16(next.Motion.Timer)},
		{30, previous.Contact30, next.Contact30}, {34, previous.Association34, next.Association34}, {36, previous.AssociationBack36, next.AssociationBack36},
		{40, previous.Hero40, next.Hero40}, {42, previous.Captive42, next.Captive42}, {44, previous.CaptiveBack44, next.CaptiveBack44},
		{46, previous.Founded46, next.Founded46}, {48, previous.Extra48, next.Extra48}, {50, previous.Motion.Variant, next.Motion.Variant},
	} {
		if entry.before == entry.after {
			continue
		}
		p, err := image.Write16(ref, entry.offset, entry.after)
		if err != nil {
			return patches, err
		}
		patches = append(patches, p)
	}
	if previous.Motion.Population != next.Motion.Population {
		p, err := image.Write32(ref, 26, uint32(next.Motion.Population))
		if err != nil {
			return patches, err
		}
		patches = append(patches, p)
	}
	return patches, nil
}
