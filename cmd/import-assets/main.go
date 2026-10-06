// Command import-assets prepares private game data from the user's own ADFs.
package main

import (
	"flag"
	"fmt"
	"go-populous2/internal/assetimport"
	"os"
)

type diskPaths []string

func (p *diskPaths) String() string         { return fmt.Sprint([]string(*p)) }
func (p *diskPaths) Set(value string) error { *p = append(*p, value); return nil }

func main() {
	var disks diskPaths
	flag.Var(&disks, "adf", "original ADF disk image; repeat for multiple disks")
	executable := flag.String("executable", "", "user-supplied supported French populous.ii, if absent from the ADFs")
	output := flag.String("output", "assets/amiga", "local output directory for the game data")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "use -adf for each original disk image")
		os.Exit(1)
	}
	count, err := assetimport.Import(assetimport.Config{ADFs: disks, Executable: *executable, Output: *output})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Validated %d game files in %s. They are local build inputs, not repository assets.\n", count, *output)
}
