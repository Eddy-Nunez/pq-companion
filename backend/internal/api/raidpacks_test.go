package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jasonsoprovich/pq-companion/backend/internal/raidcomp"
)

// newRaidPackTestRouter builds a raidsHandler wired to a fresh temp user.db
// (seeded with the starter knowledge base) and registers exactly the pack
// routes under test, mirroring the router.go wiring.
func newRaidPackTestRouter(t *testing.T) (*raidsHandler, *chi.Mux, *raidcomp.Store) {
	t.Helper()
	s, err := raidcomp.OpenStore(filepath.Join(t.TempDir(), "user.db"), nil)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	h := &raidsHandler{store: s, roster: nil, pipe: nil, liveZone: nil}
	r := chi.NewRouter()
	r.Get("/api/raids/taxonomy", h.taxonomy)
	r.Get("/api/raids/roles", h.listRoles)
	r.Get("/api/raids/export", h.exportPack)
	r.Get("/api/raids/encounters/{id}/export", h.exportEncounter)
	r.Post("/api/raids/import/preview", h.importPreview)
	r.Post("/api/raids/import/commit", h.importCommit)
	r.Post("/api/raids/roles/provision", h.provisionRoles)
	return h, r, s
}

func packBody(t *testing.T, pack raidPack) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

func testPack(encs ...raidcomp.Encounter) raidPack {
	return raidPack{
		Kind: raidPackKind, Version: raidPackVersion, PackName: "test pack",
		Encounters: encs,
	}
}

func validImportEncounter(id string) raidcomp.Encounter {
	return raidcomp.Encounter{
		ID: id, Name: "Test Encounter", Zone: "Kael Drakkel", Status: raidcomp.StatusActive,
		ZoneID:   113,
		Comps:    []raidcomp.CompRow{{Role: "tank", Sub: "defensive", Min: 1, Rec: 2}},
		Reqs:     []string{"a req"},
		Strategy: map[string]string{"notes": "strategy text"},
	}
}

func doReq(t *testing.T, r *chi.Mux, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestRaidPackRoutes_DisabledWhenStoreNil keeps the nil-store 503 guard
// (maintainer review fix) intact for the new pack routes.
func TestRaidPackRoutes_DisabledWhenStoreNil(t *testing.T) {
	h := &raidsHandler{store: nil, roster: nil, pipe: nil}
	r := chi.NewRouter()
	r.Get("/api/raids/export", h.exportPack)
	r.Get("/api/raids/encounters/{id}/export", h.exportEncounter)
	r.Post("/api/raids/import/preview", h.importPreview)
	r.Post("/api/raids/import/commit", h.importCommit)

	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/raids/export"},
		{http.MethodGet, "/api/raids/encounters/aow/export"},
		{http.MethodPost, "/api/raids/import/preview"},
		{http.MethodPost, "/api/raids/import/commit"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status = %d, want 503 when store is nil", c.method, c.path, rec.Code)
		}
	}
}

