package populous2

import (
	"bytes"
	"fmt"
)

type NativeFileRequesterMode uint8

const (
	NativeFileLoad NativeFileRequesterMode = iota
	NativeFileSave
)

type NativeFileRequesterModal uint8

const (
	NativeFileBrowser NativeFileRequesterModal = iota
	NativeFileMessage
	NativeFileOverwrite
)

type NativeFileEditField uint8

const (
	NativeFileEditNone NativeFileEditField = iota
	NativeFileEditDrawer
	NativeFileEditName
)

// NativeFileEntry separates portable host identity from the Amiga display
// name. The original DOS lookup is case-insensitive; host callbacks retain
// responsibility for resolving the emitted native path on their filesystem.
type NativeFileEntry struct {
	Name []byte
	File bool
}
type NativeFileRequesterCallbacks struct {
	List   func([]byte) ([]NativeFileEntry, error)
	Exists func([]byte) (bool, error)
	Load   func([]byte) error
	Save   func([]byte) error
}

type NativeFileRequester struct {
	Presentation     *NativePresentation
	Mode             NativeFileRequesterMode
	Drawer, Filename []byte
	Files            [][]byte
	Offset, Selected uint16
	Modal            NativeFileRequesterModal
	Editing          NativeFileEditField
	Done, Cancelled  bool
	LastError        error
	callbacks        NativeFileRequesterCallbacks
	pendingPath      []byte
}

func NewNativeFileRequester(p *NativePresentation, mode NativeFileRequesterMode, drawer, filename []byte, cb NativeFileRequesterCallbacks) (*NativeFileRequester, error) {
	if p == nil || mode > NativeFileSave || cb.List == nil {
		return nil, fmt.Errorf("native file requester inputs missing")
	}
	r := &NativeFileRequester{Presentation: p, Mode: mode, Drawer: nativeFileCString(drawer), Filename: nativeFileCString(filename), callbacks: cb}
	if err := r.Refresh(); err != nil {
		return nil, err
	}
	return r, nil
}

func nativeFileCString(value []byte) []byte {
	if end := bytes.IndexByte(value, 0); end >= 0 {
		value = value[:end]
	}
	return append([]byte(nil), value...)
}

// FilterNativeFileEntries follows $19936. Directories and non-.GAM files are
// excluded before native byte uppercasing. Enumeration order is preserved;
// the 2528-byte admission check occurs before copying each accepted name.
func FilterNativeFileEntries(entries []NativeFileEntry) [][]byte {
	files := [][]byte{}
	used := 0
	for _, entry := range entries {
		name := nativeFileCString(entry.Name)
		if !entry.File || !bytes.HasSuffix(name, []byte(".GAM")) || used >= 0xd9e-0x3be {
			continue
		}
		for index, value := range name {
			if int8(value) > 0x40 {
				name[index] = value & 0x5f
			}
		}
		files = append(files, name)
		used += len(name) + 1
	}
	return files
}

// Refresh is the directory re-entry at $3f92. A listing failure leaves the
// original empty directory list, while retaining the concrete host error.
func (r *NativeFileRequester) Refresh() error {
	if r == nil || r.callbacks.List == nil {
		return fmt.Errorf("native directory callback missing")
	}
	entries, err := r.callbacks.List(nativeFileCString(r.Drawer))
	r.Offset, r.Selected = 0, 0
	r.Files = FilterNativeFileEntries(entries)
	r.LastError = err
	return nil
}

func (r *NativeFileRequester) visible() [][]byte {
	start := int(r.Offset)
	if start >= len(r.Files) {
		return nil
	}
	return r.Files[start:min(start+12, len(r.Files))]
}

