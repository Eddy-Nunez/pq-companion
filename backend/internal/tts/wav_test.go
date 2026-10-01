package tts

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func buildWAV(channels uint16, bits uint16, samples []int16) []byte {
	var data bytes.Buffer
	for _, s := range samples {
		_ = binary.Write(&data, binary.LittleEndian, s)
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+data.Len()))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, channels)
	_ = binary.Write(&b, binary.LittleEndian, uint32(22050))
	_ = binary.Write(&b, binary.LittleEndian, uint32(22050*uint32(channels)*uint32(bits/8)))
	_ = binary.Write(&b, binary.LittleEndian, channels*bits/8)
	_ = binary.Write(&b, binary.LittleEndian, bits)
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(data.Len()))
	b.Write(data.Bytes())
	return b.Bytes()
}

func TestUpmixMonoWAV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.wav")
	if err := os.WriteFile(path, buildWAV(1, 16, []int16{100, -200, 300}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpmixMonoWAV(path); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := buildWAV(2, 16, []int16{100, 100, -200, -200, 300, 300})
	if !bytes.Equal(got, want) {
		t.Fatalf("stereo output mismatch\n got %v\nwant %v", got, want)
	}

	// Idempotent: a second pass leaves the stereo file untouched.
	if err := UpmixMonoWAV(path); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if !bytes.Equal(again, want) {
		t.Fatal("second upmix changed an already-stereo file")
	}
}

func TestUpmixMonoWAV_LeavesOtherFilesAlone(t *testing.T) {
	dir := t.TempDir()
	cases := map[string][]byte{
		"8bit.wav":   buildWAV(1, 8, []int16{1, 2}),
		"notwav.wav": []byte("definitely not a wav file at all"),
		"tiny.wav":   []byte("RIFF"),
		"stereo.wav": buildWAV(2, 16, []int16{1, 2, 3, 4}),
	}
	for name, content := range cases {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := UpmixMonoWAV(p); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		got, _ := os.ReadFile(p)
		if !bytes.Equal(got, content) {
			t.Errorf("%s was modified", name)
		}
	}
}

func TestUpmixMonoWAV_StreamedPlaceholderSize(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.wav")
	b := buildWAV(1, 16, []int16{7, 8})
	binary.LittleEndian.PutUint32(b[40:44], 0xFFFFFFFF) // data size placeholder
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpmixMonoWAV(p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, buildWAV(2, 16, []int16{7, 7, 8, 8})) {
		t.Fatalf("placeholder-size WAV not upmixed correctly: %v", got)
	}
}
