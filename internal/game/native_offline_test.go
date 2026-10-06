package game

import (
	"bytes"
	"go-populous2/internal/populous2"
	"io"
	"testing"
)

func TestNativeOfflineRetainsPALFramesAndSoundtrack(t *testing.T) {
	bundle, err := populous2.Load()
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewNativeOffline(bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	g.AutoStart = true
	pcm := make([]byte, 3528)
	silence := make([]byte, len(pcm))
	audible := false
	for tick := 0; tick < 500; tick++ {
		rgba, err := g.StepNative(NativeInput{X: 190, Y: 120})
		if err != nil {
			t.Fatal(tick, err)
		}
		if len(rgba) != 320*200*4 {
			t.Fatal("incomplete native framebuffer", len(rgba))
		}
		if _, err := io.ReadFull(g.Stream, pcm); err != nil {
			t.Fatal(tick, err)
		}
		if !bytes.Equal(pcm, silence) {
			audible = true
		}
	}
	interrupts, err := g.Host.Memory.BSS.Read32(0x16)
	if err != nil {
		t.Fatal(err)
	}
	if interrupts != 500 || g.player != nil || g.image != nil || !audible || g.Frame == nil {
		t.Fatal("offline capture changed native timing, audio or startup", interrupts, audible, g.Frame != nil)
	}
	if g.Frame.RenderChildren.Protection.Started {
		t.Fatal("offline capture entered the manual challenge")
	}
}
