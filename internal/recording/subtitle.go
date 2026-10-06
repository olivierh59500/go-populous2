package recording

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	SubtitleWidth  = 960
	SubtitleHeight = 680
	gameWidth      = 320
	gameHeight     = 200
	gameScale      = 3
	captionTop     = gameHeight * gameScale
	captionMargin  = 24
)

// SubtitleCanvas preserves every pixel of the 320x200 game at a 3x integer
// scale. English commentary occupies its own dark band below the game rather
// than covering controls, followers or the minimap. It needs no system fonts
// and works with FFmpeg builds that do not include subtitle filters.
// Frame calls must be serialized; the returned buffer belongs to the canvas.
type SubtitleCanvas struct {
	image       *image.RGBA
	face        font.Face
	caption     string
	captionSet  bool
	frames      int64
	transitions []captionTransition
}

type captionTransition struct {
	frame int64
	text  string
}

func NewSubtitleCanvas() (*SubtitleCanvas, error) {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("parsing bundled caption font: %w", err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 26, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("creating caption font: %w", err)
	}
	return &SubtitleCanvas{image: image.NewRGBA(image.Rect(0, 0, SubtitleWidth, SubtitleHeight)), face: face}, nil
}

// Frame composes one game image and a caption. Text wraps by its measured
// width, with room for two lines. Excess text is rejected instead of clipped.
// An unchanged caption reuses the existing band and avoids font work per tick.
func (c *SubtitleCanvas) Frame(rgba []byte, caption string) ([]byte, error) {
	if len(rgba) != gameWidth*gameHeight*4 {
		return nil, errors.New("subtitle canvas requires a packed 320x200 RGBA frame")
	}
	caption = strings.TrimSpace(caption)
	if !c.captionSet || caption != c.caption {
		lines, err := wrapCaption(c.face, caption, SubtitleWidth-2*captionMargin)
		if err != nil {
			return nil, err
		}
		if len(lines) > 2 {
			return nil, errors.New("subtitle caption exceeds the two-line band")
		}
		band := image.Rect(0, captionTop, SubtitleWidth, SubtitleHeight)
		draw.Draw(c.image, band, image.NewUniform(color.RGBA{12, 17, 26, 255}), image.Point{}, draw.Src)
		if len(lines) != 0 {
			const lineSpacing = 32
			metrics := c.face.Metrics()
			height := metrics.Ascent.Ceil() + metrics.Descent.Ceil() + (len(lines)-1)*lineSpacing
			baseline := captionTop + (SubtitleHeight-captionTop-height)/2 + metrics.Ascent.Ceil()
			d := font.Drawer{Dst: c.image, Src: image.NewUniform(color.RGBA{245, 248, 252, 255}), Face: c.face}
			for _, line := range lines {
				d.Dot = fixed.P((SubtitleWidth-font.MeasureString(c.face, line).Ceil())/2, baseline)
				d.DrawString(line)
				baseline += lineSpacing
			}
		}
		c.caption, c.captionSet = caption, true
		c.transitions = append(c.transitions, captionTransition{frame: c.frames, text: caption})
	}
	for y := range gameHeight {
		source := rgba[y*gameWidth*4 : (y+1)*gameWidth*4]
		target := c.image.Pix[(y*gameScale)*c.image.Stride : (y*gameScale+1)*c.image.Stride]
		for x := range gameWidth {
			pixel := source[x*4 : (x+1)*4]
			for repeat := range gameScale {
				copy(target[(x*gameScale+repeat)*4:], pixel)
			}
		}
		for repeat := 1; repeat < gameScale; repeat++ {
			copy(c.image.Pix[(y*gameScale+repeat)*c.image.Stride:], target)
		}
	}
	c.frames++
	return c.image.Pix, nil
}

// SRT returns an optional editable subtitle file matching the frames composed
// so far. The MP4 already contains visible captions; no subtitle toggle is
// needed by a player. Empty captions create gaps in the SRT track.
func (c *SubtitleCanvas) SRT(fps int) ([]byte, error) {
	if fps <= 0 || fps > 1000 {
		return nil, errors.New("subtitle frame rate must be between 1 and 1000")
	}
	var out strings.Builder
	index := 1
	for i, transition := range c.transitions {
		end := c.frames
		if i+1 < len(c.transitions) {
			end = c.transitions[i+1].frame
		}
		if transition.text == "" || end <= transition.frame {
			continue
		}
		fmt.Fprintf(&out, "%d\n%s --> %s\n%s\n\n", index, subtitleTime(transition.frame, fps), subtitleTime(end, fps), transition.text)
		index++
	}
	return []byte(out.String()), nil
}

func subtitleTime(frame int64, fps int) string {
	ms := frame * 1000 / int64(fps)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

func wrapCaption(face font.Face, text string, width int) ([]string, error) {
	if text == "" {
		return nil, nil
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if font.MeasureString(face, word).Ceil() > width {
				return nil, errors.New("subtitle contains a word wider than the caption band")
			}
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if font.MeasureString(face, candidate).Ceil() > width {
				lines = append(lines, line)
				line = word
			} else {
				line = candidate
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}
