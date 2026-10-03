package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jasonsoprovich/pq-companion/backend/internal/raidcomp"
)

// newRaidSplitTestRouter wires a raidsHandler to a fresh temp user.db and
// registers only the split route, mirroring router.go.
func newRaidSplitTestRouter(t *testing.T) (*raidsHandler, *chi.Mux, *raidcomp.Store) {
	t.Helper()
	s, err := raidcomp.OpenStore(filepath.Join(t.TempDir(), "user.db"), nil)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	h := &raidsHandler{store: s, roster: raidcomp.NewRoster(), pipe: nil, liveZone: nil}
	r := chi.NewRouter()
	r.Post("/api/raids/split", h.splitComp)
	return h, r, s
}

func doSplit(t *testing.T, r *chi.Mux, body map[string]any) (int, map[string]any) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := doReq(t, r, http.MethodPost, "/api/raids/split", b)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response not JSON: %v (%s)", err, rec.Body.String())
	}
	return rec.Code, out
}

func TestRaidSplit_RequiresStore(t *testing.T) {
	h := &raidsHandler{store: nil}
	r := chi.NewRouter()
	r.Post("/api/raids/split", h.splitComp)
	rec := doReq(t, r, http.MethodPost, "/api/raids/split", []byte(`{}`))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503 when store nil, got %d", rec.Code)
	}
}

func TestRaidSplit_BadRequests(t *testing.T) {
	_, r, s := newRaidSplitTestRouter(t)
	if err := s.SaveEncounter(validSplitEncounter()); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Missing encounter_id.
	code, _ := doSplit(t, r, map[string]any{"preference": "trinity"})
	if code != http.StatusBadRequest {
		t.Errorf("missing encounter_id: want 400, got %d", code)
	}
	// Unknown encounter.
	code, _ = doSplit(t, r, map[string]any{"encounter_id": "nope", "preference": "trinity"})
	if code != http.StatusNotFound {
		t.Errorf("unknown encounter: want 404, got %d", code)
	}
	// Invalid preference.
	code, _ = doSplit(t, r, map[string]any{"encounter_id": "split-test", "preference": "bogus"})
	if code != http.StatusBadRequest {
		t.Errorf("invalid preference: want 400, got %d", code)
	}
	// Group size out of range.
	code, _ = doSplit(t, r, map[string]any{"encounter_id": "split-test", "preference": "trinity", "group_size": 13})
	if code != http.StatusBadRequest {
		t.Errorf("group_size 13: want 400, got %d", code)
	}
	// Unknown field rejected (decodeJSON is strict).
	code, _ = doSplit(t, r, map[string]any{"encounter_id": "split-test", "preference": "trinity", "bogus_field": 1})
	if code != http.StatusBadRequest {
		t.Errorf("unknown field: want 400, got %d", code)
	}
}

func TestRaidSplit_ManualRosterProposal(t *testing.T) {
	_, r, s := newRaidSplitTestRouter(t)
	if err := s.SaveEncounter(validSplitEncounter()); err != nil {
		t.Fatalf("save: %v", err)
	}
	code, out := doSplit(t, r, map[string]any{
		"encounter_id": "split-test",
		"preference":   "trinity",
		"roster": []map[string]any{
			{"name": "Bonce", "class": "war", "group": "1"},
			{"name": "Cureall", "class": "Cleric", "group": "1"},
			{"name": "Stabber", "class": "rogue"},
			{"name": "Mystery", "class": ""},
		},
	})
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d: %v", code, out)
	}
	groups, ok := out["groups"].([]any)
	if !ok || len(groups) == 0 {
		t.Fatalf("no groups in response: %v", out)
	}
	// Every roster member seated.
	seated := 0
	for _, g := range groups {
		slots := g.(map[string]any)["slots"].([]any)
		for _, slot := range slots {
			name := slot.(map[string]any)["member"].(string)
			if name == "Mystery" {
				// Classless member: seated as fill, no comp role.
				if role := slot.(map[string]any)["role"].(string); role != "" {
					t.Errorf("classless member got comp role %q", role)
				}
			}
			seated++
		}
	}
	if seated != 4 {
		t.Errorf("seated %d members, want 4", seated)
	}
	// Cureall (Cleric, free-text class) must resolve to code clr and hold
	// the healer slot.
	foundClericHealer := false
	for _, g := range groups {
		for _, slot := range g.(map[string]any)["slots"].([]any) {
			s := slot.(map[string]any)
			if s["member"] == "Cureall" && s["role"] == "healer" && s["class"] == "clr" {
				foundClericHealer = true
			}
		}
	}
	if !foundClericHealer {
		t.Errorf("Cureall should hold a healer slot with class clr; got %v", out["groups"])
	}
}

