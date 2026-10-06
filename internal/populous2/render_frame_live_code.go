package populous2

import (
	"encoding/binary"
	"fmt"
)

// nativeRenderCodeView distinguishes canonical data bytes from the linked
// descriptor procedure LONG. Raw word/byte aliases never receive an inferred
// relocation subtraction. The zero value retains legacy immutable tables.
type nativeRenderCodeView struct {
	data          FollowerCleanupMemory
	readProcedure func(int) (uint32, error)
}

func (v nativeRenderCodeView) bounds(original []byte, at, size int) bool {
	return at >= 0 && size >= 0 && (v.data.Read8 != nil || at <= len(original)-size)
}

func (v nativeRenderCodeView) byte(original []byte, at int) (uint8, error) {
	if !v.bounds(original, at, 1) {
		return 0, fmt.Errorf("native rendering CODE byte %#x unavailable", at)
	}
	if v.data.Read8 != nil {
		return v.data.Read8(at)
	}
	return original[at], nil
}

func (v nativeRenderCodeView) word(original []byte, at int) (uint16, error) {
	if at&1 != 0 || !v.bounds(original, at, 2) {
		return 0, fmt.Errorf("native rendering CODE word %#x unavailable/unaligned", at)
	}
	if v.data.Read16 != nil {
		return v.data.Read16(at)
	}
	return binary.BigEndian.Uint16(original[at:]), nil
}

func (v nativeRenderCodeView) long(original []byte, at int) (uint32, error) {
	if at&1 != 0 || !v.bounds(original, at, 4) {
		return 0, fmt.Errorf("native rendering CODE long %#x unavailable/unaligned", at)
	}
	if v.data.Read32 != nil {
		return v.data.Read32(at)
	}
	return binary.BigEndian.Uint32(original[at:]), nil
}

func (v nativeRenderCodeView) procedure(original []byte, at int) (uint32, error) {
	if v.readProcedure != nil {
		return v.readProcedure(at)
	}
	return v.long(original, at)
}

func nativeRenderCodeBinding(data FollowerCleanupMemory, readProcedure func(int) (uint32, error)) (nativeRenderCodeView, error) {
	if data.Read8 == nil || data.Read16 == nil || data.Read32 == nil || readProcedure == nil {
		return nativeRenderCodeView{}, fmt.Errorf("native live rendering data/procedure backing missing")
	}
	return nativeRenderCodeView{data: data, readProcedure: readProcedure}, nil
}

// BindCode uses live canonical CODE for scalar/table data and an explicit
// linked-offset reader only for descriptor procedure LONGs. It borrows both
// views; no per-frame CODE copy is constructed. Existing Image state remains
// caller-owned and shared with the audio scheduler.
func (r *NativeRenderFrameRules) BindCode(data FollowerCleanupMemory, readProcedure func(int) (uint32, error)) error {
	if r == nil {
		return fmt.Errorf("native live render rules missing")
	}
	v, err := nativeRenderCodeBinding(data, readProcedure)
	if err != nil {
		return err
	}
	r.view = v
	r.Images.view = v
	return nil
}

func (r *NativeEditorCursorRules) BindCode(data FollowerCleanupMemory, readProcedure func(int) (uint32, error)) error {
	if r == nil {
		return fmt.Errorf("native live image rules missing")
	}
	v, err := nativeRenderCodeBinding(data, readProcedure)
	if err != nil {
		return err
	}
	r.view = v
	return nil
}

func (r *NativeRenderFrameRules) byte(at int) (uint8, error)  { return r.view.byte(r.code, at) }
func (r *NativeRenderFrameRules) long(at int) (uint32, error) { return r.view.long(r.code, at) }
func (r *NativeRenderFrameRules) procedure(at int) (uint32, error) {
	return r.view.procedure(r.code, at)
}
func (r *NativeEditorCursorRules) byte(at int) (uint8, error) { return r.view.byte(r.code, at) }
func (r *NativeEditorCursorRules) procedure(at int) (uint32, error) {
	return r.view.procedure(r.code, at)
}
