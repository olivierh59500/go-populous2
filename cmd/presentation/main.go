// Command presentation records an input-driven game with its native soundtrack.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"

	"go-populous2/internal/game"
	"go-populous2/internal/populous2"
	"go-populous2/internal/recording"
)

func main() {
	output := flag.String("output", "recordings/populous2-gameplay.mp4", "new MP4 output path")
	seconds := flag.Int("seconds", 240, "presentation length at the normal PAL cadence")
	checkpoints := flag.String("checkpoints", "", "optional directory for one PNG checkpoint every 30 seconds")
	inspect := flag.Bool("inspect", false, "run the identical input sequence without encoding a video")
	trace := flag.String("terrain-trace", "", "optional new JSONL file recording terrain mouse actions")
	flag.Parse()
	if *seconds < 10 || *seconds > 1800 {
		log.Fatal("seconds must be between 10 and 1800")
	}
	if err := run(*output, *seconds, *checkpoints, *inspect, *trace); err != nil {
		log.Fatal(err)
	}
}

func run(output string, seconds int, checkpoints string, inspect bool, trace string) error {
	bundle, err := populous2.Load()
	if err != nil {
		return err
	}
	g, err := game.NewNativeOffline(bundle)
	if err != nil {
		return err
	}
	defer g.Close()
	pilot := game.NewNativePresentationPilot()
	var traceError error
	if trace != "" {
		file, err := os.OpenFile(trace, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		encoder := json.NewEncoder(file)
		pilot.TerrainEdit = func(edit game.NativePresentationTerrainEdit) {
			if traceError == nil {
				traceError = encoder.Encode(edit)
			}
		}
	}
	var recorder *recording.Recorder
	if !inspect {
		recorder, err = recording.New(output, 320, 200, 50, 44100)
		if err != nil {
			return err
		}
		defer recorder.Abort()
	}
	if checkpoints != "" {
		if err := os.MkdirAll(checkpoints, 0755); err != nil {
			return err
		}
	}
	pcm := make([]byte, 44100/50*4)
	frames, resultTick := 0, -1
	for tick := 0; tick < seconds*50; tick++ {
		input, err := pilot.Next(g)
		if err != nil {
			return fmt.Errorf("pilot update %d: %w", tick, err)
		}
		if traceError != nil {
			return traceError
		}
		rgba, err := g.StepNative(input)
		if err != nil {
			return fmt.Errorf("game update %d: %w", tick, err)
		}
		if _, err := io.ReadFull(g.Stream, pcm); err != nil {
			return fmt.Errorf("audio update %d: %w", tick, err)
		}
		if recorder != nil {
			if err := recorder.WriteFrame(rgba, pcm); err != nil {
				return err
			}
		}
		frames++
		if g.Result.Result != nil {
			if resultTick < 0 {
				resultTick = tick
			}
			// Retain the real result animation and statistics for ten seconds,
			// then finish instead of leaving a long inactive tail in the clip.
			if tick-resultTick >= 500 {
				break
			}
		}
		if tick%1500 == 1499 {
			fmt.Printf("%ds: %s\n", (tick+1)/50, pilot.Status(g))
			if checkpoints != "" {
				path := filepath.Join(checkpoints, fmt.Sprintf("game-%04ds.png", (tick+1)/50))
				f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
				if err != nil {
					return err
				}
				err = png.Encode(f, &image.RGBA{Pix: rgba, Stride: 320 * 4, Rect: image.Rect(0, 0, 320, 200)})
				closeErr := f.Close()
				if err != nil {
					return err
				}
				if closeErr != nil {
					return closeErr
				}
			}
		}
	}
	if recorder != nil {
		if err := recorder.Close(); err != nil {
			return err
		}
	}
	summary := map[string]any{"seconds": float64(frames) / 50, "fps": 50, "status": pilot.Status(g), "inputOnly": true, "output": output}
	b, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
