package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jasonsoprovich/pq-companion/backend/internal/sapitts"
)

// sapiHandler renders built-in Windows voices to WAV so a TTS alert can be
// routed to the user's chosen alert output device (Web Speech itself can only
// play on the Windows default device). See internal/sapitts.
type sapiHandler struct {
	svc *sapitts.Service
}

// POST /api/sapi/synthesize  body: {"text", "voice"?, "rate"?}  ->  {"path"}
// Returns the cached WAV path for text spoken by the named Web Speech voice
// (empty = Windows default). Any failure — non-Windows, voice not found,
// worker trouble — responds 503 so the frontend falls back to Web Speech on
// the default device rather than dropping the alert.
func (h *sapiHandler) synthesize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text  string  `json:"text"`
		Voice string  `json:"voice"`
		Rate  float64 `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	path, err := h.svc.Synthesize(r.Context(), req.Voice, req.Rate, req.Text)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}
