package populous2

import (
	"encoding/binary"
	"fmt"
)

// NativeTileChunkRequest is one actual $bfac half-row blit. The source offset
// remains a BLOCK word and destination is a byte displacement from the real
// bitmap, so native traversal/height/pair ordering is not reconstructed here.
type NativeTileChunkRequest struct {
	SourceOffset      uint16
	DestinationOffset int
}

type NativeTileBitmapBank struct {
	Descriptors [TileCount][6]uint16
	Chunks      map[uint16]NativePreparedSprite
}

func DecodeNativeTileBitmapBank(raw []byte) (*NativeTileBitmapBank, error) {
	if len(raw) < TileCount*12 {
		return nil, fmt.Errorf("native BLOCK descriptors truncated")
	}
	b := &NativeTileBitmapBank{Chunks: make(map[uint16]NativePreparedSprite)}
	for tile := range b.Descriptors {
		for chunk := range b.Descriptors[tile] {
			offset := binary.BigEndian.Uint16(raw[tile*12+chunk*2:])
			b.Descriptors[tile][chunk] = offset
			if offset == 0 {
				continue
			}
			if int(offset) < TileCount*12 || int(offset) > len(raw)-80 || offset&1 != 0 {
				return nil, fmt.Errorf("native BLOCK tile%d chunk%d outside aligned resource", tile, chunk)
			}
			if _, ok := b.Chunks[offset]; ok {
				continue
			}
			// $1a3f0 writes four color planes then the complemented mask.
			// The cached canonical mask-first layout has identical word data.
			planes, err := PrepareNativeMaskedPlanes(raw[int(offset):int(offset)+80], 16, 8)
			if err != nil {
				return nil, err
			}
			b.Chunks[offset] = NativePreparedSprite{Width: 16, Height: 8, Planes: planes}
		}
	}
	return b, nil
}

// PaintChunk reproduces the $0fca minterm of eight16-bit rows in each plane.
// Source words and destination offsets remain unshifted as in $bfac; native
// pair advances and zero-offset skipping belong to the world-draw producer.
func (b *NativeTileBitmapBank) PaintChunk(request NativeTileChunkRequest, bitmap []byte) error {
	if b == nil || len(bitmap) != 32000 {
		return fmt.Errorf("native tile bitmap backing missing")
	}
	if request.SourceOffset == 0 {
		return nil
	}
	chunk, ok := b.Chunks[request.SourceOffset]
	if !ok {
		return fmt.Errorf("native tile chunk%x absent from retained resource", request.SourceOffset)
	}
	at := request.DestinationOffset
	if at < 0 || at&1 != 0 || at+7*40+2 > 8000 {
		return fmt.Errorf("native tile destination%x needs adjacent bitmap backing", at)
	}
	for row := 0; row < 8; row++ {
		mask := binary.BigEndian.Uint16(chunk.Planes[row*2:])
		if mask == 0 {
			continue
		}
		for plane := 0; plane < 4; plane++ {
			dest := plane*8000 + at + row*40
			old := binary.BigEndian.Uint16(bitmap[dest:])
			color := binary.BigEndian.Uint16(chunk.Planes[(plane+1)*16+row*2:])
			binary.BigEndian.PutUint16(bitmap[dest:], old&^mask|color&mask)
		}
	}
	return nil
}
