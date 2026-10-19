package eqconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rizyClient is a real player's eqclient.ini excerpt (Riz, 2026-10-19), the
// only sample with modifier binds, plus the [KeyMaps] defaults every file has.
const rizyClient = "[Defaults]\r\nLog=TRUE\r\n" +
	"[KeyMaps]\r\nKEYMAP_DUCK = l\r\nKEYMAP_HOTPAGE10 = CONTROL+0\r\n" +
	"[KeyMaps_Rizy]\r\n" +
	"KEYMAP_HOTPAGE10 = CONTROL+0\r\n" +
	"KEYMAP_DUCK = l\r\n" +
	"KEYMAP_HotbarPage10Slot1=NOKEY\r\n" +
	"KEYMAP_SocialPage10Slot1=SHIFT+0\r\n" +
	"KEYMAP_SocialPage10Slot7=SHIFT+7\r\n" +
	"KEYMAP_SocialPage10Slot2=SHIFT+9\r\n" +
	"KEYMAP_SocialPage10Slot3=ALT+1\r\n" +
	"KEYMAP_SocialPage10Slot8=SHIFT+6\r\n" +
	"KEYMAP_SocialPage10Slot3_2=l\r\n"

func writeEQDir(t *testing.T, client string, perChar bool) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, eqClientINI), []byte(client), 0o644); err != nil {
		t.Fatal(err)
	}
	z := "[Zeal]\r\nPerCharKeybinds=false\r\n"
	if perChar {
		z = "[Zeal]\r\nPerCharKeybinds=true\r\n"
	}
	if err := os.WriteFile(filepath.Join(dir, zealINI), []byte(z), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReadPage10Binds(t *testing.T) {
	dir := writeEQDir(t, rizyClient, true)
	got, err := ReadPage10Binds(dir, "Rizy")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || !got.PerCharacter || got.Section != "KeyMaps_Rizy" {
		t.Fatalf("header = %+v", got)
	}
	want := map[int][2]string{
		1: {"SHIFT+0", ""}, 2: {"SHIFT+9", ""}, 3: {"ALT+1", "l"},
		7: {"SHIFT+7", ""}, 8: {"SHIFT+6", ""}, 4: {"", ""},
	}
	for _, kb := range got.Social {
		w, ok := want[kb.Slot]
		if !ok {
			continue
		}
		if kb.Primary != w[0] || kb.Alternate != w[1] {
			t.Errorf("slot %d = %q/%q, want %q/%q", kb.Slot, kb.Primary, kb.Alternate, w[0], w[1])
		}
	}
	if b := got.BoundTo["L"]; len(b) != 2 {
		t.Errorf("L bound to %v, want DUCK and SocialPage10Slot3_2", b)
	}
	if _, ok := got.BoundTo["NOKEY"]; ok {
		t.Error("NOKEY listed as a bound key")
	}
}

func TestReadPage10Binds_SharedSection(t *testing.T) {
	// Per-character off: Zeal reads [KeyMaps], so the character section is
	// ignored even though it exists.
	dir := writeEQDir(t, rizyClient, false)
	got, err := ReadPage10Binds(dir, "Rizy")
	if err != nil {
		t.Fatal(err)
	}
	if got.Section != "KeyMaps" || got.Social[0].Primary != "" {
		t.Errorf("section %q slot1 %q, want KeyMaps and unbound", got.Section, got.Social[0].Primary)
	}
}

func TestSetPage10Binds(t *testing.T) {
	dir := writeEQDir(t, rizyClient, true)
	before, _ := ReadPage10Binds(dir, "Rizy")
	err := SetPage10Binds(dir, "Rizy", []KeyBind{
		{Slot: 1, Primary: "CONTROL+5"},                 // change
		{Slot: 3, Primary: "ALT+1", Alternate: ""},      // clear existing alternate
		{Slot: 4, Primary: "", Alternate: ""},           // untouched, absent
		{Slot: 12, Primary: "F9", Alternate: "SHIFT+q"}, // new
	}, before.ModifiedAt)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, eqClientINI))
	s := string(b)
	for _, want := range []string{
		"KEYMAP_SocialPage10Slot1=CONTROL+5\r\n",
		"KEYMAP_SocialPage10Slot3_2=NOKEY\r\n",
		"KEYMAP_SocialPage10Slot12=F9\r\n",
		"KEYMAP_SocialPage10Slot12_2=SHIFT+q\r\n",
		// Other sections and binds untouched.
		"[KeyMaps]\r\nKEYMAP_DUCK = l\r\n",
		"KEYMAP_SocialPage10Slot7=SHIFT+7\r\n",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "SocialPage10Slot4") {
		t.Error("cleared absent slot 4 was written")
	}
	// New keys land in the character section, not the shared one.
	if i, j := strings.Index(s, "[KeyMaps_Rizy]"), strings.Index(s, "Slot12="); j < i {
		t.Error("slot 12 written outside [KeyMaps_Rizy]")
	}
}

func TestSetPage10Binds_Refuses(t *testing.T) {
	dir := writeEQDir(t, rizyClient, true)
	cases := []struct {
		name  string
		binds []KeyBind
		base  time.Time
		isErr error
	}{
		{"two modifiers", []KeyBind{{Slot: 1, Primary: "SHIFT+CONTROL+1"}}, time.Time{}, nil},
		{"upper-case letter", []KeyBind{{Slot: 1, Primary: "L"}}, time.Time{}, nil},
		{"unknown key", []KeyBind{{Slot: 1, Primary: "PAGEUP"}}, time.Time{}, nil},
		{"slot range", []KeyBind{{Slot: 13, Primary: "F1"}}, time.Time{}, nil},
		{"stale base", []KeyBind{{Slot: 1, Primary: "F1"}}, time.Unix(1, 0), ErrChangedOnDisk},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := SetPage10Binds(dir, "Rizy", tc.binds, tc.base)
			if err == nil {
				t.Fatal("expected an error")
			}
			if tc.isErr != nil && !errors.Is(err, tc.isErr) {
				t.Errorf("err = %v, want %v", err, tc.isErr)
			}
		})
	}
	b, _ := os.ReadFile(filepath.Join(dir, eqClientINI))
	if string(b) != rizyClient {
		t.Error("a refused write changed the file")
	}
}
