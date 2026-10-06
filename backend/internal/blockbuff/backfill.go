package blockbuff

import (
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/logparser"
)

// BackfillHandler replays a character's log to recover block replies seen
// before PQC was tracking them. Store writes are gated on log time, so an old
// log can never overwrite data a live session recorded more recently.
type BackfillHandler struct {
	machine   *Machine
	character string
	inserted  int
}

// NewBackfillHandler returns a handler that attributes replies to character.
func NewBackfillHandler(store *Store, names NameCheck, character string) *BackfillHandler {
	return &BackfillHandler{machine: NewMachine(store, names), character: character}
}

// HandleEvent is a no-op; replies are recognised directly from raw lines.
func (h *BackfillHandler) HandleEvent(logparser.LogEvent) {}

// HandleLine feeds one line to the shared state machine.
func (h *BackfillHandler) HandleLine(ts time.Time, msg string) {
	if h.character == "" || msg == "" {
		return
	}
	if h.machine.Feed(h.character, ts, msg) {
		h.inserted++
	}
}

// Finalize commits a listing still buffered at end of log.
func (h *BackfillHandler) Finalize() {
	if h.machine.Flush(h.character) {
		h.inserted++
	}
}

// Inserted returns how many stored updates the replay applied.
func (h *BackfillHandler) Inserted() int { return h.inserted }
