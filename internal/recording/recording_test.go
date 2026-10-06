package recording

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func requireFFmpeg(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is not installed")
	}
	return path
}

func sampleFrame() []byte {
	frame := make([]byte, 8*4*4)
	for i := 0; i < len(frame); i += 4 {
		frame[i], frame[i+1], frame[i+2], frame[i+3] = 220, 60, 40, 255
	}
	return frame
}

func sampleAudio(tick int) []byte {
	pcm := make([]byte, 44100/50*4)
	for i := 0; i < len(pcm); i += 4 {
		sample := int16(6000 * math.Sin(float64(tick*882+i/4)*2*math.Pi*440/44100))
		binary.LittleEndian.PutUint16(pcm[i:], uint16(sample))
		binary.LittleEndian.PutUint16(pcm[i+2:], uint16(sample))
	}
	return pcm
}

func TestMP4PreservesPALFramesAndApplicationPCM(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	path := filepath.Join(t.TempDir(), "recordings", "game.mp4")
	r, err := New(path, 8, 4, 50, 44100)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Abort()
	for tick := range 50 {
		if err := r.WriteFrame(sampleFrame(), sampleAudio(tick)); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := os.Stat(r.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary files remain: %v", err)
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Log("FFmpeg export passed; ffprobe is unavailable for stream inspection")
		return
	}
	data, err := exec.Command(probe, "-v", "error", "-show_streams", "-show_format", "-of", "json", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var info struct {
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			PixFmt     string `json:"pix_fmt"`
			Frames     string `json:"nb_frames"`
			FrameRate  string `json:"r_frame_rate"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
		}
		Format struct {
			Duration string `json:"duration"`
		}
	}
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	if len(info.Streams) != 2 {
		t.Fatalf("MP4 has %d streams, want video and audio", len(info.Streams))
	}
	v, a := info.Streams[0], info.Streams[1]
	if v.CodecType != "video" || v.CodecName != "h264" || v.Width != 960 || v.Height != 600 || v.PixFmt != "yuv420p" || v.Frames != "50" || v.FrameRate != "50/1" {
		t.Fatalf("unexpected video stream: %+v", v)
	}
	if a.CodecType != "audio" || a.CodecName != "aac" || a.SampleRate != "44100" || a.Channels != 2 {
		t.Fatalf("unexpected audio stream: %+v", a)
	}
	duration, _ := strconv.ParseFloat(info.Format.Duration, 64)
	if math.Abs(duration-1) > 0.025 {
		t.Fatalf("movie duration %g, want 1 second", duration)
	}
	pcm, err := exec.Command(ffmpeg, "-v", "error", "-i", path, "-map", "0:a:0", "-f", "s16le", "pipe:1").Output()
	if err != nil {
		t.Fatal(err)
	}
	var energy float64
	for i := 0; i+1 < len(pcm); i += 2 {
		value := float64(int16(binary.LittleEndian.Uint16(pcm[i:])))
		energy += value * value
	}
	if len(pcm) < 44100*4 || math.Sqrt(energy/float64(len(pcm)/2)) < 3000 {
		t.Fatal("MP4 lost the supplied application soundtrack")
	}
	image, err := exec.Command(ffmpeg, "-v", "error", "-i", path, "-map", "0:v:0", "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgba", "pipe:1").Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(image) != 960*600*4 {
		t.Fatalf("decoded image has %d RGBA bytes", len(image))
	}
	center := (300*960 + 480) * 4
	if image[0] > 8 || image[1] > 8 || image[2] > 8 || image[center] < 190 || image[center+1] > 90 {
		t.Fatal("scene or aspect-preserving black padding was lost")
	}
}

func TestNeverOverwritesAnOutputCreatedWhileRecording(t *testing.T) {
	requireFFmpeg(t)
	path := filepath.Join(t.TempDir(), "keep.mp4")
	r, err := New(path, 8, 4, 50, 44100)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Abort()
	if err := r.WriteFrame(sampleFrame(), sampleAudio(0)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("existing movie"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); !errors.Is(err, os.ErrExist) {
		t.Fatalf("Close with new output: %v, want exists error", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "existing movie" {
		t.Fatalf("existing movie changed: %q %v", data, err)
	}
	if _, err := New(path, 8, 4, 50, 44100); !errors.Is(err, os.ErrExist) {
		t.Fatalf("New with existing output: %v", err)
	}
	if _, err := os.Stat(r.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed publication left temporary files: %v", err)
	}
}

func TestMismatchedFrameAndAudioDoNotPublish(t *testing.T) {
	requireFFmpeg(t)
	for _, audio := range []bool{false, true} {
		name := "frame"
		if audio {
			name = "audio"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.mp4")
			r, err := New(path, 8, 4, 50, 44100)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Abort()
			rgba, pcm := sampleFrame(), sampleAudio(0)
			if audio {
				pcm = pcm[:len(pcm)-4]
			} else {
				rgba = rgba[:len(rgba)-4]
			}
			if err := r.WriteFrame(rgba, pcm); err == nil {
				t.Fatal("mismatched frame accepted")
			}
			if err := r.Close(); err == nil {
				t.Fatal("Close discarded frame validation error")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid recording published: %v", err)
			}
			if _, err := os.Stat(r.dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid recording left temporary files: %v", err)
			}
		})
	}
}

func TestAbortRemovesOnlyItsTemporaryFiles(t *testing.T) {
	requireFFmpeg(t)
	path := filepath.Join(t.TempDir(), "abort.mp4")
	r, err := New(path, 8, 4, 50, 44100)
	if err != nil {
		t.Fatal(err)
	}
	r.Abort()
	r.Abort()
	if err := r.Close(); err == nil {
		t.Fatal("Close after Abort should report cancellation")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("aborted output exists: %v", err)
	}
	if _, err := os.Stat(r.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("aborted temporary directory exists: %v", err)
	}
}

func TestEncoderDiagnosticIsBounded(t *testing.T) {
	b := &tailBuffer{}
	_, _ = b.Write([]byte(strings.Repeat("x", 40000)))
	_, _ = b.Write([]byte("last error"))
	if len(b.data) != 32*1024 || !strings.HasSuffix(b.message(), "last error") {
		t.Fatal("FFmpeg diagnostic tail grew without bound or lost the error")
	}
}
