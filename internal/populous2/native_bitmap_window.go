package populous2

import (
	"encoding/binary"
	"fmt"
)

// NativeBitmapWindow retains real RAM surrounding a nominal32000-byte
// bitmap. BitmapOffset identifies its native A0 origin within that backing;
// adjacent data is supplied by the owner, never generated as zero padding.
type NativeBitmapWindow struct {
	Bytes        []byte
	BitmapOffset int
}

func (w NativeBitmapWindow) word(offset int) (uint16, error) {
	at := w.BitmapOffset + offset
	if at < 0 || at&1 != 0 || at > len(w.Bytes)-2 {
		return 0, fmt.Errorf("native bitmap word displacement%x outside supplied RAM", offset)
	}
	return binary.BigEndian.Uint16(w.Bytes[at:]), nil
}

func (w NativeBitmapWindow) putWord(offset int, value uint16) error {
	at := w.BitmapOffset + offset
	if at < 0 || at&1 != 0 || at > len(w.Bytes)-2 {
		return fmt.Errorf("native bitmap word displacement%x outside supplied RAM", offset)
	}
	binary.BigEndian.PutUint16(w.Bytes[at:], value)
	return nil
}

// PaintChunkWindow executes the real plane/row DMA order, including writes
// crossing a plane boundary or the nominal end. Only supplied address backing
// limits the operation; source offsets and destination displacements persist.
func (b *NativeTileBitmapBank) PaintChunkWindow(request NativeTileChunkRequest, window NativeBitmapWindow) error {
	if b == nil || window.BitmapOffset < 0 || window.BitmapOffset > len(window.Bytes)-32000 {
		return fmt.Errorf("native tile bitmap window missing")
	}
	if request.SourceOffset == 0 {
		return nil
	}
	chunk, ok := b.Chunks[request.SourceOffset]
	if !ok {
		return fmt.Errorf("native tile chunk%x absent from retained resource", request.SourceOffset)
	}
	for plane := 0; plane < 4; plane++ {
		for row := 0; row < 8; row++ {
			offset := plane*8000 + request.DestinationOffset + row*40
			old, err := window.word(offset)
			if err != nil {
				return err
			}
			mask := binary.BigEndian.Uint16(chunk.Planes[row*2:])
			color := binary.BigEndian.Uint16(chunk.Planes[(plane+1)*16+row*2:])
			if err := window.putWord(offset, old&^mask|color&mask); err != nil {
				return err
			}
		}
	}
	return nil
}
