package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"knot-backend/sources"

	"github.com/go-chi/chi/v5"
)

type sourceRootHandler struct {
	registry *sources.Registry
}

func registerSourceRootRoutes(router chi.Router, registry *sources.Registry) {
	handler := sourceRootHandler{registry: registry}

	router.Get("/source-roots", handler.list)
	router.Post("/source-roots", handler.create)
	router.Post("/source-roots/import-legacy", handler.importLegacy)
	router.Get("/source-roots/{id}", handler.get)
	router.Put("/source-roots/{id}", handler.update)
	router.Delete("/source-roots/{id}", handler.delete)
}

func (handler sourceRootHandler) list(w http.ResponseWriter, r *http.Request) {
	if !handler.available(w) {
		return
	}
	roots, err := handler.registry.List(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to list source roots.")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"source_roots": roots})
}

func (handler sourceRootHandler) get(w http.ResponseWriter, r *http.Request) {
	if !handler.available(w) {
		return
	}
	root, err := handler.registry.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeSourceRootError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, root)
}

func (handler sourceRootHandler) create(w http.ResponseWriter, r *http.Request) {
	if !handler.available(w) {
		return
	}
	var input sources.Input
	if err := decodeSourceRootJSON(w, r, &input); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	root, err := handler.registry.Create(r.Context(), input)
	if err != nil {
		writeSourceRootError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, root)
}

func (handler sourceRootHandler) update(w http.ResponseWriter, r *http.Request) {
	if !handler.available(w) {
		return
	}
	var input sources.Input
	if err := decodeSourceRootJSON(w, r, &input); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	root, err := handler.registry.Update(
		r.Context(),
		chi.URLParam(r, "id"),
		input,
	)
	if err != nil {
		writeSourceRootError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, root)
}

func (handler sourceRootHandler) delete(w http.ResponseWriter, r *http.Request) {
	if !handler.available(w) {
		return
	}
	if err := handler.registry.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeSourceRootError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler sourceRootHandler) importLegacy(w http.ResponseWriter, r *http.Request) {
	if !handler.available(w) {
		return
	}
	var input sources.LegacyImportInput
	if err := decodeSourceRootJSON(w, r, &input); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := handler.registry.ImportLegacy(r.Context(), input)
	if err != nil {
		writeSourceRootError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (handler sourceRootHandler) available(w http.ResponseWriter) bool {
	if handler.registry != nil {
		return true
	}
	jsonError(w, http.StatusServiceUnavailable, "Source registry is unavailable.")
	return false
}

func decodeSourceRootJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("Invalid JSON request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("Invalid JSON request: only one JSON object is allowed")
	}
	return nil
}

func writeSourceRootError(w http.ResponseWriter, err error) {
	var validationError *sources.ValidationError
	switch {
	case errors.As(err, &validationError):
		jsonError(w, http.StatusBadRequest, validationError.Error())
	case errors.Is(err, sources.ErrPathConflict):
		jsonError(w, http.StatusConflict, err.Error())
	case errors.Is(err, sources.ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	default:
		jsonError(w, http.StatusInternalServerError, "Source registry operation failed.")
	}
}
