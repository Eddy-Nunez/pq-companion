package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jasonsoprovich/pq-companion/backend/internal/character"
)

type tasksHandler struct {
	store *character.Store
}

type taskRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

type subtaskRequest struct {
	Name      string `json:"name"`
	Completed bool   `json:"completed"`
	// ParentID nests a new subtask under another subtask of the same task.
	// Omitted/null = direct child of the task. Ignored on update.
	ParentID *int `json:"parent_id"`
}

type reorderRequest struct {
	OrderedIDs []int `json:"ordered_ids"`
}

func (h *tasksHandler) list(w http.ResponseWriter, r *http.Request) {
	charID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid character id")
		return
	}
	tasks, err := h.store.ListTasks(charID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []character.Task{}
	}
	writeJSON(w, http.StatusOK, map[string][]character.Task{"tasks": tasks})
}

func (h *tasksHandler) create(w http.ResponseWriter, r *http.Request) {
	charID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid character id")
		return
	}
	var req taskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	t, err := h.store.CreateTask(charID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *tasksHandler) update(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.Atoi(chi.URLParam(r, "taskID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	var req taskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.store.UpdateTask(taskID, req.Name, req.Description, req.Completed); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *tasksHandler) del(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.Atoi(chi.URLParam(r, "taskID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	if err := h.store.DeleteTask(taskID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *tasksHandler) reorder(w http.ResponseWriter, r *http.Request) {
	charID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid character id")
		return
	}
	var req reorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.ReorderTasks(charID, req.OrderedIDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *tasksHandler) createSubtask(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.Atoi(chi.URLParam(r, "taskID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	var req subtaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	sub, err := h.store.CreateSubtask(taskID, req.ParentID, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, character.ErrSubtaskTooDeep), errors.Is(err, character.ErrSubtaskNotFound):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

func (h *tasksHandler) updateSubtask(w http.ResponseWriter, r *http.Request) {
	subID, err := strconv.Atoi(chi.URLParam(r, "subtaskID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid subtask id")
		return
	}
	var req subtaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.store.UpdateSubtask(subID, req.Name, req.Completed); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *tasksHandler) deleteSubtask(w http.ResponseWriter, r *http.Request) {
	subID, err := strconv.Atoi(chi.URLParam(r, "subtaskID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid subtask id")
		return
	}
	if err := h.store.DeleteSubtask(subID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// importTask creates a whole task (with its nested steps) for the character
// from a file produced by the Export button.
func (h *tasksHandler) importTask(w http.ResponseWriter, r *http.Request) {
	charID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid character id")
		return
	}
	var in character.TaskImport
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "not a valid task file: "+err.Error())
		return
	}
	t, err := h.store.ImportTask(charID, in)
	if err != nil {
		if errors.Is(err, character.ErrInvalidTaskImport) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}
