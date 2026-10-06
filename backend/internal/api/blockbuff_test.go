package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jasonsoprovich/pq-companion/backend/internal/blockbuff"
	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
)

func newBlockbuffRouter(t *testing.T) (*chi.Mux, *db.DB, *blockbuff.Store) {
	t.Helper()
	d := openFocusTestDB(t)
	store, err := blockbuff.OpenStore(filepath.Join(t.TempDir(), "user.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	h := &blockbuffHandler{store: store, db: d}
	r := chi.NewRouter()
	r.Route("/api/blockbuffs", func(r chi.Router) {
		r.Get("/", h.get)
		r.Put("/", h.put)
		r.Post("/adopt", h.adopt)
	})
	return r, d, store
}

func doJSON(r http.Handler, method, url string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, url, &buf)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// findSpell returns the first spell matching name whose GoodEffect equals good.
func findSpell(t *testing.T, d *db.DB, name string, good int) *db.Spell {
	t.Helper()
	res, err := d.SearchSpells(name, -1, 0, 0, 50, 0, false)
	if err != nil {
		t.Fatalf("SearchSpells: %v", err)
	}
	for i := range res.Items {
		if res.Items[i].Name == name && res.Items[i].GoodEffect == good {
			return &res.Items[i]
		}
	}
	t.Skipf("spell %q (good=%d) not in test db", name, good)
	return nil
}

func TestBlockbuffPutValidatesAndReconciles(t *testing.T) {
	r, d, store := newBlockbuffRouter(t)
	sow := findSpell(t, d, "Spirit of Wolf", 1)
	nuke := findSpell(t, d, "Lightning Bolt", 0)

	// A detrimental spell can't be blocked server-side, so it is rejected.
	w := doJSON(r, http.MethodPut, "/api/blockbuffs/", map[string]any{
		"character": "Osui", "entries": []blockbuff.Entry{{SpellID: nuke.ID}},
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("non-buff status = %d, want 400 (%s)", w.Code, w.Body.String())
	}
	// Unknown spell ids are rejected too.
	w = doJSON(r, http.MethodPut, "/api/blockbuffs/", map[string]any{
		"character": "Osui", "entries": []blockbuff.Entry{{SpellID: 99999999}},
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown spell status = %d, want 400", w.Code)
	}
	// Path-like character names are rejected.
	w = doJSON(r, http.MethodPut, "/api/blockbuffs/", map[string]any{"character": "../x"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("unsafe character status = %d, want 400", w.Code)
	}

	// A valid save with no server listing seen yet is "unconfirmed" and
	// produces a block command.
	w = doJSON(r, http.MethodPut, "/api/blockbuffs/", map[string]any{
		"character": "Osui", "entries": []blockbuff.Entry{{SpellID: sow.ID}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("put status = %d (%s)", w.Code, w.Body.String())
	}
	var v blockbuffViewDTO
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Rows) != 1 || v.Rows[0].Status != blockbuff.StatusUnconfirmed || v.Rows[0].SpellName != "Spirit of Wolf" {
		t.Fatalf("rows = %+v", v.Rows)
	}
	if len(v.Commands) != 1 || v.Commands[0] != blockbuff.Command(blockbuff.Entry{SpellID: sow.ID}, true) {
		t.Errorf("commands = %v", v.Commands)
	}

	// After the server lists an empty set, the same row needs a block, and an
	// observed-only row from a later listing is offered for adoption.
	if _, err := store.ObserveSnapshot("Osui", nil, 1000); err != nil {
		t.Fatal(err)
	}
	w = doJSON(r, http.MethodGet, "/api/blockbuffs/?character=Osui", nil)
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if v.SyncedAt != 1000 || v.Rows[0].Status != blockbuff.StatusNeedsBlock {
		t.Errorf("synced view = %+v", v)
	}

	if _, err := store.ObserveSnapshot("Osui", []blockbuff.Entry{{SpellID: sow.ID}}, 2000); err != nil {
		t.Fatal(err)
	}
	w = doJSON(r, http.MethodPost, "/api/blockbuffs/adopt", map[string]any{"character": "Osui"})
	if w.Code != http.StatusOK {
		t.Fatalf("adopt status = %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if len(v.Rows) != 1 || v.Rows[0].Status != blockbuff.StatusApplied || len(v.Commands) != 0 {
		t.Errorf("after adopt = %+v", v)
	}
}
