package api

import (
	"errors"
	"net/http"

	"github.com/hpalmer/cookarr/internal/store"
	"github.com/hpalmer/cookarr/internal/types"
)

// activePresetKey returns the pref key tracking which preset is chosen for a
// device class (per docs/PLAN.md: "the user's chosen preset" in the layout
// resolution order).
func activePresetKey(deviceClass string) string { return "active_preset:" + deviceClass }

func deviceClassParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	dc := r.URL.Query().Get("device_class")
	if dc == "" {
		dc = string(types.DeviceDesktop)
	}
	if !types.ValidDeviceClass(dc) {
		writeError(w, http.StatusBadRequest, "bad_request", "device_class must be desktop, tablet or phone")
		return "", false
	}
	return dc, true
}

type layoutBody struct {
	Layout map[string]any `json:"layout"`
}

func (s *Server) listLayoutPresets(w http.ResponseWriter, r *http.Request) {
	dc, ok := deviceClassParam(w, r)
	if !ok {
		return
	}
	presets, err := s.cfg.Store.ListLayoutPresets(r.Context(), dc)
	if err != nil {
		s.internalError(w, "list layout presets", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": presets})
}

func (s *Server) saveLayoutPreset(w http.ResponseWriter, r *http.Request) {
	dc, ok := deviceClassParam(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "preset name is required")
		return
	}
	var body layoutBody
	if !s.decodeJSON(w, r, &body) {
		return
	}
	raw, err := marshalLayout(w, body.Layout)
	if err != nil {
		return
	}
	if err := s.cfg.Store.SaveLayoutPreset(r.Context(), name, dc, raw); err != nil {
		s.internalError(w, "save layout preset", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteLayoutPreset(w http.ResponseWriter, r *http.Request) {
	dc, ok := deviceClassParam(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	switch err := s.cfg.Store.DeleteLayoutPreset(r.Context(), name, dc); {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "preset not found")
	case err != nil:
		s.internalError(w, "delete layout preset", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) getActivePreset(w http.ResponseWriter, r *http.Request) {
	dc, ok := deviceClassParam(w, r)
	if !ok {
		return
	}
	name, err := s.cfg.Store.GetPref(r.Context(), activePresetKey(dc))
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]string{"preset": ""})
		return
	}
	if err != nil {
		s.internalError(w, "get active preset", err)
		return
	}
	var preset string
	if err := jsonUnmarshalString(name, &preset); err != nil {
		s.internalError(w, "decode active preset", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"preset": preset})
}

func (s *Server) setActivePreset(w http.ResponseWriter, r *http.Request) {
	dc, ok := deviceClassParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Preset string `json:"preset"`
	}
	if !s.decodeJSON(w, r, &body) {
		return
	}
	raw, err := jsonMarshalString(body.Preset)
	if err != nil {
		s.internalError(w, "encode active preset", err)
		return
	}
	if err := s.cfg.Store.SetPref(r.Context(), activePresetKey(dc), raw); err != nil {
		s.internalError(w, "set active preset", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getRecipeLayout resolves a layout for a recipe + device class in the order
// documented in docs/PLAN.md: per-recipe override, then the saved preset the
// user has chosen, else 404 (the frontend then falls back to a built-in
// default, which the server has no opinion on).
func (s *Server) getRecipeLayout(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "recipe not found")
		return
	}
	dc, ok := deviceClassParam(w, r)
	if !ok {
		return
	}

	override, err := s.cfg.Store.GetRecipeLayout(r.Context(), id, dc)
	switch {
	case err == nil:
		writeRawLayout(w, "override", override)
		return
	case !errors.Is(err, store.ErrNotFound):
		s.internalError(w, "get recipe layout override", err)
		return
	}

	activeName, err := s.cfg.Store.GetPref(r.Context(), activePresetKey(dc))
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.internalError(w, "get active preset", err)
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "no layout saved; use a built-in default")
		return
	}
	var name string
	if err := jsonUnmarshalString(activeName, &name); err != nil || name == "" {
		writeError(w, http.StatusNotFound, "not_found", "no layout saved; use a built-in default")
		return
	}

	preset, err := presetByName(r.Context(), s.cfg.Store, name, dc)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no layout saved; use a built-in default")
		return
	}
	if err != nil {
		s.internalError(w, "get layout preset", err)
		return
	}
	writeRawLayout(w, "preset", preset)
}

func (s *Server) saveRecipeLayout(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "recipe not found")
		return
	}
	dc, ok := deviceClassParam(w, r)
	if !ok {
		return
	}
	var body layoutBody
	if !s.decodeJSON(w, r, &body) {
		return
	}
	raw, err := marshalLayout(w, body.Layout)
	if err != nil {
		return
	}
	if err := s.cfg.Store.SaveRecipeLayout(r.Context(), id, dc, raw); err != nil {
		s.internalError(w, "save recipe layout", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteRecipeLayout(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "recipe not found")
		return
	}
	dc, ok := deviceClassParam(w, r)
	if !ok {
		return
	}
	switch err := s.cfg.Store.DeleteRecipeLayout(r.Context(), id, dc); {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "no override to clear")
	case err != nil:
		s.internalError(w, "delete recipe layout", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
