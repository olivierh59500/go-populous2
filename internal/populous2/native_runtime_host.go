package populous2

import (
	"fmt"
	"io/fs"
)

// NativeRuntimeHostConfig supplies explicit numeric load addresses. These
// labels describe the Go host layout, not an inferred AmigaOS allocation.
type NativeRuntimeHostConfig struct {
	HunkBases                        []uint32
	Regions                          []NativeHostRegion
	AllocationStart, AllocationLimit uint32
	Level                            int
	Custom                           bool
}

// NativeRuntimeHost keeps a single owner for original resource bytes, live
// World BSS, input, screens and relocated CODE. Construction does not claim
// that source startup has run: InitializePresentation, resource allocation
// and the retained startup controller are separate real operations.
type NativeRuntimeHost struct {
	// Access owns complete native operations and PCM reads. Do not copy
	// the runtime or access borrowed fields concurrently outside Execute.
	Access             NativeRuntimeAccess
	Bundle             *Bundle
	Host               *NativeHostMemory
	Memory             *NativeSessionMemory
	Code               *NativeSharedCode
	PresentationCode   *NativePresentationCodeAlias
	ImageAudioCode     *NativeImageAudioCodeAlias
	World              *World
	Session            *NativeFrameSession
	Allocator          *NativeHostAllocator
	Files              *NativeResourceFilesystem
	ResourceRules      NativeResourceFrameRules
	allocation         NativeStartupAllocationState
	allocationResource *NativeResourceHostFrameState
}

func NewNativeRuntimeHost(bundle *Bundle, files fs.FS, config NativeRuntimeHostConfig) (*NativeRuntimeHost, error) {
	if bundle == nil || bundle.Executable == nil || len(bundle.Executable.Hunks) != 6 || len(config.HunkBases) != 6 {
		return nil, fmt.Errorf("native runtime original HUNK layout missing")
	}
	host, err := NewNativeHunkMemory(bundle.Executable, config.HunkBases)
	if err != nil {
		return nil, err
	}
	for _, region := range config.Regions {
		if err := host.MapRegion(region); err != nil {
			return nil, err
		}
	}
	allocator, err := NewNativeHostAllocator(host, config.AllocationStart, config.AllocationLimit)
	if err != nil {
		return nil, err
	}
	disk, err := NewNativeResourceFilesystem(files)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*NativeRuntimeHost, error) { _ = disk.Close(); return nil, err }
	world, err := NewWorld(bundle, config.Level, config.Custom)
	if err != nil {
		return fail(err)
	}
	session, err := NewNativeFrameSession(bundle, world.Level.Terrain, config.HunkBases[4], config.HunkBases[3])
	if err != nil {
		return fail(err)
	}
	physicalCode, err := host.Span(config.HunkBases[0], int(bundle.Executable.Hunks[0].AllocatedBytes))
	if err != nil {
		return fail(err)
	}
	code, err := NewNativeSharedCode(bundle.Executable, physicalCode, config.HunkBases)
	if err != nil {
		return fail(err)
	}
	// World routines read scalar/table data directly. Linked procedure
	// consumers must explicitly use Code.Logical(), never a copied array.
	world.NativeAI.Code = code.RawData()
	world.nativeSharedCode = code
	memory, err := NewNativeSessionMemory(host, session, world, config.HunkBases[0], config.HunkBases[1])
	if err != nil {
		return fail(err)
	}
	// Start with the actual zero-initialized HUNK1, not typed prototype state.
	if err := memory.ImportBSS(); err != nil {
		return fail(err)
	}
	presentationCode, err := NewNativePresentationCodeAlias(session.Presentation, memory.RAM, config.HunkBases[0])
	if err != nil {
		return fail(err)
	}
	memory.RAM, memory.Code = presentationCode.RAM, presentationCode.Code
	imageAudioCode, err := NewNativeImageAudioCodeAlias(&session.Image, &session.Audio, memory.RAM, config.HunkBases[0], NativeImageCodeOwner)
	if err != nil {
		return fail(err)
	}
	memory.RAM, memory.Code = imageAudioCode.RAM, imageAudioCode.Code
	session.imageAudioCode = imageAudioCode
	session.requireInput = true
	rules, err := DecodeNativeResourceFrameRules(bundle.Executable)
	if err != nil {
		return fail(err)
	}
	return &NativeRuntimeHost{Bundle: bundle, Host: host, Memory: memory, Code: code, PresentationCode: presentationCode, ImageAudioCode: imageAudioCode, World: world, Session: session, Allocator: allocator, Files: disk, ResourceRules: rules}, nil
}

