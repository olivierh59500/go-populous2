package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"

	"go-populous2/assets"
	"go-populous2/internal/amiga"
	"go-populous2/internal/populous2"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("data", "", "external installation directory (default: embedded Amiga data)")
	output := flag.String("decode", "", "write decompressed resources into a new directory")
	jsonOutput := flag.Bool("json", false, "write resource table and presence as JSON")
	images := flag.String("images", "", "write decoded tile/sprite atlases and interface PNGs to a new directory")
	flag.Parse()
	files, err := fs.Sub(assets.Files, "amiga")
	if err != nil {
		return err
	}
	if *dir != "" {
		files = os.DirFS(*dir)
	}
	data, err := fs.ReadFile(files, "populous.ii")
	if err != nil {
		return err
	}
	exe, err := amiga.ParseExecutable(data)
	if err != nil {
		return err
	}
	resources, err := populous2.ResourceTable(exe)
	if err != nil {
		return err
	}
	status, err := populous2.Inventory(files, resources)
	if err != nil {
		return err
	}
	if missing := populous2.MissingResources(status); len(missing) > 0 {
		return fmt.Errorf("missing Populous II resources: %v", missing)
	}
	if *output != "" {
		if err := os.Mkdir(*output, 0755); err != nil {
			return fmt.Errorf("decode output must be a new directory: %w", err)
		}
	}
	for _, resource := range resources {
		decoded, err := populous2.LoadResource(files, resource)
		if err != nil {
			return err
		}
		if *output != "" {
			if err := os.WriteFile(filepath.Join(*output, resource.Name), decoded, 0644); err != nil {
				return err
			}
		}
		if !*jsonOutput {
			fmt.Printf("%2d %-16s packed=%t decoded=%7d planar=0x%08x table=0x%x SHA256=%s\n", resource.Index, resource.Name, resource.Packed, len(decoded), resource.PlanarTable, resource.TableOffset, amiga.Digest(decoded))
		}
	}
	if *images != "" {
		bundle, err := populous2.LoadFS(files)
		if err != nil {
			return err
		}
		if err := os.Mkdir(*images, 0755); err != nil {
			return err
		}
		for i, tiles := range bundle.Tiles {
			if err := writePNG(filepath.Join(*images, fmt.Sprintf("tiles-%d.png", i)), populous2.TileAtlas(tiles, 16)); err != nil {
				return err
			}
			sprites := bundle.Sprites[i]
			atlas := image.NewRGBA(image.Rect(0, 0, 32*24, ((len(sprites)+23)/24)*64))
			for n, sprite := range sprites {
				if sprite.Image != nil {
					x, y := n%24*32, n/24*64+64-sprite.AnchorY
					draw.Draw(atlas, image.Rect(x, y, x+sprite.Image.Bounds().Dx(), y+sprite.Image.Bounds().Dy()), sprite.Image, image.Point{}, draw.Src)
				}
			}
			if err := writePNG(filepath.Join(*images, fmt.Sprintf("sprites-%d.png", i)), atlas); err != nil {
				return err
			}
		}
		if err := writePNG(filepath.Join(*images, "interface.png"), bundle.Background); err != nil {
			return err
		}
		font, err := populous2.DecodeNativeMenuFont(exe)
		if err != nil {
			return err
		}
		if err := writePNG(filepath.Join(*images, "menu-font.png"), font.Atlas(bundle.Landscapes[0].Palettes[0])); err != nil {
			return err
		}
		rules, err := populous2.DecodeNativeRequesterRules(exe)
		if err != nil {
			return err
		}
		startup, err := rules.Startup(exe)
		if err != nil {
			return err
		}
		palette, err := populous2.NativeStartupPalette(exe)
		if err != nil {
			return err
		}
		startupImage, err := startup.Image(font, palette)
		if err != nil {
			return err
		}
		if err := writePNG(filepath.Join(*images, "requester-startup.png"), startupImage); err != nil {
			return err
		}
		definition, err := populous2.NativeRequesterTemplate(exe, 0x8c42)
		if err != nil {
			return err
		}
		options, err := rules.Compile(definition, [][]byte{[]byte("LE BLEU"), nil})
		if err != nil {
			return err
		}
		optionsImage, err := options.Image(font, bundle.Landscapes[0].Palettes[0])
		if err != nil {
			return err
		}
		if err := writePNG(filepath.Join(*images, "requester-options.png"), optionsImage); err != nil {
			return err
		}
	}
	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}
	return nil
}

func writePNG(name string, img image.Image) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	encodeErr := png.Encode(f, img)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}
