package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jasonsoprovich/pq-companion/backend/internal/raidcomp"
)

// Raid composition pack export/import. A "pack" is a portable JSON file
// carrying raid encounters (with their comps/reqs/strategy children) so a
// knowledge base can be shared between users or backed up — the raidcomp
// analogue of trigger packs. Import follows the trigger wizard's two-step
// shape: preview parses + validates WITHOUT persisting, commit installs the
// user-selected subset.
//
// Role taxonomy is deliberately NOT part of a pack: encounters reference
// taxonomy leaf ids that are validated against the importing user's live
// taxonomy, exactly like the ordinary encounter CRUD. Preview surfaces any
// comp rows whose roles don't exist locally as errors so the user isn't
// surprised at commit time.

const (
	raidPackKind     = "pq-companion.raidcomp-pack"
	raidPackVersion  = 1
	raidImportSource = "import"
	// Encounters are a few KB each; even a hundred-encounter pack stays far
	// below this. Caps memory against oversized or hostile uploads.
	maxRaidImportBytes = 2 << 20 // 2 MiB
)

// raidPack is the on-disk export format. Encounters reuse the raidcomp
// Encounter JSON shape verbatim (created_at/updated_at are carried for
// provenance but ignored on import — SaveEncounter owns both).
type raidPack struct {
	Kind        string               `json:"kind"`
	Version     int                  `json:"version"`
	PackName    string               `json:"pack_name"`
	Description string               `json:"description,omitempty"`
	ExportedAt  int64                `json:"exported_at"`
	Encounters  []raidcomp.Encounter `json:"encounters"`
}

// ── Export ─────────────────────────────────────────────────────────────────

// exportPack exports every encounter in the knowledge base as one pack.
func (h *raidsHandler) exportPack(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	encs, err := h.store.ListEncounters()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load raid encounters: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, newRaidPack("Raid Composition Export", encs))
}

// exportEncounter exports a single encounter as a one-item pack (sharing one
// comp without shipping the whole knowledge base).
func (h *raidsHandler) exportEncounter(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	id := chi.URLParam(r, "id")
	enc, err := h.store.GetEncounter(id)
	if errors.Is(err, raidcomp.ErrNotFound) {
		writeError(w, http.StatusNotFound, "raid encounter not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load raid encounter: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, newRaidPack(enc.Name, []raidcomp.Encounter{*enc}))
}

func newRaidPack(name string, encs []raidcomp.Encounter) raidPack {
	if encs == nil {
		encs = []raidcomp.Encounter{}
	}
	return raidPack{
		Kind:        raidPackKind,
		Version:     raidPackVersion,
		PackName:    name,
		Description: "Exported from PQ Companion",
		ExportedAt:  time.Now().Unix(),
		Encounters:  encs,
	}
}

// ── Import preview ─────────────────────────────────────────────────────────

// raidImportPreviewItem is one encounter from an uploaded pack, annotated
// with what commit would do. Errors block that encounter's import; warnings
// don't. Exists marks an id already present locally (the UI then offers
// skip-vs-overwrite per encounter, defaulting to skip). MissingRoles lists
// comp paths absent from the local taxonomy — not an error: the wizard
// offers to auto-provision stub roles so the encounter can import.
type raidImportPreviewItem struct {
	Encounter    raidcomp.Encounter `json:"encounter"`
	Exists       bool               `json:"exists"`
	Errors       []string           `json:"errors,omitempty"`
	Warnings     []string           `json:"warnings,omitempty"`
	MissingRoles []string           `json:"missing_roles,omitempty"`
}

type raidImportPreviewResponse struct {
	PackName   string                  `json:"pack_name"`
	Encounters []raidImportPreviewItem `json:"encounters"`
}

// importPreview parses an uploaded pack and validates every encounter
// against the local store + taxonomy WITHOUT persisting anything.
func (h *raidsHandler) importPreview(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	pack, ok := decodeRaidPack(w, r)
	if !ok {
		return
	}
	leaves, err := h.store.Leaves()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load role taxonomy: "+err.Error())
		return
	}
	items := make([]raidImportPreviewItem, 0, len(pack.Encounters))
	for _, enc := range pack.Encounters {
		item := raidImportPreviewItem{Encounter: enc}
		if _, err := h.store.GetEncounter(enc.ID); err == nil {
			item.Exists = true
		}
		item.Errors = raidImportErrors(enc)
		if missing := missingRolePaths(leaves, enc); len(missing) > 0 {
			item.MissingRoles = missing
		}
		if enc.ZoneID == 0 {
			item.Warnings = append(item.Warnings,
				"zone_id is not set — encounter detection will not match until a zone is assigned")
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, raidImportPreviewResponse{PackName: pack.PackName, Encounters: items})
}

// decodeRaidPack reads and structurally validates an uploaded pack body.
// Returns ok=false with the response already written on any failure.
func decodeRaidPack(w http.ResponseWriter, r *http.Request) (raidPack, bool) {
	var pack raidPack
	r.Body = http.MaxBytesReader(w, r.Body, maxRaidImportBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body (max 2 MiB): "+err.Error())
		return pack, false
	}
	if err := json.Unmarshal(body, &pack); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return pack, false
	}
	if pack.Kind != raidPackKind {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("not a raid composition pack (kind %q, want %q)", pack.Kind, raidPackKind))
		return pack, false
	}
	if pack.Version != raidPackVersion {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported pack version %d (want %d)", pack.Version, raidPackVersion))
		return pack, false
	}
	if len(pack.Encounters) == 0 {
		writeError(w, http.StatusBadRequest, "no encounters found in pack")
		return pack, false
	}
	return pack, true
}

