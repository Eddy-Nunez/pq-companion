// Package sapitts renders the built-in Windows voices (the ones Chromium
// exposes through Web Speech) to WAV files, so a TTS alert can be played as
// an ordinary audio clip and routed to a user-chosen output device.
//
// Web Speech has no output-device selection — speechSynthesis always plays on
// the Windows default device — so the "Alert output device" preference can
// only reach default-voice TTS by synthesizing it here first. The frontend
// uses this path only when an output device is selected; with the system
// default selected nothing changes. Any failure is reported to the caller,
// which falls back to plain Web Speech so an alert is never lost.
//
// Synthesis runs in one persistent, hidden PowerShell worker (see
// worker_windows.go) rather than a process per phrase: PowerShell plus the
// speech assemblies take ~1s to start, which would make every uncached alert
// late. The worker starts lazily on the first request and exits after an idle
// period. Results share the content-addressed internal/tts cache, so a
// repeated phrase is a file-stat away.
package sapitts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/tts"
)

// ErrUnsupported is returned on platforms without Windows speech.
var ErrUnsupported = errors.New("windows speech synthesis is only available on Windows")

// idleTimeout is how long the worker may sit unused before it is stopped to
// release its memory. The next request restarts it.
const idleTimeout = 15 * time.Minute

// Service serializes synthesis requests onto the single worker. One instance
// is constructed at router startup and reused for every request.
type Service struct {
	baseDir string

	mu     sync.Mutex
	w      *worker
	idle   *time.Timer
	closed bool
}

// NewService returns a Service caching into baseDir's tts-cache (baseDir is
// ~/.pq-companion, the parent of config.yaml).
func NewService(baseDir string) *Service {
	return &Service{baseDir: baseDir}
}

// Synthesize returns the path of a WAV of text spoken by voice (a Web Speech
// voice name, empty = the Windows default voice) at the Web Speech rate
// (1.0 = normal), generating and caching it on a miss.
func (s *Service) Synthesize(ctx context.Context, voice string, rate float64, text string) (string, error) {
	text = tts.NormalizeText(text)
	if text == "" {
		return "", errors.New("text is required")
	}
	if !(rate > 0) || math.IsInf(rate, 0) {
		rate = 1
	}
	rate = math.Min(2, math.Max(0.5, rate))

	path := cachePath(s.baseDir, voice, rate, text)
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		tts.TouchCacheFile(path)
		return path, nil
	}
	if err := os.MkdirAll(tts.CacheDir(s.baseDir), 0o755); err != nil {
		return "", fmt.Errorf("create tts cache dir: %w", err)
	}

	// Render to a non-.wav temp name and rename into place, so a partial
	// file is never served from (or swept as) the cache.
	tmp := path + ".partial-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	defer os.Remove(tmp)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("speech service stopped")
	}
	if s.w == nil || s.w.dead() {
		w, err := startWorker()
		if err != nil {
			return "", err
		}
		s.w = w
	}
	s.resetIdleLocked()

	req := request{Voice: voice, Rate: rate, SAPIRate: sapiRate(rate), Text: text, Out: tmp}
	if err := s.w.synthesize(ctx, req); err != nil {
		if s.w.dead() {
			s.w = nil
		}
		return "", err
	}
	if info, err := os.Stat(tmp); err != nil || info.Size() <= 44 {
		return "", errors.New("speech synthesis produced no audio")
	}
	// Default Windows voices are 16-bit mono; same left-ear-only issue as
	// Piper on some headsets, so write real stereo.
	_ = tts.UpmixMonoWAV(tmp)
	if err := os.Rename(tmp, path); err != nil {
		return "", fmt.Errorf("finalize wav: %w", err)
	}
	return path, nil
}

// Close stops the worker. Further Synthesize calls fail.
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.idle != nil {
		s.idle.Stop()
	}
	if s.w != nil {
		s.w.stop()
		s.w = nil
	}
}

func (s *Service) resetIdleLocked() {
	if s.idle != nil {
		s.idle.Stop()
	}
	s.idle = time.AfterFunc(idleTimeout, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.w != nil {
			s.w.stop()
			s.w = nil
		}
	})
}

// request is one synthesis job sent to the worker.
type request struct {
	Voice    string  `json:"voice"`
	Rate     float64 `json:"rate"`  // Web Speech / WinRT scale, 1.0 = normal
	SAPIRate int     `json:"srate"` // System.Speech scale, -10..10, 0 = normal
	Text     string  `json:"text"`
	Out      string  `json:"out"`
}

// sapiRate maps a Web Speech rate onto System.Speech's -10..10 scale, where
// each step of 10 is roughly a 3x speed change (the SAPI convention).
func sapiRate(rate float64) int {
	r := int(math.Round(10 * math.Log(rate) / math.Log(3)))
	if r < -10 {
		return -10
	}
	if r > 10 {
		return 10
	}
	return r
}

// cachePath is the content-addressed WAV path for (voice, rate, text),
// namespaced so it can't collide with Piper/Kokoro entries in the shared
// tts-cache dir.
func cachePath(baseDir, voice string, rate float64, text string) string {
	key := "sapi\x00" + voice + "\x00" + strconv.FormatFloat(rate, 'f', 2, 64) + "\x00" + text
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(tts.CacheDir(baseDir), hex.EncodeToString(sum[:])+".wav")
}
