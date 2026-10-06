// Package recording exports the application's framebuffer and actual stereo
// PCM to MP4 without capturing the desktop, microphone or system audio mixer.
package recording

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const outputWidth, outputHeight = 960, 600

// Recorder consumes one packed RGBA image and one matching stereo PCM block
// per tick. Calls must be serialized by the owner of the application runtime.
// Frames are streamed to FFmpeg and audio is spooled to a temporary file, so
// memory use does not grow with the recording duration.
type Recorder struct {
	path, dir, ffmpeg        string
	width, height, fps, rate int
	frameBytes, pcmBytes     int
	encoder                  *exec.Cmd
	stdin                    io.WriteCloser
	stderr                   *tailBuffer
	audio                    *os.File
	frames                   int64
	closed                   bool
	err                      error
}

// New exports a 960x600 H.264/AAC MP4 with nearest-neighbor image scaling.
// sampleRate must divide evenly by fps; PCM is signed 16-bit little-endian
// stereo. An existing output is never overwritten, even if created later.
func New(path string, width, height, fps, sampleRate int) (*Recorder, error) {
	if path == "" {
		return nil, errors.New("recording: an output path is required")
	}
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return nil, errors.New("recording: source dimensions must be between 1 and 16384")
	}
	if fps <= 0 || fps > 240 || sampleRate < 8000 || sampleRate > 192000 || sampleRate%fps != 0 {
		return nil, errors.New("recording: sample rate must divide evenly by a frame rate between 1 and 240")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("recording output path: %w", err)
	}
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("recording output already exists: %w", os.ErrExist)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("recording output: %w", err)
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("recording requires FFmpeg: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("recording output directory: %w", err)
	}
	dir, err := os.MkdirTemp(filepath.Dir(path), ".populous2-recording-*")
	if err != nil {
		return nil, fmt.Errorf("recording temporary directory: %w", err)
	}
	r := &Recorder{
		path: path, dir: dir, ffmpeg: ffmpeg, width: width, height: height,
		fps: fps, rate: sampleRate, frameBytes: width * height * 4,
		pcmBytes: sampleRate / fps * 4, stderr: &tailBuffer{},
	}
	r.audio, err = os.Create(filepath.Join(dir, "audio.pcm"))
	if err != nil {
		r.Abort()
		return nil, fmt.Errorf("recording audio: %w", err)
	}
	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2:flags=neighbor,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1", outputWidth, outputHeight, outputWidth, outputHeight)
	r.encoder = exec.Command(ffmpeg,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-n",
		"-f", "rawvideo", "-pixel_format", "rgba", "-video_size", fmt.Sprintf("%dx%d", width, height),
		"-framerate", strconv.Itoa(fps), "-i", "pipe:0", "-an",
		"-vf", filter, "-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p",
		filepath.Join(dir, "video.mp4"),
	)
	r.encoder.Stderr = r.stderr
	r.stdin, err = r.encoder.StdinPipe()
	if err == nil {
		err = r.encoder.Start()
	}
	if err != nil {
		r.Abort()
		return nil, fmt.Errorf("starting recording encoder: %w", err)
	}
	return r, nil
}

// WriteFrame consumes its buffers synchronously; callers may reuse them.
// Each block must contain exactly sampleRate/fps stereo audio samples. This
// keeps the original video cadence and PCM timeline in lockstep.
func (r *Recorder) WriteFrame(rgba, pcm []byte) error {
	if r.closed {
		return errors.New("recording is closed")
	}
	if r.err != nil {
		return r.err
	}
	if len(rgba) != r.frameBytes || len(pcm) != r.pcmBytes {
		r.err = fmt.Errorf("recording frame contains %d RGBA and %d PCM bytes, want %d and %d", len(rgba), len(pcm), r.frameBytes, r.pcmBytes)
		return r.err
	}
	if err := writeAll(r.stdin, rgba); err != nil {
		r.err = fmt.Errorf("recording video: %w%s", err, r.stderr.message())
		return r.err
	}
	if err := writeAll(r.audio, pcm); err != nil {
		r.err = fmt.Errorf("recording audio: %w", err)
		return r.err
	}
	r.frames++
	return nil
}

// Close finishes encoding and publishes the complete MP4 atomically. It does
// not replace another file and returns the same result on subsequent calls.
func (r *Recorder) Close() error {
	if r.closed {
		return r.err
	}
	if r.err != nil {
		r.Abort()
		return r.err
	}
	if r.frames == 0 {
		r.err = errors.New("recording contains no frames")
		r.Abort()
		return r.err
	}
	r.closed = true
	defer r.cleanup()
	pipeErr := r.stdin.Close()
	encodeErr := r.encoder.Wait()
	audioErr := r.audio.Close()
	if encodeErr != nil {
		r.err = fmt.Errorf("encoding recording video: %w%s", encodeErr, r.stderr.message())
		return r.err
	}
	if err := errors.Join(pipeErr, audioErr); err != nil {
		r.err = fmt.Errorf("finishing recording streams: %w", err)
		return r.err
	}
	duration := strconv.FormatFloat(float64(r.frames)/float64(r.fps), 'f', 9, 64)
	mux := exec.Command(r.ffmpeg,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-n",
		"-i", filepath.Join(r.dir, "video.mp4"),
		"-f", "s16le", "-ar", strconv.Itoa(r.rate), "-ac", "2", "-i", filepath.Join(r.dir, "audio.pcm"),
		"-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
		"-t", duration, "-movflags", "+faststart", filepath.Join(r.dir, "complete.mp4"),
	)
	stderr := &tailBuffer{}
	mux.Stderr = stderr
	if err := mux.Run(); err != nil {
		r.err = fmt.Errorf("muxing recording audio: %w%s", err, stderr.message())
		return r.err
	}
	// The temporary movie is on the output filesystem. Link is atomic and
	// fails when the destination exists, whereas Rename could overwrite it.
	if err := os.Link(filepath.Join(r.dir, "complete.mp4"), r.path); err != nil {
		r.err = fmt.Errorf("publishing recording without overwriting %q: %w", r.path, err)
	}
	return r.err
}

// Abort stops the encoder and removes only this recorder's temporary files.
// It never removes a published movie or a pre-existing destination.
func (r *Recorder) Abort() {
	if r == nil || r.closed {
		return
	}
	r.closed = true
	if r.err == nil {
		r.err = errors.New("recording aborted")
	}
	if r.stdin != nil {
		_ = r.stdin.Close()
	}
	if r.encoder != nil && r.encoder.Process != nil {
		_ = r.encoder.Process.Kill()
		_ = r.encoder.Wait()
	}
	if r.audio != nil {
		_ = r.audio.Close()
	}
	r.cleanup()
}

func (r *Recorder) cleanup() {
	for _, name := range []string{"audio.pcm", "video.mp4", "complete.mp4"} {
		_ = os.Remove(filepath.Join(r.dir, name))
	}
	_ = os.Remove(r.dir)
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) != 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

// FFmpeg can report errors while the video pipe is being written. Keep the
// latest diagnostic without letting subprocess output grow without bound.
type tailBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	const limit = 32 * 1024
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if len(p) >= limit {
		b.data = append(b.data[:0], p[len(p)-limit:]...)
	} else {
		if overflow := len(b.data) + len(p) - limit; overflow > 0 {
			copy(b.data, b.data[overflow:])
			b.data = b.data[:len(b.data)-overflow]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

func (b *tailBuffer) message() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if text := strings.TrimSpace(string(b.data)); text != "" {
		return ": " + text
	}
	return ""
}
