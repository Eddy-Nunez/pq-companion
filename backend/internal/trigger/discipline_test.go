package trigger

import "testing"

func TestDisciplineReuseSecs(t *testing.T) {
	tests := []struct {
		name    string
		spellID int
		class   int
		level   int
		want    int
		wantOK  bool
	}{
		{"duelist at use level", 4676, ClassRogue, 59, 1800, true},
		{"duelist at 60", 4676, ClassRogue, 60, 1746, true},
		{"duelist at 65", 4676, ClassRogue, 65, 1476, true},
		// Two Discord reports from 2026-07-22 (LIMITATIONS.md §11.3).
		{"resistant monk 60", 4585, ClassMonk, 60, 1980, true},
		{"resistant bard 51", 4585, ClassBard, 51, 3600, true},
		{"sub-1620 base stops scaling at 60", 4499, ClassWarrior, 65, 630, true},
		{"clamped to 234 minimum", 4510, ClassMonk, 65, 234, true},
		{"below use level", 4676, ClassRogue, 58, 0, false},
		{"wrong class", 4676, ClassMonk, 65, 0, false},
		{"not a discipline", 1546, ClassCleric, 65, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DisciplineReuseSecs(tt.spellID, tt.class, tt.level)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("got (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
