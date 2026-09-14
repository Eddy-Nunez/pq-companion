package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jasonsoprovich/pq-companion/backend/internal/raidcomp"
	"github.com/jasonsoprovich/pq-companion/backend/internal/zealpipe"
)

// raidsHandler serves the raid knowledge base (encounter CRUD), the live raid
// roster, and the raid composition checker. The role taxonomy is also
// user-editable here; it lives in the raidcomp store.
type raidsHandler struct {
	store  *raidcomp.Store
	roster *raidcomp.Roster
	pipe   *zealpipe.Supervisor
}

// unavailable responds 503 and reports true when the raid store failed to
// open at startup (store is nil in that case) — same non-fatal pattern as
// lockouts/combat-history/emotes.
func (h *raidsHandler) unavailable(w http.ResponseWriter) bool {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "raid knowledge base unavailable")
		return true
	}
	return false
}

// ── Taxonomy ───────────────────────────────────────────────────────────────

type taxonomySubDTO struct {
	Label   string               `json:"label"`
	Classes []raidcomp.ClassCode `json:"classes"`
}

type taxonomyRoleDTO struct {
	Label    string                    `json:"label"`
	Classes  []raidcomp.ClassCode      `json:"classes,omitempty"`
	SubRoles map[string]taxonomySubDTO `json:"sub_roles,omitempty"`
}

type taxonomyResponse struct {
	RoleOrder     []string                      `json:"role_order"`
	Roles         map[string]taxonomyRoleDTO    `json:"roles"`
	ClassNames    map[raidcomp.ClassCode]string `json:"class_names"`
	StrategyOrder []string                      `json:"strategy_order"`
}

func (h *raidsHandler) taxonomy(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	rows, err := h.store.ListRoles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load role taxonomy: "+err.Error())
		return
	}
	roles := make(map[string]taxonomyRoleDTO)
	var roleOrder []string
	for _, row := range rows {
		dto, ok := roles[row.Role]
		if !ok {
			dto = taxonomyRoleDTO{}
			roleOrder = append(roleOrder, row.Role)
		}
		if dto.Label == "" {
			dto.Label = row.Label
		}
		if row.Sub == "" {
			dto.Classes = row.Classes
		} else {
			if dto.SubRoles == nil {
				dto.SubRoles = map[string]taxonomySubDTO{}
			}
			dto.SubRoles[row.Sub] = taxonomySubDTO{Label: row.Label, Classes: row.Classes}
		}
		roles[row.Role] = dto
	}
	writeJSON(w, http.StatusOK, taxonomyResponse{
		RoleOrder:     roleOrder,
		Roles:         roles,
		ClassNames:    raidcomp.ClassNames,
		StrategyOrder: raidcomp.StrategyOrder,
	})
}

func (h *raidsHandler) listRoles(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	rows, err := h.store.ListRoles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load role taxonomy: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": rows})
}

func (h *raidsHandler) saveRole(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	var role raidcomp.Role
	if err := decodeJSON(r, &role); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.SaveRole(role); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true})
}

func (h *raidsHandler) deleteRole(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	role := r.URL.Query().Get("role")
	sub := r.URL.Query().Get("sub")
	if err := h.store.DeleteRole(role, sub); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Roster ─────────────────────────────────────────────────────────────────

// rosterResponse distinguishes three states the UI renders differently:
//   - Zeal pipe not connected   -> "check the Zeal connection"
//   - connected, no raid members -> "Zeal active but not in a raid"
//   - connected, members present  -> live roster
//
// in_raid is true when a MsgRaid roster with members has ever arrived.
type rosterResponse struct {
	ZealConnected bool `json:"zeal_connected"`
	InRaid        bool `json:"in_raid"`
	raidcomp.Snapshot
}

func (h *raidsHandler) getRoster(w http.ResponseWriter, r *http.Request) {
	snap, _ := h.roster.Get()
	connected := h.pipe != nil && h.pipe.Status().State == zealpipe.StateConnected
	if snap.Members == nil {
		snap.Members = []raidcomp.Member{}
	}
	writeJSON(w, http.StatusOK, rosterResponse{
		ZealConnected: connected,
		InRaid:        len(snap.Members) > 0,
		Snapshot:      snap,
	})
}

// ── Encounter CRUD ─────────────────────────────────────────────────────────

func (h *raidsHandler) listEncounters(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	encs, err := h.store.ListEncounters()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load raid encounters: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"encounters": encs})
}

func (h *raidsHandler) getEncounter(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, enc)
}

func (h *raidsHandler) createEncounter(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	var enc raidcomp.Encounter
	if err := decodeJSON(r, &enc); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.SaveEncounter(&enc); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	saved, err := h.store.GetEncounter(enc.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encounter saved but reload failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (h *raidsHandler) updateEncounter(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	id := chi.URLParam(r, "id")
	var enc raidcomp.Encounter
	if err := decodeJSON(r, &enc); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	enc.ID = id
	if err := h.store.SaveEncounter(&enc); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	saved, err := h.store.GetEncounter(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encounter saved but reload failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (h *raidsHandler) deleteEncounter(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteEncounter(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete raid encounter: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Composition check ──────────────────────────────────────────────────────

// checkCompRequest asks for a composition check. roster is optional: when
// omitted (or empty), the live Zeal roster snapshot is used. Members' classes
// accept either a taxonomy code ("sk") or a class name ("Shadow Knight").
type checkCompRequest struct {
	EncounterID string `json:"encounter_id"`
	Roster      []struct {
		Name  string `json:"name"`
		Class string `json:"class"`
	} `json:"roster,omitempty"`
}

func (h *raidsHandler) checkComp(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	var req checkCompRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.EncounterID == "" {
		writeError(w, http.StatusBadRequest, "encounter_id required")
		return
	}
	enc, err := h.store.GetEncounter(req.EncounterID)
	if errors.Is(err, raidcomp.ErrNotFound) {
		writeError(w, http.StatusNotFound, "raid encounter not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load raid encounter: "+err.Error())
		return
	}
	leaves, err := h.store.Leaves()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load role taxonomy: "+err.Error())
		return
	}

	members := make([]raidcomp.RosterMember, 0, len(req.Roster))
	if len(req.Roster) > 0 {
		for _, m := range req.Roster {
			code, _ := raidcomp.CodeForInput(m.Class)
			members = append(members, raidcomp.RosterMember{Name: m.Name, Class: code})
		}
	} else {
		snap, seen := h.roster.Get()
		if seen {
			for _, m := range snap.Members {
				members = append(members, raidcomp.RosterMember{
					Name: m.Name, Class: m.Code, Level: m.Level, Group: m.Group, Rank: m.Rank,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, raidcomp.Check(leaves, enc, members))
}

// decodeJSON decodes a JSON request body with unknown fields rejected and a
// body-size cap (defense against junk payloads on write endpoints).
func decodeJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.New("invalid JSON body: " + err.Error())
	}
	return nil
}
