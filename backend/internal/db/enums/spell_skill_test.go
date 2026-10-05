package enums

import "testing"

func TestSpellSkillName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{12, "Brass Instruments"},
		{41, "Singing"},
		{49, "Stringed Instruments"},
		{54, "Wind Instruments"},
		{70, "Percussion Instruments"}, // Tuyen's Chant of Flame (743)
		{999, ""},
	}
	for _, tt := range tests {
		if got := SpellSkillName(tt.id); got != tt.want {
			t.Errorf("SpellSkillName(%d) = %q, want %q", tt.id, got, tt.want)
		}
	}
}