// TestRaidPack_ExportRoundTrip covers export-all (every encounter, pack
// envelope correct), per-encounter export, and the 404 path.
func TestRaidPack_ExportRoundTrip(t *testing.T) {
	_, r, s := newRaidPackTestRouter(t)
	if err := s.SaveEncounter(&raidcomp.Encounter{
		ID: "vulak", Name: "Vulak'Aerr", Zone: "Kael Drakkel", Status: raidcomp.StatusActive,
		Comps: []raidcomp.CompRow{{Role: "damage", Min: 18, Rec: 22}},
	}); err != nil {
		t.Fatal(err)
	}

	rec := doReq(t, r, http.MethodGet, "/api/raids/export", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d, want 200", rec.Code)
	}
	var pack raidPack
	if err := json.Unmarshal(rec.Body.Bytes(), &pack); err != nil {
		t.Fatal(err)
	}
	if pack.Kind != raidPackKind || pack.Version != raidPackVersion {
		t.Errorf("pack envelope wrong: kind=%q version=%d", pack.Kind, pack.Version)
	}
	if pack.ExportedAt == 0 {
		t.Error("exported_at not stamped")
	}
	if len(pack.Encounters) != 2 { // seed aow + vulak
		t.Fatalf("export-all encounters = %d, want 2", len(pack.Encounters))
	}
	ids := map[string]bool{}
	for _, e := range pack.Encounters {
		ids[e.ID] = true
	}
	if !ids["aow"] || !ids["vulak"] {
		t.Errorf("export-all missing encounters: %v", ids)
	}

	rec = doReq(t, r, http.MethodGet, "/api/raids/encounters/vulak/export", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("single export status = %d, want 200", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pack); err != nil {
		t.Fatal(err)
	}
	if len(pack.Encounters) != 1 || pack.Encounters[0].ID != "vulak" {
		t.Errorf("single export = %+v, want one vulak", pack.Encounters)
	}
	if len(pack.Encounters[0].Comps) != 1 {
		t.Errorf("single export lost comps: %+v", pack.Encounters[0].Comps)
	}

	rec = doReq(t, r, http.MethodGet, "/api/raids/encounters/nope/export", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing export status = %d, want 404", rec.Code)
	}
}

// TestRaidPack_ImportPreviewValidation covers the preview's structural
// rejections and its per-encounter validation annotations.
func TestRaidPack_ImportPreviewValidation(t *testing.T) {
	_, r, _ := newRaidPackTestRouter(t)

	cases := []struct {
		name    string
		body    string
		want    int
		wantSub string // substring of the error message
	}{
		{"wrong kind", `{"kind":"other","version":1,"encounters":[]}`, http.StatusBadRequest, "not a raid composition pack"},
		{"wrong version", `{"kind":"` + raidPackKind + `","version":99,"encounters":[]}`, http.StatusBadRequest, "unsupported pack version"},
		{"no encounters", `{"kind":"` + raidPackKind + `","version":1,"encounters":[]}`, http.StatusBadRequest, "no encounters"},
		{"invalid json", `{not json`, http.StatusBadRequest, "invalid JSON"},
	}
	for _, c := range cases {
		rec := doReq(t, r, http.MethodPost, "/api/raids/import/preview", []byte(c.body))
		if rec.Code != c.want {
			t.Errorf("%s: status = %d, want %d (body: %s)", c.name, rec.Code, c.want, rec.Body.String())
		}
		if c.wantSub != "" && !strings.Contains(rec.Body.String(), c.wantSub) {
			t.Errorf("%s: error %q missing substring %q", c.name, rec.Body.String(), c.wantSub)
		}
	}

	// Per-encounter annotations: conflict, warnings, and each validation error class.
	existing := validImportEncounter("aow") // collides with the seed
	existing.Name = "Renamed AoW"
	noZone := validImportEncounter("no-zone")
	noZone.ZoneID = 0
	badStatus := validImportEncounter("bad-status")
	badStatus.Status = "bogus"
	unknownRole := validImportEncounter("unknown-role")
	unknownRole.Comps = append(unknownRole.Comps, raidcomp.CompRow{Role: "not_a_role", Min: 1, Rec: 1})
	dupComp := validImportEncounter("dup-comp")
	dupComp.Comps = append(dupComp.Comps, raidcomp.CompRow{Role: "tank", Sub: "defensive", Min: 3, Rec: 4})

	recBody := packBody(t, testPack(existing, noZone, badStatus, unknownRole, dupComp))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/raids/import/preview", recBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		PackName   string                  `json:"pack_name"`
		Encounters []raidImportPreviewItem `json:"encounters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.PackName != "test pack" {
		t.Errorf("pack_name = %q", resp.PackName)
	}
	byID := map[string]raidImportPreviewItem{}
	for _, item := range resp.Encounters {
		byID[item.Encounter.ID] = item
	}

	if !byID["aow"].Exists {
		t.Error("existing encounter not flagged exists")
	}
	if len(byID["no-zone"].Warnings) == 0 {
		t.Error("zero zone_id not warned")
	}
	if len(byID["bad-status"].Errors) == 0 {
		t.Error("bad status not flagged as error")
	}
	// Unknown roles are warnings + a machine list, NOT errors: the wizard
	// offers auto-provisioning instead of blocking.
	if len(byID["unknown-role"].Errors) != 0 {
		t.Errorf("unknown role should not be an error anymore: %v", byID["unknown-role"].Errors)
	}
	if len(byID["unknown-role"].MissingRoles) != 1 || byID["unknown-role"].MissingRoles[0] != "not_a_role" {
		t.Errorf("missing_roles wrong: %v", byID["unknown-role"].MissingRoles)
	}
	if !strings.Contains(strings.Join(byID["dup-comp"].Errors, ";"), "duplicate comp row") {
		t.Errorf("duplicate comp row not flagged: %v", byID["dup-comp"].Errors)
	}
	if byID["dup-comp"].Exists {
		t.Error("non-existent encounter flagged exists")
	}
}

// TestRaidPack_ImportCommit covers skip-vs-overwrite conflict resolution,
// the import source stamp, per-encounter partial failure, and the guard
// rails (empty selection, unknown fields, hand-built invalid payloads).
func TestRaidPack_ImportCommit(t *testing.T) {
	_, r, s := newRaidPackTestRouter(t)

	seedAow, err := s.GetEncounter("aow")
	if err != nil {
		t.Fatal(err)
	}
	modified := validImportEncounter("aow")
	modified.Name = "Overwritten AoW"

	req := raidImportCommitRequest{
		PackName: "test pack",
		Encounters: []raidImportCommitItem{
			{Encounter: validImportEncounter("fresh")},       // new → saved, source=import
			{Encounter: modified},                            // exists, default skip → skipped
			{Encounter: modified, Overwrite: true},           // same id again → saved, source preserved
			{Encounter: brokenEncounter(), Overwrite: false}, // validation failure → failed map
		},
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	rec := doReq(t, r, http.MethodPost, "/api/raids/import/commit", b)
	if rec.Code != http.StatusOK {
		t.Fatalf("commit status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp raidImportCommitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Saved) != 2 || !contains(resp.Saved, "fresh") || !contains(resp.Saved, "aow") {
		t.Errorf("saved = %v, want [fresh aow]", resp.Saved)
	}
	// The skip-default aow item resolves to Skipped; the overwrite=true item wins.
	if len(resp.Skipped) != 1 || resp.Skipped[0] != "aow" {
		t.Errorf("skipped = %v, want [aow]", resp.Skipped)
	}
	if len(resp.Failed) != 1 || resp.Failed["broken"] == "" {
		t.Errorf("failed = %v, want only broken", resp.Failed)
	}

	// New encounter: source stamped, children persisted.
	fresh, err := s.GetEncounter("fresh")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Source != raidImportSource {
		t.Errorf("new encounter source = %q, want %q", fresh.Source, raidImportSource)
	}
	if len(fresh.Comps) != 1 || fresh.Strategy["notes"] != "strategy text" || len(fresh.Reqs) != 1 {
		t.Errorf("new encounter children wrong: %+v", fresh)
	}

	// Overwrite: local provenance preserved, content replaced.
	aow, err := s.GetEncounter("aow")
	if err != nil {
		t.Fatal(err)
	}
	if aow.Name != "Overwritten AoW" {
		t.Errorf("overwrite did not apply: name = %q", aow.Name)
	}
	if aow.Source != seedAow.Source {
		t.Errorf("overwrite clobbered source: %q, want %q", aow.Source, seedAow.Source)
	}
	if len(aow.Comps) != 1 || aow.Comps[0].Path() != "tank.defensive" {
		t.Errorf("overwrite did not replace children: %+v", aow.Comps)
	}
}

// TestRaidPack_TaxonomyDTOWithStubs pins the no-null classes contract at the
// HTTP layer: after provisioning a stub, both /taxonomy and /roles must emit
// real arrays (never null) for the stubbed sub-role. Regression for the
// TaxonomyEditor crash ('Cannot read properties of null (reading map)').
func TestRaidPack_TaxonomyDTOWithStubs(t *testing.T) {
	_, r, _ := newRaidPackTestRouter(t)

	rec := doReq(t, r, http.MethodPost, "/api/raids/roles/provision",
		[]byte(`{"paths":["stubrole.sub"]}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("provision status = %d: %s", rec.Code, rec.Body.String())
	}

	for _, path := range []string{"/api/raids/taxonomy", "/api/raids/roles"} {
		res := doReq(t, r, http.MethodGet, path, nil)
		if res.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, res.Code)
		}
		body := res.Body.String()
		if strings.Contains(body, `"classes":null`) {
			t.Errorf("%s emitted classes:null for a stub: %s", path, body)
		}
		if !strings.Contains(body, `"classes":[]`) {
			t.Errorf("%s missing the stub's empty classes array: %s", path, body)
		}
	}
}

