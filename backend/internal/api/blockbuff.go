package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jasonsoprovich/pq-companion/backend/internal/blockbuff"
	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
)

// blockbuffHandler serves the Blocked Buffs editor. The block list itself
// lives in the game server (#blockbuff / #blockbuffif / #allowbuff), so this
// stores the list the player wants, tracks what the server last confirmed in
// the log, and renders the commands that close the gap.
type blockbuffHandler struct {
	store *blockbuff.Store
	db    *db.DB
}

type blockbuffRowDTO struct {
	blockbuff.Row
	SpellName   string `json:"spell_name"`
	IfSpellName string `json:"if_spell_name,omitempty"`
}

type blockbuffViewDTO struct {
	Character string            `json:"character"`
	SyncedAt  int64             `json:"synced_at"` // unix; 0 = server list never seen
	Rows      []blockbuffRowDTO `json:"rows"`
	Commands  []string          `json:"commands"`
}

func (h *blockbuffHandler) spellName(id int) string {
	if id == 0 || h.db == nil {
		return ""
	}
	s, err := h.db.GetSpell(id)
	if err != nil || s == nil {
		return ""
	}
	return s.Name
}

func (h *blockbuffHandler) view(character string) (*blockbuffViewDTO, error) {
	desired, err := h.store.Desired(character)
	if err != nil {
		return nil, err
	}
	observed, err := h.store.Observed(character)
	if err != nil {
		return nil, err
	}
	syncedAt, err := h.store.SyncedAt(character)
	if err != nil {
		return nil, err
	}
	rows := blockbuff.Reconcile(desired, observed, syncedAt > 0)
	out := &blockbuffViewDTO{
		Character: character,
		SyncedAt:  syncedAt,
		Rows:      make([]blockbuffRowDTO, 0, len(rows)),
		Commands:  blockbuff.Commands(rows),
	}
	for _, r := range rows {
		out.Rows = append(out.Rows, blockbuffRowDTO{
			Row:         r,
			SpellName:   h.spellName(r.SpellID),
			IfSpellName: h.spellName(r.IfSpellID),
		})
	}
	return out, nil
}

// get handles GET /api/blockbuffs?character=<name>.
func (h *blockbuffHandler) get(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "blocked buffs store unavailable")
		return
	}
	character := r.URL.Query().Get("character")
	if character == "" || !isSafeCharacterName(character) {
		writeError(w, http.StatusBadRequest, "valid character is required")
		return
	}
	v, err := h.view(character)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// validateBuff mirrors the server's rule (IsBeneficialSpell && IsBuffSpell):
// only beneficial spells that leave a lasting buff can be blocked.
func (h *blockbuffHandler) validateBuff(id int) error {
	s, err := h.db.GetSpell(id)
	if err != nil || s == nil {
		return fmt.Errorf("spell %d not found", id)
	}
	if s.GoodEffect != 1 || s.BuffDuration <= 0 {
		return fmt.Errorf("%s (%d) is not a beneficial buff, so the server won't block it", s.Name, id)
	}
	return nil
}

// put handles PUT /api/blockbuffs and replaces the character's wanted list.
func (h *blockbuffHandler) put(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "blocked buffs store unavailable")
		return
	}
	var body struct {
		Character string            `json:"character"`
		Entries   []blockbuff.Entry `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Character == "" || !isSafeCharacterName(body.Character) {
		writeError(w, http.StatusBadRequest, "valid character is required")
		return
	}
	for _, e := range body.Entries {
		if err := h.validateBuff(e.SpellID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if e.IfSpellID != 0 {
			if s, err := h.db.GetSpell(e.IfSpellID); err != nil || s == nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("condition spell %d not found", e.IfSpellID))
				return
			}
			if e.IfSpellID == e.SpellID {
				writeError(w, http.StatusBadRequest, "a buff can't be blocked only while it is itself active")
				return
			}
		}
	}
	if err := h.store.SetDesired(body.Character, body.Entries); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	v, err := h.view(body.Character)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// adopt handles POST /api/blockbuffs/adopt and makes the wanted list match
// what the server last reported.
func (h *blockbuffHandler) adopt(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "blocked buffs store unavailable")
		return
	}
	var body struct {
		Character string `json:"character"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Character == "" || !isSafeCharacterName(body.Character) {
		writeError(w, http.StatusBadRequest, "valid character is required")
		return
	}
	if err := h.store.AdoptObserved(body.Character); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	v, err := h.view(body.Character)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}
