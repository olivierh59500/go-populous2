package gamcodec

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/engine"
)

// Original save files express the two presentation selections as byte
// distances in their fixed-width follower records. Runtime UI state uses IDs.
func decodeSelectionMetadata(r fileReader, m *Metadata) error {
	follower := func(at int) (int, error) {
		value := r.long(at)
		if value%52 != 0 || value/52 >= engine.FollowerCapacity {
			return 0, fmt.Errorf("GAM selected follower is outside the file pool")
		}
		return int(value / 52), nil
	}
	var err error
	if m.SelectedFollower, err = follower(0xf36); err != nil {
		return err
	}
	if m.BackupFollower, err = follower(0xf32); err != nil {
		return err
	}
	m.SelectionReturnFrames = r.word(0xf30)
	if m.SelectionReturnFrames > 100 {
		return fmt.Errorf("GAM selection return exceeds 100 main frames")
	}
	m.initialSelectedFollower, m.initialBackupFollower = m.SelectedFollower, m.BackupFollower
	m.initialSelectionReturnFrames = m.SelectionReturnFrames
	return nil
}

func encodeSelectionMetadata(data []byte, m Metadata) error {
	if m.SelectedFollower < 0 || m.SelectedFollower >= engine.FollowerCapacity || m.BackupFollower < 0 || m.BackupFollower >= engine.FollowerCapacity || m.SelectionReturnFrames > 100 {
		return fmt.Errorf("GAM selection presentation state is invalid")
	}
	binary.BigEndian.PutUint16(data[0xf30-fileStart:], m.SelectionReturnFrames)
	binary.BigEndian.PutUint32(data[0xf32-fileStart:], uint32(m.BackupFollower*52))
	binary.BigEndian.PutUint32(data[0xf36-fileStart:], uint32(m.SelectedFollower*52))
	return nil
}
