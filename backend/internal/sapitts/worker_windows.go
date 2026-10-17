//go:build windows

package sapitts

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
)

const (
	startTimeout   = 30 * time.Second
	requestTimeout = 15 * time.Second
	createNoWindow = 0x08000000
)

// workerScript runs inside powershell.exe. It reads one base64(UTF-8 JSON)
// request per stdin line — base64 so voice names, phrase text, and a
// non-ASCII profile path survive the console code page — and answers each
// with a single "OK" or "ERR <message>" line.
//
// Voice resolution mirrors how Chromium names Web Speech voices on Windows:
// OneCore voices ("Microsoft David - English (United States)") are matched
// through the WinRT synthesizer by description; classic SAPI desktop voices
// ("Microsoft David Desktop - English (United States)") through
// System.Speech by name. An empty voice uses the Windows default voice. A
// named voice that matches neither is an error, so the caller falls back to
// Web Speech with the right voice rather than speaking the wrong one.
const workerScript = `
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Speech
$sapi = New-Object System.Speech.Synthesis.SpeechSynthesizer
$sapiDefault = $sapi.Voice.Name
$rt = $null
try {
  Add-Type -AssemblyName System.Runtime.WindowsRuntime
  $rtType = [Windows.Media.SpeechSynthesis.SpeechSynthesizer,Windows.Media.SpeechSynthesis,ContentType=WindowsRuntime]
  $streamType = [Windows.Media.SpeechSynthesis.SpeechSynthesisStream,Windows.Media.SpeechSynthesis,ContentType=WindowsRuntime]
  $asTask = [System.WindowsRuntimeSystemExtensions].GetMethods() |
    Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and
      $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation` + "`" + `1' } |
    Select-Object -First 1
  $asTaskStream = $asTask.MakeGenericMethod($streamType)
  $rt = New-Object Windows.Media.SpeechSynthesis.SpeechSynthesizer
} catch { $rt = $null }

function Find-RtVoice([string]$name) {
  if ($null -eq $rt) { return $null }
  if (-not $name) { return $rtType::DefaultVoice }
  $short = ($name -split ' - ')[0]
  foreach ($v in $rtType::AllVoices) {
    if ($v.Description -eq $name -or $v.DisplayName -eq $name -or $v.DisplayName -eq $short) { return $v }
  }
  return $null
}

function Find-SapiVoice([string]$name) {
  if (-not $name) { return $sapiDefault }
  $short = ($name -split ' - ')[0]
  foreach ($iv in $sapi.GetInstalledVoices()) {
    $info = $iv.VoiceInfo
    if ($iv.Enabled -and ($info.Name -eq $name -or $info.Name -eq $short -or $info.Description -eq $name)) {
      return $info.Name
    }
  }
  return $null
}

function Speak-Rt($voice, $req) {
  $rt.Voice = $voice
  try { $rt.Options.SpeakingRate = [double]$req.rate } catch {}
  $task = $asTaskStream.Invoke($null, @($rt.SynthesizeTextToStreamAsync([string]$req.text)))
  if (-not $task.Wait(12000)) { throw 'WinRT synthesis timed out' }
  $stream = $task.Result
  $in = [System.IO.WindowsRuntimeStreamExtensions]::AsStreamForRead($stream)
  $fs = [System.IO.File]::Create([string]$req.out)
  try { $in.CopyTo($fs) } finally { $fs.Dispose(); $in.Dispose(); $stream.Dispose() }
}

function Speak-Sapi([string]$voiceName, $req) {
  $sapi.SelectVoice($voiceName)
  $sapi.Rate = [int]$req.srate
  $sapi.Volume = 100
  $sapi.SetOutputToWaveFile([string]$req.out)
  try { $sapi.Speak([string]$req.text) } finally { $sapi.SetOutputToNull() }
}

[Console]::Out.WriteLine('READY')
[Console]::Out.Flush()
while ($true) {
  $line = [Console]::In.ReadLine()
  if ($null -eq $line) { break }
  try {
    $req = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($line)) | ConvertFrom-Json
    $voice = [string]$req.voice
    $rv = Find-RtVoice $voice
    if ($null -ne $rv) {
      Speak-Rt $rv $req
    } else {
      $sv = Find-SapiVoice $voice
      if ($null -eq $sv) { throw "voice not found: $voice" }
      Speak-Sapi $sv $req
    }
    [Console]::Out.WriteLine('OK')
  } catch {
    $msg = ($_.Exception.Message -replace '[\r\n]+', ' ')
    [Console]::Out.WriteLine("ERR $msg")
  }
  [Console]::Out.Flush()
}
`

// worker is one persistent powershell.exe running workerScript.
type worker struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	lines   chan string
	isDead  atomic.Bool
	stopped atomic.Bool
}

func encodePowerShell(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		b[2*i] = byte(c)
		b[2*i+1] = byte(c >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func startWorker() (*worker, error) {
	cmd := exec.Command("powershell.exe",
		"-NoLogo", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodePowerShell(workerScript))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start powershell: %w", err)
	}
	w := &worker{cmd: cmd, stdin: stdin, lines: make(chan string, 4)}
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			w.lines <- strings.TrimSpace(sc.Text())
		}
		w.isDead.Store(true)
		close(w.lines)
		_ = cmd.Wait()
	}()

	timer := time.NewTimer(startTimeout)
	defer timer.Stop()
	for {
		select {
		case line, ok := <-w.lines:
			if !ok {
				return nil, errors.New("speech worker exited during startup")
			}
			if line == "READY" {
				slog.Debug("sapitts: worker started", "pid", cmd.Process.Pid)
				return w, nil
			}
		case <-timer.C:
			w.stop()
			return nil, errors.New("speech worker did not start in time")
		}
	}
}

func (w *worker) dead() bool { return w.isDead.Load() }

func (w *worker) stop() {
	if !w.stopped.CompareAndSwap(false, true) {
		return
	}
	w.isDead.Store(true)
	_ = w.stdin.Close() // ends the ReadLine loop for a clean exit
	if w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
	}
}

// synthesize sends one request and waits for its reply. Callers serialize
// (Service.mu), so replies can't interleave. A timeout or cancellation kills
// the worker — a late reply would otherwise be read as the next request's.
func (w *worker) synthesize(ctx context.Context, req request) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	line := base64.StdEncoding.EncodeToString(body) + "\n"
	if _, err := io.WriteString(w.stdin, line); err != nil {
		w.stop()
		return fmt.Errorf("write to speech worker: %w", err)
	}
	timer := time.NewTimer(requestTimeout)
	defer timer.Stop()
	select {
	case reply, ok := <-w.lines:
		if !ok {
			return errors.New("speech worker exited")
		}
		if reply == "OK" {
			return nil
		}
		return errors.New(strings.TrimPrefix(reply, "ERR "))
	case <-timer.C:
		w.stop()
		return errors.New("speech synthesis timed out")
	case <-ctx.Done():
		w.stop()
		return ctx.Err()
	}
}
