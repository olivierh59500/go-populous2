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
	showcase := flag.Bool("showcase", false, "tour the menus and begin a conquest with visible English subtitles")
	flag.Parse()
	if *seconds < 10 || *seconds > 1800 {
		log.Fatal("seconds must be between 10 and 1800")
	}
	if *showcase && *seconds > 360 {
		log.Fatal("a showcase cannot exceed six minutes")
	}
	if err := run(*output, *seconds, *checkpoints, *inspect, *trace, *showcase); err != nil {
		log.Fatal(err)
	}
}

func run(output string, seconds int, checkpoints string, inspect bool, trace string, showcase bool) error {
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
	next, status := pilot.Next, func() string { return pilot.Status(g) }
	var tour *game.NativeShowcasePilot
	var subtitles *recording.SubtitleCanvas
	var cues []subtitleCue
	if showcase {
		tour = game.NewNativeShowcasePilot()
		next, status = tour.Next, func() string { return tour.Status(g) }
		subtitles, err = recording.NewSubtitleCanvas()
		if err != nil {
			return err
		}
	}
	var subtitleFile *os.File
	subtitlesComplete := false
	if showcase && !inspect {
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			return err
		}
		subtitleFile, err = os.OpenFile(output+".en.srt", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer func() {
			subtitleFile.Close()
			if !subtitlesComplete {
				os.Remove(output + ".en.srt")
			}
		}()
	}
	var traceError error
	if trace != "" {
		file, err := os.OpenFile(trace, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		encoder := json.NewEncoder(file)
		observe := func(edit game.NativePresentationTerrainEdit) {
			if traceError == nil {
				traceError = encoder.Encode(edit)
			}
		}
		pilot.TerrainEdit = observe
		if tour != nil {
			tour.TerrainEdit = observe
		}
	}
	var recorder *recording.Recorder
	if !inspect {
		if showcase {
			recorder, err = recording.NewSized(output, 960, 680, 960, 680, 50, 44100)
		} else {
			recorder, err = recording.New(output, 320, 200, 50, 44100)
		}
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
		input, err := next(g)
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
		if showcase {
			if len(cues) == 0 || cues[len(cues)-1].text != tour.Caption {
				if len(cues) > 0 {
					cues[len(cues)-1].end = tick
				}
				cues = append(cues, subtitleCue{start: tick, text: tour.Caption})
			}
		}
		if _, err := io.ReadFull(g.Stream, pcm); err != nil {
			return fmt.Errorf("audio update %d: %w", tick, err)
		}
		video, width, height := rgba, 320, 200
		if showcase {
			video, err = subtitles.Frame(rgba, tour.Caption)
			if err != nil {
				return fmt.Errorf("caption update %d: %w", tick, err)
			}
			width, height = 960, 680
		}
		if recorder != nil {
			if err := recorder.WriteFrame(video, pcm); err != nil {
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
		interval := 1500
		if showcase {
			interval = 500
		}
		if tick%interval == interval-1 {
			fmt.Printf("%ds: %s\n", (tick+1)/50, status())
			if checkpoints != "" {
				path := filepath.Join(checkpoints, fmt.Sprintf("game-%04ds.png", (tick+1)/50))
				f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
				if err != nil {
					return err
				}
				err = png.Encode(f, &image.RGBA{Pix: video, Stride: width * 4, Rect: image.Rect(0, 0, width, height)})
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
	if showcase && len(cues) > 0 {
		cues[len(cues)-1].end = frames
		if !inspect {
			data, err := subtitles.SRT(50)
			if err != nil {
				return err
			}
			_, writeErr := subtitleFile.Write(data)
			closeErr := subtitleFile.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
			subtitlesComplete = true
		}
		for _, cue := range cues {
			if cue.text != "" {
				fmt.Printf("caption %.2f–%.2fs: %s\n", float64(cue.start)/50, float64(cue.end)/50, cue.text)
			}
		}
	}
	summary := map[string]any{"seconds": float64(frames) / 50, "fps": 50, "status": status(), "inputOnly": true, "output": output}
	b, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

type subtitleCue struct {
	start, end int
	text       string
}
