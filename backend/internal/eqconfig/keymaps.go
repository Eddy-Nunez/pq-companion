package eqconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ── Zeal Page 10 social keybinds ──────────────────────────────────────────────
//
// Zeal adds SocialPage10Slot1-12 (bind ids 141-152, Zeal/zeal.cpp), each running
// [Socials] Page10Button<N> from <Char>_pq.proj.ini. The binds themselves live
// in eqclient.ini as
//
//	KEYMAP_SocialPage10Slot1=SHIFT+0     primary
//	KEYMAP_SocialPage10Slot3_2=l         alternate
//
// Format confirmed from a player's file (Riz, 2026-10-19): one modifier is
// written SHIFT+, CONTROL+ or ALT+ before the key, plain keys are bare (letters
// lowercase), and NOKEY means unbound. How EQ orders two modifiers together is
// still unseen, so the writer accepts at most one.
//
// Zeal's per-character option ([Zeal] PerCharKeybinds in zeal.ini) moves every
// bind for that character to [KeyMaps_<Name>]; otherwise all characters share
// [KeyMaps] (Zeal/binds.cpp). Zeal never copies between the two.

const (
	// Page10SocialSlots is the number of Zeal Page 10 social binds.
	Page10SocialSlots = 12

	keyMapsSection     = "KeyMaps"
	perCharKeybindsKey = "PerCharKeybinds"
	noKey              = "NOKEY"
)

// KeyBind is one bind's primary and alternate key, as EQ wrote them ("SHIFT+0",
// "l"). Empty means unbound.
type KeyBind struct {
	Slot      int    `json:"slot"`
	Primary   string `json:"primary"`
	Alternate string `json:"alternate"`
}

// Page10Binds is one character's Page 10 social binds.
type Page10Binds struct {
	// Found is false when eqclient.ini is missing.
	Found bool `json:"found"`
	// PerCharacter reports Zeal's PerCharKeybinds, which picks the section.
	PerCharacter bool   `json:"per_character"`
	Section      string `json:"section"`
	// ModifiedAt is eqclient.ini's mtime, sent back on write so a file the
	// client rewrote meanwhile is never clobbered.
	ModifiedAt time.Time `json:"modified_at"`
	Social     []KeyBind `json:"social"`
	// BoundTo maps each key in use anywhere in the section to the binds using
	// it (minus the KEYMAP_ prefix), so the editor can flag a clash such as
	// "l" also being Duck.
	BoundTo map[string][]string `json:"bound_to"`
}

func socialKey(slot int, alternate bool) string {
	k := fmt.Sprintf("KEYMAP_SocialPage10Slot%d", slot)
	if alternate {
		k += "_2"
	}
	return k
}

// readPerCharKeybinds reports zeal.ini's [Zeal] PerCharKeybinds. Missing file
// or key is false, Zeal's own default.
func readPerCharKeybinds(eqPath string) bool {
	b, err := os.ReadFile(filepath.Join(eqPath, zealINI))
	if err != nil {
		return false
	}
	v, ok := getINIValueInSection(string(b), zealSection, perCharKeybindsKey)
	return ok && isTruthy(v)
}

// KeyMapSection returns the eqclient.ini section Zeal reads the character's
// binds from.
func KeyMapSection(eqPath, character string) (string, bool) {
	if readPerCharKeybinds(eqPath) {
		return keyMapsSection + "_" + character, true
	}
	return keyMapsSection, false
}

// ReadPage10Binds reads the character's Page 10 social binds from eqclient.ini.
func ReadPage10Binds(eqPath, character string) (Page10Binds, error) {
	section, perChar := KeyMapSection(eqPath, character)
	out := Page10Binds{
		PerCharacter: perChar,
		Section:      section,
		Social:       make([]KeyBind, 0, Page10SocialSlots),
		BoundTo:      map[string][]string{},
	}
	path := filepath.Join(eqPath, eqClientINI)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, fmt.Errorf("stat %s: %w", eqClientINI, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return out, fmt.Errorf("read %s: %w", eqClientINI, err)
	}
	content := string(b)
	out.Found = true
	out.ModifiedAt = info.ModTime()

	for slot := 1; slot <= Page10SocialSlots; slot++ {
		out.Social = append(out.Social, KeyBind{
			Slot:      slot,
			Primary:   bindValue(content, section, socialKey(slot, false)),
			Alternate: bindValue(content, section, socialKey(slot, true)),
		})
	}
	for name, v := range sectionKeyMaps(content, section) {
		out.BoundTo[v] = append(out.BoundTo[v], name)
	}
	return out, nil
}

