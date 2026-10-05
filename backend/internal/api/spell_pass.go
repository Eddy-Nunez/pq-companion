package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jasonsoprovich/pq-companion/backend/internal/character"
)

// spellPassHandler serves a character's "passed" spells — spells excluded from
// the spell checklist's missing list (see character.Store.ListSpellPasses).
type spellPassHandler struct {
	store *character.Store
}

type spellPassResponse struct {
	SpellIDs []int `json:"spell_ids"`
}

// charID parses and validates the {id} path param, writing the error response
// itself; ok is false when the caller should stop.
func (h *spellPassHandler) charID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid character id")
		return 0, false
	}
	if _, ok, err := h.store.Get(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return 0, false
	} else if !ok {
		writeError(w, http.StatusNotFound, "character not found")
		return 0, false
	}
	return id, true
}

// list handles GET /api/characters/{id}/spell-pass.
func (h *spellPassHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := h.charID(w, r)
	if !ok {
		return
	}
	ids, err := h.store.ListSpellPasses(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, spellPassResponse{SpellIDs: ids})
}

// add handles PUT /api/characters/{id}/spell-pass/{spellID}.
func (h *spellPassHandler) add(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.store.AddSpellPass)
}

// remove handles DELETE /api/characters/{id}/spell-pass/{spellID}.
func (h *spellPassHandler) remove(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.store.RemoveSpellPass)
}

func (h *spellPassHandler) mutate(w http.ResponseWriter, r *http.Request, apply func(charID, spellID int) error) {
	id, ok := h.charID(w, r)
	if !ok {
		return
	}
	spellID, err := strconv.Atoi(chi.URLParam(r, "spellID"))
	if err != nil || spellID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid spell id")
		return
	}
	if err := apply(id, spellID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