// raidImportErrors validates one imported encounter the same way commit will
// (Encounter.Validate + duplicate comp rows) so the preview never promises
// something commit would reject. Unknown comp roles are deliberately NOT an
// error here — they are surfaced as missing_roles so the wizard can offer
// auto-provisioning; commit re-checks them via SaveEncounter's own taxonomy
// guard. Duplicate comp rows matter beyond cosmetics: SaveEncounter inserts
// them inside its transaction and the (encounter_id, comp_level, role,
// sub_role) PK violation rolls the whole save back — surfaced here as an
// error instead of a surprise.
func raidImportErrors(enc raidcomp.Encounter) []string {
	var errs []string
	if err := enc.Validate(); err != nil {
		errs = append(errs, err.Error())
	}
	seen := map[string]bool{}
	for _, c := range enc.Comps {
		if seen[c.Path()] {
			errs = append(errs, fmt.Sprintf("raidcomp: duplicate comp row %q", c.Path()))
		}
		seen[c.Path()] = true
	}
	return errs
}

// missingRolePaths returns the deduped comp paths of an encounter that are
// absent from the local taxonomy.
func missingRolePaths(leaves []raidcomp.RoleLeaf, enc raidcomp.Encounter) []string {
	allowed := make(map[string]bool, len(leaves))
	for _, l := range leaves {
		allowed[l.Path()] = true
	}
	var missing []string
	seen := map[string]bool{}
	for _, c := range enc.Comps {
		if !allowed[c.Path()] && !seen[c.Path()] {
			missing = append(missing, c.Path())
			seen[c.Path()] = true
		}
	}
	return missing
}

// ── Role provisioning (import convenience) ─────────────────────────────

// provisionRolesRequest asks for stub taxonomy rows for comp paths missing
// from the local taxonomy (raid pack import convenience).
type provisionRolesRequest struct {
	Paths []string `json:"paths"`
}

type provisionRolesResponse struct {
	Created []string `json:"created"`
	Present []string `json:"present"`
}

// provisionRoles creates the missing stub roles. Idempotent — already-present
// paths are reported, never overwritten. Encounters that failed preview with
// missing_roles become importable after this.
func (h *raidsHandler) provisionRoles(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	var req provisionRolesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "no role paths supplied")
		return
	}
	created, present, err := h.store.ProvisionRoles(req.Paths)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if created == nil {
		created = []string{}
	}
	if present == nil {
		present = []string{}
	}
	writeJSON(w, http.StatusOK, provisionRolesResponse{Created: created, Present: present})
}

// ── Import commit ───────────────────────────────────────────────────────────

// raidImportCommitItem is one user-selected encounter from the preview and
// its conflict choice. Overwrite only matters when the id already exists; it
// is ignored for new ids.
type raidImportCommitItem struct {
	Encounter raidcomp.Encounter `json:"encounter"`
	Overwrite bool               `json:"overwrite"`
}

type raidImportCommitRequest struct {
	PackName   string                 `json:"pack_name,omitempty"`
	Encounters []raidImportCommitItem `json:"encounters"`
}

// raidImportCommitResponse reports per-encounter outcomes. Each encounter is
// saved in its own transaction, so one failure never blocks the rest.
type raidImportCommitResponse struct {
	Saved   []string          `json:"saved"`
	Skipped []string          `json:"skipped"`
	Failed  map[string]string `json:"failed,omitempty"`
}

// importCommit installs the selected subset. Everything is re-validated here
// (this endpoint accepts client-submitted JSON directly, so a hand-built
// commit request could otherwise bypass the preview's checks — same defense
// as the trigger import). Existing encounters are skipped unless the item
// opts into overwrite; overwritten encounters keep their local source and
// created_at, and their children are replaced wholesale by SaveEncounter.
func (h *raidsHandler) importCommit(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	var req raidImportCommitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Encounters) == 0 {
		writeError(w, http.StatusBadRequest, "no encounters selected")
		return
	}
	leaves, err := h.store.Leaves()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load role taxonomy: "+err.Error())
		return
	}
	resp := raidImportCommitResponse{Saved: []string{}, Skipped: []string{}}
	for _, item := range req.Encounters {
		enc := item.Encounter
		err := h.commitOne(leaves, enc, item.Overwrite)
		switch {
		case errors.Is(err, errSkipped):
			resp.Skipped = append(resp.Skipped, enc.ID)
		case err != nil:
			if resp.Failed == nil {
				resp.Failed = map[string]string{}
			}
			resp.Failed[enc.ID] = err.Error()
		default:
			resp.Saved = append(resp.Saved, enc.ID)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// commitOne validates and persists one imported encounter. exists/overwrite
// resolution and the import-source stamp live here; the store handles the
// multi-table transaction.
func (h *raidsHandler) commitOne(leaves []raidcomp.RoleLeaf, enc raidcomp.Encounter, overwrite bool) error {
	if errs := raidImportErrors(enc); len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	if missing := missingRolePaths(leaves, enc); len(missing) > 0 {
		return fmt.Errorf("comp role(s) not in taxonomy (provision them first): %s",
			strings.Join(missing, ", "))
	}
	existing, err := h.store.GetEncounter(enc.ID)
	if err != nil && !errors.Is(err, raidcomp.ErrNotFound) {
		return fmt.Errorf("load existing encounter: %w", err)
	}
	if existing != nil {
		if !overwrite {
			return errSkipped
		}
		// Overwrite: keep the local record's provenance rather than the
		// pack's (the importing user's history outranks the file's).
		enc.Source = existing.Source
	} else {
		enc.Source = raidImportSource
	}
	if err := h.store.SaveEncounter(&enc); err != nil {
		return err
	}
	return nil
}

// errSkipped marks a commit item whose id already exists and whose conflict
// choice was "skip". It never reaches the client — importCommit maps it to
// the Skipped list.
var errSkipped = errors.New("skipped: encounter already exists")