func (h *NativeRuntimeHost) Close() error {
	if h == nil || h.Files == nil {
		return nil
	}
	return h.Files.Close()
}

func (h *NativeRuntimeHost) Bitmap(address uint32) ([]byte, error) {
	if h == nil || h.Host == nil {
		return nil, fmt.Errorf("native runtime bitmap owner missing")
	}
	return h.Host.Span(address, 32000)
}

// InitializePresentation executes the proven source Copper builders against
// the shared chip allocation. It does not substitute for initial menus or
// the source world constructor.
func (h *NativeRuntimeHost) InitializePresentation(sample NativeMouseSample) ([]NativeFrameHardwareWrite, error) {
	if h == nil || h.Session == nil {
		return nil, fmt.Errorf("native runtime presentation missing")
	}
	return h.Session.Presentation.Initialize(h.Bundle.Executable, sample)
}

// ResourceCallbacks binds actual encoded filesystem reads and physical
// addresses to the retained loader/error requester. Sound and other genuine
// child operations remain supplied by the caller.
func (h *NativeRuntimeHost) ResourceCallbacks(frame *NativeFrameRegisterContext, supplied NativeErrorFrameCallbacks) (NativeResourceHostFrameCallbacks, error) {
	if h == nil || h.Memory == nil || h.Session == nil || h.Files == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase {
		return NativeResourceHostFrameCallbacks{}, fmt.Errorf("native runtime resource context/base missing")
	}
	supplied.Frame, supplied.Memory, supplied.Code = frame, h.Memory.BSS, h.Memory.Code
	supplied.CodeBase, supplied.RAM = h.Memory.CodeBase, h.Memory.RAM
	supplied.Presentation, supplied.Bitmap = h.Session.Presentation, h.Bitmap
	supplied.ReadAbsolute = func(address uint32) (uint8, error) { return h.Memory.RAM.Read8(int(address)) }
	return NativeResourceHostFrameCallbacks{NativeErrorFrameCallbacks: supplied, IO: h.Files.IO}, nil
}

// AdvanceAllocations executes actual1A43E/1A4BC with real FX/QAZ loading.
// A loader error keeps its requester pending; resumption retains the same
// allocation and does not run an earlier completed load again.
func (h *NativeRuntimeHost) AdvanceAllocations(routine int, frame *NativeFrameRegisterContext, supplied NativeErrorFrameCallbacks) (bool, error) {
	if h == nil || h.Memory == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase {
		return false, fmt.Errorf("native runtime allocation context missing")
	}
	if h.allocation.Complete && h.allocation.Routine != routine {
		h.allocation = NativeStartupAllocationState{}
	}
	cb := NativeStartupAllocationCallbacks{Code: h.Memory.Code, Memory: h.Memory.BSS, CodeBase: h.Memory.CodeBase, Frame: frame, ReadExecBase: func() (uint32, error) { return h.Memory.RAM.Read32(4) }, Allocate: h.Allocator.Allocate}
	cb.Call = func(call NativeFileFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x19cd0 {
			return NativeCommandFrameResult{}, fmt.Errorf("native runtime allocation child%x unsupported", call.Routine)
		}
		if h.allocationResource == nil {
			h.allocationResource = &NativeResourceHostFrameState{}
		}
		resource, err := h.ResourceCallbacks(call.Frame, supplied)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		resource.ResourceCallerA = call.A
		step, err := h.allocationResource.Advance(&h.ResourceRules, resource)
		if step.Complete {
			h.allocationResource = nil
		}
		return NativeCommandFrameResult{Complete: step.Complete}, err
	}
	return h.allocation.Advance(routine, cb)
}
