package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hpalmer/cookarr/internal/capture"
	"github.com/hpalmer/cookarr/internal/fetch"
	"github.com/hpalmer/cookarr/internal/store"
)

type importRequest struct {
	URL string `json:"url"`
	// HTML is a pointer so "omitted" (fetch the URL) differs from "empty"
	// (a client error).
	HTML *string `json:"html"`
}

func (s *Server) importRecipe(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if !s.decodeJSON(w, r, &req) {
		return
	}
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "url must be an absolute http(s) URL")
		return
	}

	var page []byte
	switch {
	case req.HTML == nil:
		page, err = s.cfg.Fetcher.Get(r.Context(), req.URL)
		if err != nil {
			switch {
			case errors.Is(err, fetch.ErrTooLarge):
				writeError(w, http.StatusRequestEntityTooLarge, "too_large", "page too large")
			case errors.Is(err, fetch.ErrBlocked):
				writeError(w, http.StatusUnprocessableEntity, "blocked", "that address cannot be fetched")
			default:
				s.log.Warn("fetch failed", "url", req.URL, "err", err)
				writeError(w, http.StatusBadGateway, "fetch_failed", "could not fetch the page")
			}
			return
		}
	case *req.HTML == "":
		writeError(w, http.StatusUnprocessableEntity, "invalid", "html must not be empty; omit it to fetch the url")
		return
	default:
		page = []byte(*req.HTML)
	}

	stripped, err := capture.Strip(page)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "could not parse html")
		return
	}
	rec, err := capture.Parse(req.URL, stripped, capture.Options{Extractor: capture.Extractor{}})
	if err != nil {
		if hasRecipeErr(err) {
			writeError(w, http.StatusUnprocessableEntity, "no_recipe", "no recipe found on that page")
		} else {
			s.internalError(w, "parse", err)
		}
		return
	}

	saved, err := s.cfg.Store.CreateRecipe(r.Context(), rec, stripped)
	if err != nil {
		s.internalError(w, "save recipe", err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/recipes/%d", saved.ID))
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) getRecipe(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "recipe not found")
		return
	}
	rec, err := s.cfg.Store.GetRecipe(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "recipe not found")
	case err != nil:
		s.internalError(w, "get recipe", err)
	default:
		writeJSON(w, http.StatusOK, rec)
	}
}

func (s *Server) listRecipes(w http.ResponseWriter, r *http.Request) {
	limit, offset := 50, 0
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			writeError(w, http.StatusBadRequest, "bad_request", "limit must be between 1 and 200")
			return
		}
		limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "bad_request", "offset must be 0 or more")
			return
		}
		offset = n
	}
	items, total, err := s.cfg.Store.ListRecipes(r.Context(), limit, offset)
	if err != nil {
		s.internalError(w, "list recipes", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": limit, "offset": offset,
	})
}

func (s *Server) deleteRecipe(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "recipe not found")
		return
	}
	switch err := s.cfg.Store.DeleteRecipe(r.Context(), id); {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "recipe not found")
	case err != nil:
		s.internalError(w, "delete recipe", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
