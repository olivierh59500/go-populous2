package populous2

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeFileRequesterAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/file_requester_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name, Drawer, Filename string
				Save                   bool
				Offset, Action         uint16
				Entries                []struct {
					Name string
					File bool
				}
			}
			Files                               []string
			BeforeTextHex, Filename, Path, Stop string
			Offset, Selected                    uint16
			D0                                  uint32
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 160 {
		t.Fatal("native file requester fixture catalog incomplete")
	}
	p, err := DecodeNativePresentation(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			mode := NativeFileLoad
			if input.Save {
				mode = NativeFileSave
			}
			entries := []NativeFileEntry{}
			for _, entry := range input.Entries {
				entries = append(entries, NativeFileEntry{Name: []byte(entry.Name), File: entry.File})
			}
			r, err := NewNativeFileRequester(p, mode, []byte(input.Drawer), []byte(input.Filename), NativeFileRequesterCallbacks{List: func([]byte) ([]NativeFileEntry, error) { return entries, nil }, Exists: func([]byte) (bool, error) { return false, nil }, Load: func([]byte) error { return nil }, Save: func([]byte) error { return nil }})
			if err != nil {
				t.Fatal(err)
			}
			r.Offset = input.Offset
			files := []string{}
			for _, name := range r.Files {
				files = append(files, string(name))
			}
			if !reflect.DeepEqual(files, fixture.Files) {
				t.Fatal("native directory order/filter/uppercase differs")
			}
			plan, err := r.Plan()
			if err != nil {
				t.Fatal(err)
			}
			expected, err := hex.DecodeString(fixture.BeforeTextHex)
			if err != nil || !bytes.Equal(plan.Text, expected) {
				t.Fatalf("native prepared page differs: got%q want%q", plan.Text, expected)
			}
			if err := r.Action(int(input.Action)); err != nil {
				t.Fatal(err)
			}
			if r.Offset != fixture.Offset || r.Selected != fixture.Selected || string(r.Filename) != fixture.Filename {
				t.Fatalf("native file action differs: got%+v nativeoffset%d selected%d filename%q", r, fixture.Offset, fixture.Selected, fixture.Filename)
			}
			if fixture.Stop == "submit" && string(r.pendingPath) != fixture.Path {
				t.Fatal("native submit path differs")
			}
			if fixture.Stop == "edit" && r.Editing == NativeFileEditNone {
				t.Fatal("native edit action lost asynchronous field request")
			}
			if input.Action == 36 && !r.Cancelled {
				t.Fatal("native cancel action did not close requester")
			}
		})
	}
}