// Plan returns the original compiled text geometry. The two edit buffers are
// non-NULL even when empty, so their native glyph 'k' padding is opaque.
func (r *NativeFileRequester) Plan() (*NativeRequester, error) {
	if r == nil || r.Presentation == nil {
		return nil, fmt.Errorf("native file presentation missing")
	}
	if r.Modal == NativeFileMessage {
		return r.Presentation.Compile(NativeMenuMessage, [][]byte{[]byte("ERREUR FICHIER ")})
	}
	if r.Modal == NativeFileOverwrite {
		return r.Presentation.Compile(NativeMenuOverwrite, [][]byte{r.Filename})
	}
	verb := []byte("CHAR")
	if r.Mode == NativeFileSave {
		verb = []byte("SAUV")
	}
	parameters := make([][]byte, 16)
	parameters[0], parameters[15] = verb, verb
	for index, name := range r.visible() {
		parameters[index+1] = name
	}
	parameters[13], parameters[14] = r.Drawer, r.Filename
	// The original argument table contains a non-NULL pointer for each edit
	// buffer, including an empty one. The shared text compiler uses slice
	// length to distinguish a missing argument; one padding glyph preserves
	// both the native opaque empty field and advancement to the next argument.
	for _, index := range []int{13, 14} {
		if len(parameters[index]) == 0 {
			parameters[index] = []byte{'k'}
		}
	}
	plan, err := r.Presentation.Compile(NativeMenuFiles, parameters)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// Click uses the verified native glyph-marker regions and event IDs. Text
// editing remains an explicit asynchronous operation completed by FinishEdit.
func (r *NativeFileRequester) Click(x, y int) error {
	plan, err := r.Plan()
	if err != nil {
		return err
	}
	return r.Action(r.Presentation.Requesters.Click(plan, x, y))
}

func (r *NativeFileRequester) Action(action int) error {
	if r == nil {
		return fmt.Errorf("native file requester missing")
	}
	if r.Done || r.Cancelled {
		return nil
	}
	if r.Modal == NativeFileMessage {
		if action == 2 {
			r.Modal = NativeFileBrowser
		}
		return nil
	}
	if r.Modal == NativeFileOverwrite {
		switch action {
		case 2:
			return r.perform()
		case 4:
			r.Modal = NativeFileBrowser
			r.pendingPath = nil
		}
		return nil
	}
	switch {
	case action == 0:
		return nil
	case action == 2:
		if r.Offset != 0 {
			r.Offset--
		}
	case action >= 4 && action <= 26 && action&1 == 0:
		row := (action - 4) / 2
		visible := r.visible()
		r.Filename = nil
		r.Selected = uint16(row)
		if row < len(visible) {
			r.Filename = nativeFileCString(visible[row])
		}
	case action == 28:
		if maximum := len(r.Files) - 12; maximum > 0 {
			r.Offset = uint16(min(int(r.Offset)+1, maximum))
		}
	case action == 30:
		r.Editing = NativeFileEditDrawer
	case action == 32:
		r.Editing = NativeFileEditName
	case action == 34:
		return r.submit()
	case action == 36:
		r.Cancelled = true
	default:
		return fmt.Errorf("native file action%d outside original dispatcher", action)
	}
	return nil
}

func (r *NativeFileRequester) FinishEdit(value []byte) error {
	if r == nil {
		return fmt.Errorf("native file requester missing")
	}
	value = nativeFileCString(value)
	switch r.Editing {
	case NativeFileEditDrawer:
		r.Drawer = value
		r.Editing = NativeFileEditNone
		return r.Refresh()
	case NativeFileEditName:
		r.Filename = value
		r.Editing = NativeFileEditNone
		return nil
	default:
		return fmt.Errorf("native file requester is not editing")
	}
}

func (r *NativeFileRequester) failure(err error) error {
	r.LastError = err
	r.Modal = NativeFileMessage
	return nil
}

func (r *NativeFileRequester) submit() error {
	r.pendingPath = NativeGAMPath(r.Drawer, r.Filename)
	// $420e also appends the suffix into the persistent filename buffer.
	r.Filename = NativeGAMPath(nil, r.Filename)
	if r.Mode == NativeFileSave {
		if r.callbacks.Exists == nil {
			return fmt.Errorf("native overwrite probe callback missing")
		}
		exists, err := r.callbacks.Exists(r.pendingPath)
		if err != nil {
			// $19b38 treats every unsuccessful old-file open as absent and
			// still attempts the actual write. Only that transfer can fail
			// into the file-error requester.
			r.LastError = err
			exists = false
		}
		if exists {
			r.Modal = NativeFileOverwrite
			return nil
		}
	}
	return r.perform()
}

func (r *NativeFileRequester) perform() error {
	var err error
	if r.Mode == NativeFileLoad {
		if r.callbacks.Load == nil {
			return fmt.Errorf("native load callback missing")
		}
		err = r.callbacks.Load(nativeFileCString(r.pendingPath))
	} else {
		if r.callbacks.Save == nil {
			return fmt.Errorf("native save callback missing")
		}
		err = r.callbacks.Save(nativeFileCString(r.pendingPath))
	}
	if err != nil {
		return r.failure(err)
	}
	r.Done = true
	r.Modal = NativeFileBrowser
	return nil
}
