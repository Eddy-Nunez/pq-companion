package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jasonsoprovich/pq-companion/backend/internal/config"
	"github.com/jasonsoprovich/pq-companion/backend/internal/killtracker"
	"github.com/jasonsoprovich/pq-companion/backend/internal/logparser"
)

type killsHandler struct {
	store    *killtracker.Store // nil when user.db failed to open
	counters *killtracker.Engine
	mgr      *config.Manager
	tailer   *logparser.Tailer
}

func (h *killsHandler) scopeCharacter(r *http.Request) string {
	if c := r.URL.Query().Get("character"); c != "" {
		return c
	}
	return h.detectedCharacter()
}

func (h *killsHandler) detectedCharacter() string {
	if h.tailer != nil {
		if c := h.tailer.ActiveCharacter(); c != "" {
			return c
		}
	}
	return h.mgr.Get().Character
}

func (h *killsHandler) requireStore(w http.ResponseWriter) bool {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "kill log unavailable")
		return false
	}
	return true
}

// meta handles GET /api/kills/meta — character tabs plus the distinct zones
// present (for the filter dropdown).
func (h *killsHandler) meta(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	byLower := map[string]string{}
	for _, d := range logparser.DiscoverCharacters(h.mgr.Get().EQPath) {
		byLower[strings.ToLower(d.Name)] = d.Name
	}
	if withKills, err := h.store.Characters(); err == nil {
		for _, c := range withKills {
			if _, ok := byLower[strings.ToLower(c)]; !ok {
				byLower[strings.ToLower(c)] = c
			}
		}
	}
	chars := make([]string, 0, len(byLower))
	for _, n := range byLower {
		chars = append(chars, n)
	}
	sort.Slice(chars, func(i, j int) bool { return strings.ToLower(chars[i]) < strings.ToLower(chars[j]) })

	zones, err := h.store.Zones(h.scopeCharacter(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"characters": chars,
		"zones":      zones,
		"active":     h.detectedCharacter(),
	})
}

// list handles GET /api/kills — the filtered kill log.
func (h *killsHandler) list(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	limit := queryInt(r, "limit", 2000)
	if limit > 10000 {
		limit = 10000
	}
	q := r.URL.Query()
	f := killtracker.Filters{
		Character: h.scopeCharacter(r),
		Search:    q.Get("search"),
		Zone:      q.Get("zone"),
		Scope:     killtracker.Scope(q.Get("scope")),
		From:      int64(queryInt(r, "from", 0)),
		To:        int64(queryInt(r, "to", 0)),
		SortDesc:  q.Get("sort") != "asc",
		Limit:     limit,
		Offset:    queryInt(r, "offset", 0),
	}
	rows, err := h.store.List(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kills": rows})
}

// clear handles POST /api/kills/clear — wipes the scoped character's kills.
func (h *killsHandler) clear(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	n, err := h.store.Clear(h.scopeCharacter(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": n})
}

// ── Counters ────────────────────────────────────────────────────────────────

func (h *killsHandler) counterState(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.counters.State())
}

func (h *killsHandler) presets(w http.ResponseWriter, _ *http.Request) {
	type presetInfo struct {
		killtracker.Preset
		Installed bool `json:"installed"`
	}
	ps := killtracker.Presets()
	out := make([]presetInfo, len(ps))
	for i, p := range ps {
		out[i] = presetInfo{Preset: p, Installed: h.counters.HasPreset(p.Key)}
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": out})
}

func (h *killsHandler) installPreset(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if h.counters.HasPreset(key) {
		writeError(w, http.StatusConflict, "preset already installed")
		return
	}
	if _, err := h.counters.InstallPreset(key); err != nil {
		h.counterError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.counters.State())
}

func (h *killsHandler) decodeDef(w http.ResponseWriter, r *http.Request) (killtracker.CounterDef, bool) {
	var def killtracker.CounterDef
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return def, false
	}
	def, err := killtracker.Validate(def)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return def, false
	}
	return def, true
}

func (h *killsHandler) createCounter(w http.ResponseWriter, r *http.Request) {
	def, ok := h.decodeDef(w, r)
	if !ok {
		return
	}
	if _, err := h.counters.Create(def); err != nil {
		h.counterError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.counters.State())
}

func (h *killsHandler) updateCounter(w http.ResponseWriter, r *http.Request) {
	def, ok := h.decodeDef(w, r)
	if !ok {
		return
	}
	def.ID = chi.URLParam(r, "id")
	if _, err := h.counters.Update(def); err != nil {
		h.counterError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.counters.State())
}

func (h *killsHandler) deleteCounter(w http.ResponseWriter, r *http.Request) {
	if err := h.counters.Delete(chi.URLParam(r, "id")); err != nil {
		h.counterError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.counters.State())
}

// counterActionRequest carries the optional argument for /action calls.
type counterActionRequest struct {
	Action  string `json:"action"` // adjust | set | reset | start_clock | stop_clock | enable | disable | move
	Value   int    `json:"value"`
	Enabled bool   `json:"enabled"`
}

// action handles POST /api/kill-counters/{id}/action — every live-state
// change that isn't a definition edit.
func (h *killsHandler) action(w http.ResponseWriter, r *http.Request) {
	var req counterActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	id := chi.URLParam(r, "id")
	var err error
	switch req.Action {
	case "adjust":
		err = h.counters.Adjust(id, req.Value)
	case "set":
		err = h.counters.SetCount(id, req.Value)
	case "reset":
		err = h.counters.Reset(id)
	case "start_clock":
		err = h.counters.StartClock(id)
	case "stop_clock":
		err = h.counters.StopClock(id)
	case "enable":
		err = h.counters.SetEnabled(id, req.Enabled)
	case "move":
		err = h.counters.Move(id, req.Value)
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}
	if err != nil {
		h.counterError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.counters.State())
}

func (h *killsHandler) counterError(w http.ResponseWriter, err error) {
	if errors.Is(err, killtracker.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}