func TestNativeFileRequesterHostOperationsAndDialogs(t *testing.T) {
	p, err := DecodeNativePresentation(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	writes, loads := 0, 0
	cb := NativeFileRequesterCallbacks{List: func([]byte) ([]NativeFileEntry, error) {
		return []NativeFileEntry{{Name: []byte("A.GAM"), File: true}}, nil
	}, Exists: func([]byte) (bool, error) { return true, nil }, Load: func(path []byte) error { loads++; return fmt.Errorf("host read failed: %s", path) }, Save: func(path []byte) error {
		writes++
		if string(path) != "DF0:A.GAM" {
			t.Fatal("save callback path differs")
		}
		return nil
	}}
	r, err := NewNativeFileRequester(p, NativeFileSave, []byte("DF0:"), []byte("A"), cb)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Action(34); err != nil || r.Modal != NativeFileOverwrite || writes != 0 {
		t.Fatal("native overwrite gate skipped")
	}
	if err := r.Action(4); err != nil || r.Modal != NativeFileBrowser || writes != 0 {
		t.Fatal("native overwrite cancellation wrote data")
	}
	_ = r.Action(34)
	if err := r.Action(2); err != nil || !r.Done || writes != 1 {
		t.Fatal("confirmed native overwrite did not call real save")
	}
	r, err = NewNativeFileRequester(p, NativeFileLoad, []byte("DF0:"), []byte("A.GAM"), cb)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Action(34); err != nil || r.Modal != NativeFileMessage || loads != 1 || r.LastError == nil {
		t.Fatal("native load failure was reported as success")
	}
	plan, err := r.Plan()
	if err != nil || !bytes.Contains(plan.Text, []byte("ERREUR FICHIER")) {
		t.Fatal("native error template missing")
	}
	if err := r.Action(2); err != nil || r.Modal != NativeFileBrowser || r.Done {
		t.Fatal("native error acknowledgement did not return to browser")
	}
	_ = r.Action(30)
	if err := r.FinishEdit([]byte("SAVE/")); err != nil || string(r.Drawer) != "SAVE/" || r.Offset != 0 {
		t.Fatal("drawer editing failed to refresh list")
	}
	_ = r.Action(32)
	if err := r.FinishEdit([]byte("CUSTOM")); err != nil || string(r.Filename) != "CUSTOM" {
		t.Fatal("filename editing lost entered bytes")
	}
	r.callbacks.Load = nil
	if err := r.Action(34); err == nil || r.Done {
		t.Fatal("missing host operation became fabricated success")
	}
}

func TestNativeFileRequesterTransfersAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/file_requester_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Transfers []struct {
			Input struct {
				Name                       string
				Save                       bool
				OldOpen, NewOpen, Transfer uint32
				Confirmation               uint16
			}
			Stop                    string
			Probes, Reads, NewOpens int
			Dialogs                 []struct{ Kind, TextHex string }
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Transfers) != 11 {
		t.Fatalf("native transfer fixture catalog incomplete: %v", err)
	}
	p, err := DecodeNativePresentation(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Transfers {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			probes, reads, saves := 0, 0, 0
			cb := NativeFileRequesterCallbacks{
				List: func([]byte) ([]NativeFileEntry, error) { return nil, nil },
				Exists: func([]byte) (bool, error) {
					probes++
					if int32(input.OldOpen) < 0 {
						return false, fmt.Errorf("native old-file probe failed")
					}
					return input.OldOpen > 0, nil
				},
				Load: func(path []byte) error {
					reads++
					if string(path) != "DF0:A.GAM" {
						t.Fatal("native load path differs")
					}
					if int32(input.OldOpen) <= 0 || input.Transfer != NativeGAMSize {
						return fmt.Errorf("native read failed")
					}
					return nil
				},
				Save: func(path []byte) error {
					saves++
					if string(path) != "DF0:A.GAM" {
						t.Fatal("native save path differs")
					}
					if int32(input.NewOpen) <= 0 || input.Transfer != NativeGAMSize {
						return fmt.Errorf("native write failed")
					}
					return nil
				},
			}
			mode := NativeFileLoad
			if input.Save {
				mode = NativeFileSave
			}
			r, err := NewNativeFileRequester(p, mode, []byte("DF0:"), []byte("A.GAM"), cb)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Action(34); err != nil {
				t.Fatal(err)
			}
			for _, dialog := range fixture.Dialogs {
				wantModal := NativeFileMessage
				if dialog.Kind == "overwrite" {
					wantModal = NativeFileOverwrite
				}
				if r.Modal != wantModal {
					t.Fatal("native transfer dialog order differs")
				}
				plan, err := r.Plan()
				want, decodeErr := hex.DecodeString(dialog.TextHex)
				if err != nil || decodeErr != nil || !bytes.Equal(plan.Text, want) {
					t.Fatal("native transfer dialog text differs")
				}
				action := 2
				if dialog.Kind == "overwrite" {
					action = int(input.Confirmation)
				}
				if err := r.Action(action); err != nil {
					t.Fatal(err)
				}
			}
			if r.Done != (fixture.Stop == "done") || r.Modal != NativeFileBrowser || probes != fixture.Probes || reads != fixture.Reads || saves != fixture.NewOpens {
				t.Fatalf("native transfer outcome/callbacks differ: done%v modal%d probe%d read%d save%d", r.Done, r.Modal, probes, reads, saves)
			}
		})
	}
}

func TestNativeFileRequesterGeometryAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/file_requester_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Pages []struct {
			Name, TextHex              string
			Column, Row, Width, Height int
			Clicks                     []struct{ X, Y, Action int }
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Pages) != 3 {
		t.Fatalf("native file page catalog incomplete: %v", err)
	}
	p, err := DecodeNativePresentation(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewNativeFileRequester(p, NativeFileLoad, []byte("DF0:"), []byte("A.GAM"), NativeFileRequesterCallbacks{List: func([]byte) ([]NativeFileEntry, error) {
		return []NativeFileEntry{{Name: []byte("A.GAM"), File: true}, {Name: []byte("B.GAM"), File: true}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range catalog.Pages {
		t.Run(page.Name, func(t *testing.T) {
			r.Modal = NativeFileBrowser
			if page.Name == "message" {
				r.Modal = NativeFileMessage
			}
			if page.Name == "overwrite" {
				r.Modal = NativeFileOverwrite
			}
			plan, err := r.Plan()
			want, decodeErr := hex.DecodeString(page.TextHex)
			if err != nil || decodeErr != nil || !bytes.Equal(plan.Text, want) || plan.Column != page.Column || plan.Row != page.Row || plan.Width != page.Width || plan.Height != page.Height {
				t.Fatal("native file page geometry/text differs")
			}
			if len(page.Clicks) != 1000 {
				t.Fatal("native file click fixture incomplete")
			}
			for _, click := range page.Clicks {
				if got := p.Requesters.Click(plan, click.X, click.Y); got != click.Action {
					t.Fatalf("click%d,%d: action%d native%d", click.X, click.Y, got, click.Action)
				}
			}
		})
	}
}
