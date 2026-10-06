package populous2

// nativeFileBrowserBacking uses the physical source address space for
// compiler parameters. A list string belongs to BSS even when a parameter
// slot itself belongs to CODE; null operands retain physical address zero.
func nativeFileBrowserBacking(cb NativeFileFrameCallbacks) nativeRequesterFrameBacking {
	code := cb.Code
	if cb.ReadAbsolute != nil {
		original := code
		code = nativeByteAddressMemory(func(at int) (byte, error) {
			return cb.ReadAbsolute(uint32(int64(cb.CodeBase) + int64(at)))
		}, original.Write8)
	}
	return nativeRequesterFrameBacking{Code: code, Memory: cb.Memory, CodeBase: cb.CodeBase, Frame: cb.Frame, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: cb.ReadAbsolute}
}

func nativeFileBrowserCompile(b nativeRequesterFrameBacking, a *[7]NativeRequesterAddress, definition, parameter uint32) error {
	if e := campaignRequesterCompile(b, a, definition, parameter); e != nil {
		return e
	}
	if a[3].Address >= b.Frame.AddressBase && a[3].Address < b.Frame.AddressBase+0x11280 {
		a[3].Code = false
	}
	return nil
}
