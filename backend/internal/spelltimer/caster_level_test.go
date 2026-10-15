package spelltimer

import (
	"testing"

	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
	"github.com/jasonsoprovich/pq-companion/backend/internal/ws"
)

// levelEngine builds a DB-backed engine for an active character of the given
// class index and level (0 = unknown level).
func levelEngine(t *testing.T, class, level int) (*Engine, *db.DB) {
	t.Helper()
	database, err := db.Open("../../data/quarm.db")
	if err != nil {
		t.Skipf("quarm.db not available: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	charCtx := func() (string, string, int) { return "", "Tester", class }
	e := NewEngine(ws.NewHub(), database, charCtx,
		func() string { return scopeAnyone }, nil, nil, nil, nil, nil)
	e.SetLevelProvider(func() int { return level })
	return e, database
}

func TestRescaleFormulaDuration(t *testing.T) {
	const (
		enchanter = 13
		warrior   = 0
		tashanian = 1702 // formula 9, base 140: 130 ticks at 60, 140 at 65
	)
	tests := []struct {
		name  string
		class int
		level int
		in    float64
		want  float64
	}{
		{"own cast at 65 extends level-60 pack value", enchanter, 65, 780, 840},
		{"own cast at 58 shortens it", enchanter, 58, 780, 756},
		{"unknown level assumes cap", enchanter, 0, 780, 840},
		{"someone else's cast assumes cap", warrior, 50, 780, 840},
		{"non-formula value is a user override", enchanter, 65, 700, 700},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, database := levelEngine(t, tt.class, tt.level)
			spell, err := database.GetSpell(tashanian)
			if err != nil || spell == nil {
				t.Fatalf("spell %d: %v", tashanian, err)
			}
			if got := e.rescaleFormulaDuration(spell, tt.in, ""); got != tt.want {
				t.Errorf("got %g, want %g", got, tt.want)
			}
		})
	}
}