// bindValue returns a key's value with NOKEY normalised to "".
func bindValue(content, section, key string) string {
	v, ok := getINIValueInSection(content, section, key)
	if !ok || strings.EqualFold(v, noKey) {
		return ""
	}
	return v
}

// sectionKeyMaps returns every bound KEYMAP_ entry in section, keyed by name
// without the prefix, valued with the key normalised to upper case so "l" and
// "L" compare equal.
func sectionKeyMaps(content, section string) map[string]string {
	out := map[string]string{}
	inTarget := false
	for _, line := range splitLines(content) {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			inTarget = strings.EqualFold(strings.TrimSpace(t[1:len(t)-1]), section)
			continue
		}
		if !inTarget {
			continue
		}
		eq := strings.IndexByte(t, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(t[:eq])
		v := strings.TrimSpace(t[eq+1:])
		if !strings.HasPrefix(strings.ToUpper(k), "KEYMAP_") || v == "" || strings.EqualFold(v, noKey) {
			continue
		}
		out[k[len("KEYMAP_"):]] = strings.ToUpper(v)
	}
	return out
}

// bindKeyRe is what the writer accepts: at most one modifier, then a key EQ is
// known to spell this way. Letters lowercase, as the client writes them.
// Deliberately narrow — a value EQ cannot parse is dropped silently in game,
// so only spellings seen in real files are allowed.
var bindKeyRe = regexp.MustCompile(
	`^(?:(?:SHIFT|CONTROL|ALT)\+)?(?:[a-z0-9]|F(?:[1-9]|1[0-2])|TAB|SPACE|INSERT|\\)$`)

// ValidBindValue reports whether v can be written as a bind ("" clears it).
func ValidBindValue(v string) bool {
	return v == "" || bindKeyRe.MatchString(v)
}

// SetPage10Binds writes the given Page 10 social binds into the character's
// keymap section, touching only those keys. A cleared bind is written as NOKEY
// when the key exists and left absent when it does not, so an untouched file
// gains no noise. eqclient.ini must exist, and base (when non-zero) must match
// its mtime or nothing is written.
func SetPage10Binds(eqPath, character string, binds []KeyBind, base time.Time) error {
	for _, kb := range binds {
		if kb.Slot < 1 || kb.Slot > Page10SocialSlots {
			return fmt.Errorf("slot %d out of range (1..%d)", kb.Slot, Page10SocialSlots)
		}
		for _, v := range []string{kb.Primary, kb.Alternate} {
			if !ValidBindValue(v) {
				return fmt.Errorf("slot %d: unsupported key %q", kb.Slot, v)
			}
		}
	}
	path := filepath.Join(eqPath, eqClientINI)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", eqClientINI, err)
	}
	if !base.IsZero() && !info.ModTime().Equal(base) {
		return ErrChangedOnDisk
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", eqClientINI, err)
	}
	content := string(b)
	section, _ := KeyMapSection(eqPath, character)
	for _, kb := range binds {
		for _, alt := range []bool{false, true} {
			v := kb.Primary
			if alt {
				v = kb.Alternate
			}
			key := socialKey(kb.Slot, alt)
			if v == "" {
				if _, ok := getINIValueInSection(content, section, key); !ok {
					continue
				}
				v = noKey
			}
			content = setINIKeyInSection(content, section, key, v)
		}
	}
	if err := writeFileAtomic(path, content); err != nil {
		return fmt.Errorf("write %s: %w", eqClientINI, err)
	}
	return nil
}

// ErrChangedOnDisk is returned when eqclient.ini changed since it was read.
var ErrChangedOnDisk = errors.New("eqclient.ini changed on disk since it was loaded")
