package tts

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// UpmixMonoWAV rewrites the WAV at path as 2-channel stereo (each sample
// duplicated into left and right) when it is 16-bit PCM mono, and leaves any
// other file untouched. Piper voices emit mono WAVs; Chromium plays those
// through a single output channel on some devices (virtual-surround
// headsets notably), so the alert is heard in the left ear only. Writing
// real stereo removes the device's choice.
//
// Idempotent and cheap to call on a cache hit: the already-stereo case is
// decided from the first few bytes, without reading the whole file.
func UpmixMonoWAV(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var riff [12]byte
	if _, err := io.ReadFull(f, riff[:]); err != nil {
		return nil // too short to be a WAV; not ours to touch
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return nil
	}

	// Walk chunks until fmt, peeking only the headers.
	var channels, bits, format uint16
	var sampleRate uint32
	sawFmt := false
	var dataOff, dataLen int64 = -1, 0
	pos := int64(12)
	for {
		var hdr [8]byte
		if _, err := f.ReadAt(hdr[:], pos); err != nil {
			break
		}
		id := string(hdr[0:4])
		size := int64(binary.LittleEndian.Uint32(hdr[4:8]))
		body := pos + 8
		switch id {
		case "fmt ":
			var b [16]byte
			if _, err := f.ReadAt(b[:], body); err != nil {
				return nil
			}
			format = binary.LittleEndian.Uint16(b[0:2])
			channels = binary.LittleEndian.Uint16(b[2:4])
			sampleRate = binary.LittleEndian.Uint32(b[4:8])
			bits = binary.LittleEndian.Uint16(b[14:16])
			sawFmt = true
			if format != 1 || channels != 1 || bits != 16 {
				return nil // not 16-bit PCM mono: nothing to do
			}
		case "data":
			dataOff, dataLen = body, size
		}
		if dataOff >= 0 && sawFmt {
			break
		}
		pos = body + size + (size & 1) // chunks are word-aligned
	}
	if !sawFmt || dataOff < 0 {
		return nil
	}

	st, err := f.Stat()
	if err != nil {
		return err
	}
	// A streamed WAV can carry a placeholder (0 / 0xFFFFFFFF) data size, or
	// one larger than what was actually written; trust the file length.
	if remaining := st.Size() - dataOff; dataLen == 0 || dataLen > remaining {
		dataLen = remaining
	}
	dataLen &^= 1 // whole 16-bit samples only
	if dataLen <= 0 {
		return nil
	}

	mono := make([]byte, dataLen)
	if _, err := f.ReadAt(mono, dataOff); err != nil {
		return fmt.Errorf("read wav data: %w", err)
	}

	stereo := make([]byte, 0, dataLen*2)
	for i := int64(0); i+1 < dataLen; i += 2 {
		stereo = append(stereo, mono[i], mono[i+1], mono[i], mono[i+1])
	}

	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(36+len(stereo)))
	out.WriteString("WAVEfmt ")
	_ = binary.Write(&out, binary.LittleEndian, uint32(16))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1)) // PCM
	_ = binary.Write(&out, binary.LittleEndian, uint16(2)) // channels
	_ = binary.Write(&out, binary.LittleEndian, sampleRate)
	_ = binary.Write(&out, binary.LittleEndian, sampleRate*4) // byte rate
	_ = binary.Write(&out, binary.LittleEndian, uint16(4))    // block align
	_ = binary.Write(&out, binary.LittleEndian, uint16(16))   // bits
	out.WriteString("data")
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(stereo)))
	out.Write(stereo)

	if int64(out.Len()) > int64(^uint32(0)) {
		return errors.New("wav too large to upmix")
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".upmix-*.wav")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_, werr := tmp.Write(out.Bytes())
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmpPath)
		if werr != nil {
			return werr
		}
		return cerr
	}
	// Windows can't rename over a file another handle has open (our own f
	// above); close it first.
	_ = f.Close()
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
