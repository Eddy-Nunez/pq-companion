package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestRaidsEndpoints_DisabledWhenStoreNil verifies every /api/raids/* route
// responds 503 rather than panicking when the raid store failed to open at
// startup (store is nil in that case — the same non-fatal pattern as
// lockouts/combat-history/emotes). Before the unavailable() guard was added,
// each of these dereferenced a nil *raidcomp.Store and middleware.Recoverer
// turned the panic into an opaque 500.
func TestRaidsEndpoints_DisabledWhenStoreNil(t *testing.T) {
	h := &raidsHandler{store: nil, roster: nil, pipe: nil}
	r := chi.NewRouter()
	r.Get("/api/raids/taxonomy", h.taxonomy)
	r.Get("/api/raids/roles", h.listRoles)
	r.Post("/api/raids/roles", h.saveRole)
	r.Delete("/api/raids/roles", h.deleteRole)
	r.Get("/api/raids/encounters", h.listEncounters)
	r.Get("/api/raids/encounters/{id}", h.getEncounter)
	r.Post("/api/raids/encounters", h.createEncounter)
	r.Put("/api/raids/encounters/{id}", h.updateEncounter)
	r.Delete("/api/raids/encounters/{id}", h.deleteEncounter)
	r.Post("/api/raids/check", h.checkComp)

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/raids/taxonomy"},
		{http.MethodGet, "/api/raids/roles"},
		{http.MethodPost, "/api/raids/roles"},
		{http.MethodDelete, "/api/raids/roles"},
		{http.MethodGet, "/api/raids/encounters"},
		{http.MethodGet, "/api/raids/encounters/aow"},
		{http.MethodPost, "/api/raids/encounters"},
		{http.MethodPut, "/api/raids/encounters/aow"},
		{http.MethodDelete, "/api/raids/encounters/aow"},
		{http.MethodPost, "/api/raids/check"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status = %d, want 503 when store is nil", c.method, c.path, rec.Code)
		}
	}
}
