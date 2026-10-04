// amiga-inspect inventories an ADF, an extracted installation or one executable.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go-populous2/internal/amiga"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("amiga-inspect", flag.ContinueOnError)
	input := flags.String("input", "previous/Populous2_PatchFR", "ADF, extracted installation directory or Hunk executable")
	jsonOutput := flags.Bool("json", false, "write machine-readable inventory")
	includeStrings := flags.Bool("strings", false, "include printable strings and file offsets in JSON")
	dumpDir := flags.String("dump-hunks", "", "write code/data payloads to a new directory for 68000 analysis")
	extractDir := flags.String("extract", "", "extract ADF files into a new directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional argument %q; use -input", flags.Arg(0))
	}
	report, err := inspect(*input, *includeStrings)
	if err != nil {
		return err
	}
	if *dumpDir != "" {
		if err := dumpHunks(*dumpDir, report); err != nil {
			return err
		}
	}
	if *extractDir != "" {
		if err := extractDisk(*input, *extractDir); err != nil {
			return err
		}
	}
	if *jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	fmt.Fprintf(out, "%s: %s, identity %s\n", report.Input, report.Format, report.Identity)
	if report.Volume != "" {
		fmt.Fprintf(out, "Volume: %s; image SHA-256: %s\n", report.Volume, report.SHA256)
	}
	for _, evidence := range report.Evidence {
		fmt.Fprintln(out, evidence)
	}
	for _, file := range report.Files {
		match := ""
		if file.Populous1 {
			match = " [Populous 1 identical]"
		}
		fmt.Fprintf(out, "%7d  %-24s %s%s\n", file.Size, file.Path, file.SHA256, match)
		if file.HunkError != "" {
			fmt.Fprintf(out, "  Hunk error: %s\n", file.HunkError)
		}
		if file.Executable != nil {
			for _, h := range file.Executable.Hunks {
				fmt.Fprintf(out, "  hunk %d %-4s %-8s allocated=%d payload=%d file=0x%x relocations=%d\n", h.Index, h.Kind, h.Memory, h.AllocatedBytes, h.PayloadBytes, h.FileOffset, len(h.Relocations))
			}
		}
	}
	return nil
}

func extractDisk(input, dir string) (err error) {
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	disk, err := amiga.ParseDisk(data)
	if err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return fmt.Errorf("extraction needs a new directory: %w", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir) // Only the directory created by this call.
		}
	}()
	for _, entry := range disk.Entries() {
		p := filepath.Join(dir, filepath.FromSlash(entry.Path))
		if entry.Directory {
			if err := os.MkdirAll(p, 0755); err != nil {
				return err
			}
			continue
		}
		contents, err := disk.ReadFile(entry.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(p, contents, 0644); err != nil {
			return err
		}
	}
	return nil
}

func inspect(input string, includeStrings bool) (*amiga.Report, error) {
	info, err := os.Stat(input)
	if err != nil {
		return nil, err
	}
	r := &amiga.Report{Input: input}
	if info.IsDir() {
		r.Format = "directory"
		files := os.DirFS(input)
		err = fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("unsupported nonregular file %q", name)
			}
			data, err := fs.ReadFile(files, name)
			if err != nil {
				return err
			}
			r.Files = append(r.Files, amiga.InspectFile(name, data, includeStrings))
			return nil
		})
	} else {
		data, readErr := os.ReadFile(input)
		if readErr != nil {
			return nil, readErr
		}
		if strings.EqualFold(filepath.Ext(input), ".adf") || len(data) >= 3 && string(data[:3]) == "DOS" {
			var disk *amiga.Disk
			disk, err = amiga.ParseDisk(data)
			if err == nil {
				r.Format, r.Volume, r.SHA256 = disk.Format, disk.Name, amiga.Digest(data)
				for _, entry := range disk.Entries() {
					if entry.Directory {
						continue
					}
					var contents []byte
					contents, err = disk.ReadFile(entry.Path)
					if err != nil {
						break
					}
					r.Files = append(r.Files, amiga.InspectFile(entry.Path, contents, includeStrings))
				}
			}
		} else {
			r.Format = "file"
			r.Files = append(r.Files, amiga.InspectFile(filepath.Base(input), data, includeStrings))
		}
	}
	if err != nil {
		return nil, err
	}
	r.Identify()
	return r, nil
}

func dumpHunks(dir string, report *amiga.Report) error {
	// Require a new directory so a later run cannot overwrite reviewed dumps.
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	for _, file := range report.Files {
		if file.Executable == nil {
			continue
		}
		p := filepath.Join(dir, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(p, 0755); err != nil {
			return err
		}
		for _, hunk := range file.Executable.Hunks {
			if hunk.Kind == "bss" {
				continue
			}
			name := fmt.Sprintf("hunk-%02d-%s.bin", hunk.Index, hunk.Kind)
			if err := os.WriteFile(filepath.Join(p, name), hunk.Data, 0644); err != nil {
				return err
			}
		}
		metadata, err := json.MarshalIndent(file.Executable, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(p, "hunks.json"), append(metadata, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}