func brokenEncounter() raidcomp.Encounter {
	enc := validImportEncounter("broken")
	enc.Comps = append(enc.Comps, raidcomp.CompRow{Role: "not_a_role", Min: 1, Rec: 1})
	return enc
}

// TestRaidPack_RoleProvisioning covers the auto-provision endpoint and the
// import-after-provision flow: an encounter with unknown comp roles fails
// commit until the stubs exist, imports cleanly after, and provisioning is
// idempotent (second call reports present, never overwrites).
func TestRaidPack_RoleProvisioning(t *testing.T) {
	_, r, s := newRaidPackTestRouter(t)

	// Preview flags the missing path without erroring.
	enc := validImportEncounter("prov-enc")
	enc.Comps = []raidcomp.CompRow{
		{Role: "tank", Sub: "defensive", Min: 1, Rec: 2},
		{Role: "brandnew", Sub: "mappings", Min: 3, Rec: 4},
	}
	res := doReq(t, r, http.MethodPost, "/api/raids/import/preview",
		[]byte(mustJSON(t, testPack(enc))))
	var prev struct {
		Encounters []raidImportPreviewItem `json:"encounters"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &prev); err != nil {
		t.Fatal(err)
	}
	if len(prev.Encounters[0].MissingRoles) != 1 || prev.Encounters[0].MissingRoles[0] != "brandnew.mappings" {
		t.Fatalf("preview missing_roles = %v", prev.Encounters[0].MissingRoles)
	}

	// Commit before provisioning: per-item failure with an actionable message.
	commitBody := mustJSON(t, raidImportCommitRequest{
		Encounters: []raidImportCommitItem{{Encounter: enc}},
	})
	rec := doReq(t, r, http.MethodPost, "/api/raids/import/commit", commitBody)
	var failResp raidImportCommitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &failResp); err != nil {
		t.Fatal(err)
	}
	if failResp.Failed["prov-enc"] == "" || !strings.Contains(failResp.Failed["prov-enc"], "brandnew.mappings") {
		t.Errorf("pre-provision commit failure = %q", failResp.Failed["prov-enc"])
	}

	// Provision: creates the stub; already-present paths are reported.
	provBody := mustJSON(t, provisionRolesRequest{
		Paths: []string{"brandnew.mappings", "tank.defensive"},
	})
	rec = doReq(t, r, http.MethodPost, "/api/raids/roles/provision", provBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("provision status = %d: %s", rec.Code, rec.Body.String())
	}
	var prov provisionRolesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &prov); err != nil {
		t.Fatal(err)
	}
	if len(prov.Created) != 1 || prov.Created[0] != "brandnew.mappings" {
		t.Errorf("created = %v, want [brandnew.mappings]", prov.Created)
	}
	if len(prov.Present) != 1 || prov.Present[0] != "tank.defensive" {
		t.Errorf("present = %v, want [tank.defensive]", prov.Present)
	}

	// Stub exists with a label and NO class mappings (checker shows GAP).
	roles, err := s.ListRoles()
	if err != nil {
		t.Fatal(err)
	}
	var stub *raidcomp.Role
	for i := range roles {
		if roles[i].Role == "brandnew" && roles[i].Sub == "mappings" {
			stub = &roles[i]
		}
	}
	if stub == nil {
		t.Fatal("provisioned stub row missing from raid_roles")
	}
	if stub.Label != "Mappings" || len(stub.Classes) != 0 {
		t.Errorf("stub wrong: label=%q classes=%v", stub.Label, stub.Classes)
	}

	// Second provision is idempotent.
	rec = doReq(t, r, http.MethodPost, "/api/raids/roles/provision", provBody)
	var prov2 provisionRolesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &prov2); err != nil {
		t.Fatal(err)
	}
	if len(prov2.Created) != 0 || len(prov2.Present) != 2 {
		t.Errorf("re-provision = created %v present %v", prov2.Created, prov2.Present)
	}

	// The encounter now imports cleanly and the checker runs without panic.
	rec = doReq(t, r, http.MethodPost, "/api/raids/import/commit", commitBody)
	var okResp raidImportCommitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &okResp); err != nil {
		t.Fatal(err)
	}
	if len(okResp.Saved) != 1 || okResp.Saved[0] != "prov-enc" {
		t.Errorf("post-provision commit = %+v", okResp)
	}

	// Guard rails: empty path list and malformed path rejected.
	if rec := doReq(t, r, http.MethodPost, "/api/raids/roles/provision", []byte(`{"paths":[]}`)); rec.Code != http.StatusBadRequest {
		t.Errorf("empty provision status = %d, want 400", rec.Code)
	}
	if rec := doReq(t, r, http.MethodPost, "/api/raids/roles/provision", []byte(`{"paths":[".bad"]}`)); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed path provision status = %d, want 400", rec.Code)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestRaidPack_ImportCommit_GuardRails covers request-shape rejections.
func TestRaidPack_ImportCommit_GuardRails(t *testing.T) {
	_, r, _ := newRaidPackTestRouter(t)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"empty selection", `{"encounters":[]}`, http.StatusBadRequest},
		{"unknown field", `{"encounters":[],"extra":1}`, http.StatusBadRequest},
		{"invalid json", `{`, http.StatusBadRequest},
		// Hand-built payload bypassing the preview must still be re-validated.
		{"invalid encounter", `{"encounters":[{"encounter":{"id":"x","name":"X","zone":"Z","status":"bogus","comps":[]},"overwrite":false}]}`, http.StatusOK},
	}
	for _, c := range cases {
		rec := doReq(t, r, http.MethodPost, "/api/raids/import/commit", []byte(c.body))
		if rec.Code != c.want {
			t.Errorf("%s: status = %d, want %d (body: %s)", c.name, rec.Code, c.want, rec.Body.String())
		}
	}

	// The invalid-encounter case lands in the per-item failed map, not a 4xx.
	rec := doReq(t, r, http.MethodPost, "/api/raids/import/commit",
		[]byte(`{"encounters":[{"encounter":{"id":"x","name":"X","zone":"Z","status":"bogus","comps":[]},"overwrite":false}]}`))
	var resp raidImportCommitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Failed["x"] == "" {
		t.Errorf("invalid hand-built encounter not reported as failed: %+v", resp)
	}
}
