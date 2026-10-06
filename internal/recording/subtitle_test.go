package recording

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func nativeCaptionFrame() []byte {
	rgba := make([]byte, gameWidth*gameHeight*4)
	for y := range gameHeight {
		for x := range gameWidth {
			i := (y*gameWidth + x) * 4
			rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = byte(x), byte(y), byte(x^y), 255
		}
	}
	return rgba
}

func TestSubtitleCanvasPreservesEntireGameAndSeparateCaptionBand(t *testing.T) {
	c, err := NewSubtitleCanvas()
	if err != nil {
		t.Fatal(err)
	}
	native := nativeCaptionFrame()
	frame, err := c.Frame(native, "Build a stable plateau so towns can grow.\nMore followers generate more mana.")
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != SubtitleWidth*SubtitleHeight*4 {
		t.Fatalf("subtitle frame has %d bytes", len(frame))
	}
	for y := range captionTop {
		for x := range SubtitleWidth {
			got := frame[(y*SubtitleWidth+x)*4 : (y*SubtitleWidth+x+1)*4]
			want := native[((y/gameScale)*gameWidth+x/gameScale)*4 : ((y/gameScale)*gameWidth+x/gameScale+1)*4]
			if !bytes.Equal(got, want) {
				t.Fatalf("game pixel changed at %d,%d: %v, want %v", x, y, got, want)
			}
		}
	}
	ink := 0
	for y := captionTop; y < SubtitleHeight; y++ {
		for x := range SubtitleWidth {
			i := (y*SubtitleWidth + x) * 4
			if frame[i] > 200 && frame[i+1] > 200 && frame[i+2] > 200 {
				ink++
				if x < captionMargin || x >= SubtitleWidth-captionMargin || y < captionTop+5 || y >= SubtitleHeight-5 {
					t.Fatalf("caption is clipped at %d,%d", x, y)
				}
			}
		}
	}
	if ink < 100 {
		t.Fatal("caption band contains no readable text")
	}
	band := append([]byte(nil), frame[captionTop*SubtitleWidth*4:]...)
	native[0]++
	frame, err = c.Frame(native, c.caption)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(band, frame[captionTop*SubtitleWidth*4:]) || len(c.transitions) != 1 {
		t.Fatal("unchanged caption was not retained")
	}
	frame, err = c.Frame(native, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := captionTop * SubtitleWidth * 4; i < len(frame); i += 4 {
		if !bytes.Equal(frame[i:i+4], []byte{12, 17, 26, 255}) {
			t.Fatal("previous caption remained after clearing the band")
		}
	}
}

func TestSubtitleCanvasRejectsClippedTextAndInvalidFrame(t *testing.T) {
	c, err := NewSubtitleCanvas()
	if err != nil {
		t.Fatal(err)
	}
	frame := nativeCaptionFrame()
	if _, err := c.Frame(frame[:len(frame)-1], "bad frame"); err == nil {
		t.Fatal("invalid source size accepted")
	}
	for _, text := range []string{strings.Repeat("W", 100), strings.Repeat("long caption text ", 20), "one\ntwo\nthree"} {
		if _, err := c.Frame(frame, text); err == nil {
			t.Errorf("clipped caption accepted: %q", text)
		}
	}
	if c.frames != 0 || len(c.transitions) != 0 {
		t.Fatal("rejected captions changed the subtitle timeline")
	}
}

func TestSubtitleSRTMatchesFrameTimelineAndEmptyGaps(t *testing.T) {
	c, err := NewSubtitleCanvas()
	if err != nil {
		t.Fatal(err)
	}
	frame := nativeCaptionFrame()
	for i := range 100 {
		caption := "Opening the conquest menu."
		if i >= 50 {
			caption = ""
		}
		if i >= 75 {
			caption = "The first settlement is ready."
		}
		if _, err := c.Frame(frame, caption); err != nil {
			t.Fatal(err)
		}
	}
	srt, err := c.SRT(50)
	if err != nil {
		t.Fatal(err)
	}
	want := "1\n00:00:00,000 --> 00:00:01,000\nOpening the conquest menu.\n\n2\n00:00:01,500 --> 00:00:02,000\nThe first settlement is ready.\n\n"
	if string(srt) != want {
		t.Fatalf("unexpected SRT:\n%s", srt)
	}
	if _, err := c.SRT(0); err == nil {
		t.Fatal("zero frame rate accepted")
	}
}

func TestSizedRecorderRetainsCaptionMovieDimensions(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	path := filepath.Join(t.TempDir(), "caption.mp4")
	c, err := NewSubtitleCanvas()
	if err != nil {
		t.Fatal(err)
	}
	frame, err := c.Frame(nativeCaptionFrame(), "Explore the menu, then begin a conquest.")
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewSized(path, SubtitleWidth, SubtitleHeight, SubtitleWidth, SubtitleHeight, 50, 44100)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Abort()
	if err := r.WriteFrame(frame, sampleAudio(0)); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	decoded, err := exec.Command(ffmpeg, "-v", "error", "-i", path, "-map", "0:v:0", "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgba", "pipe:1").Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(frame) {
		t.Fatalf("encoded caption movie has %d RGBA bytes, want %d", len(decoded), len(frame))
	}
	// Check the border at the end of the complete game, plus visible caption
	// pixels underneath it. YUV conversion may slightly alter their colors.
	if decoded[(599*SubtitleWidth+959)*4+1] < 170 {
		t.Fatal("lower-right game image was rescaled or cropped")
	}
	ink := 0
	for i := captionTop * SubtitleWidth * 4; i < len(decoded); i += 4 {
		if decoded[i] > 180 && decoded[i+1] > 180 && decoded[i+2] > 180 {
			ink++
		}
	}
	if ink < 100 {
		t.Fatal("encoded movie lost the caption band")
	}
}

func TestSizedRecorderRejectsInvalidOutputBeforeCreatingFiles(t *testing.T) {
	for _, size := range [][2]int{{0, 2}, {2, 0}, {3, 2}, {2, 3}, {16386, 2}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			if _, err := NewSized(filepath.Join(t.TempDir(), "bad.mp4"), 8, 4, size[0], size[1], 50, 44100); err == nil {
				t.Fatal("invalid output dimensions accepted")
			}
		})
	}
}