func TestRaidSplit_CuratedCapsSurfaceInResponse(t *testing.T) {
	_, r, s := newRaidSplitTestRouter(t)
	if err := s.SaveEncounter(validSplitEncounter()); err != nil {
		t.Fatalf("save: %v", err)
	}
	code, out := doSplit(t, r, map[string]any{
		"encounter_id": "split-test",
		"preference":   "curated",
		"wildcards": []map[string]any{
			{"kind": "member", "member": "Ghost", "group": 1},
			{"kind": "class", "value": "clr", "max": 0, "min": 1},
		},
	})
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d: %v", code, out)
	}
	warnings, _ := out["warnings"].([]any)
	joined := strings.Join(toStrings(warnings), " | ")
	if !strings.Contains(joined, "Ghost") {
		t.Errorf("want a warning about pinned member Ghost, got %q", joined)
	}
	if !strings.Contains(joined, "wanted at least 1") {
		t.Errorf("want an unmet-min warning for clr, got %q", joined)
	}
}

func toStrings(v []any) []string {
	out := make([]string, 0, len(v))
	for _, x := range v {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// validSplitEncounter is the shared encounter fixture for the split tests.
func validSplitEncounter() *raidcomp.Encounter {
	return &raidcomp.Encounter{
		ID: "split-test", Name: "Split Test Encounter", Zone: "Kael Drakkel",
		ZoneID: 113, Status: raidcomp.StatusActive,
		Comps: []raidcomp.CompRow{
			{Role: "tank", Sub: "defensive", Min: 1, Rec: 2},
			{Role: "healer", Sub: "ch_cleric", Min: 1, Rec: 2},
			{Role: "damage", Min: 2, Rec: 4},
		},
	}
}

// TestRaidSplit_NoNullArrays pins the no-null contract: Go nil slices
// marshal as JSON null and the report renderer indexes these arrays
// unconditionally, so every list field must be [] even when empty (the
// null-classes lesson, applied to the split report).
func TestRaidSplit_NoNullArrays(t *testing.T) {
	_, r, s := newRaidSplitTestRouter(t)
	if err := s.SaveEncounter(validSplitEncounter()); err != nil {
		t.Fatalf("save: %v", err)
	}
	for name, body := range map[string]map[string]any{
		"small roster": { // everyone seats → empty unassigned
			"encounter_id": "split-test",
			"preference":   "trinity",
			"roster": []map[string]any{
				{"name": "Tank", "class": "war"},
				{"name": "Heal", "class": "clr"},
				{"name": "Dps", "class": "rog"},
			},
		},
		"empty roster": {"encounter_id": "split-test", "preference": "trinity"},
		"compless encounter": {"encounter_id": "no-comps", "preference": "trinity"},
	} {
		t.Run(name, func(t *testing.T) {
			if name == "compless encounter" {
				if err := s.SaveEncounter(&raidcomp.Encounter{ID: "no-comps", Name: "No Comps", Zone: "kael", Status: raidcomp.StatusActive}); err != nil {
					t.Fatalf("save: %v", err)
				}
			}
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			rec := doReq(t, r, http.MethodPost, "/api/raids/split", b)
			if rec.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
			}
			for _, field := range []string{"groups", "unassigned", "min", "rec"} {
				if strings.Contains(rec.Body.String(), `"`+field+`":null`) {
					t.Errorf("response contains %q:null — nil slice leaked (body: %s)", field, rec.Body.String())
				}
			}
		})
	}
}
