package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"knot-backend/contextmanifest"

	"github.com/go-chi/chi/v5"
)

type contextManifestHandler struct {
	resolver *contextmanifest.Resolver
}

func registerContextManifestRoutes(
	router chi.Router,
	resolver *contextmanifest.Resolver,
) {
	handler := contextManifestHandler{resolver: resolver}
	router.Post("/context/discover", handler.discover)
	router.Get("/context/{id}", handler.get)
}

func (handler contextManifestHandler) discover(
	w http.ResponseWriter,
	request *http.Request,
) {
	if !handler.available(w) {
		return
	}
	var input contextmanifest.DiscoverRequest
	if err := decodeContextJSON(w, request, &input); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	manifest, err := handler.resolver.Discover(request.Context(), input)
	if err != nil {
		writeContextManifestError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, manifest)
}

func (handler contextManifestHandler) get(
	w http.ResponseWriter,
	request *http.Request,
) {
	if !handler.available(w) {
		return
	}
	manifest, err := handler.resolver.Get(
		request.Context(),
		chi.URLParam(request, "id"),
	)
	if err != nil {
		writeContextManifestError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, manifest)
}

func (handler contextManifestHandler) available(w http.ResponseWriter) bool {
	if handler.resolver != nil {
		return true
	}
	jsonError(
		w,
		http.StatusServiceUnavailable,
		"Context discovery is unavailable.",
	)
	return false
}

func decodeContextJSON(
	w http.ResponseWriter,
	request *http.Request,
	target any,
) error {
	request.Body = http.MaxBytesReader(w, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("Invalid JSON request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("Invalid JSON request: only one JSON object is allowed")
	}
	return nil
}

func writeContextManifestError(w http.ResponseWriter, err error) {
	var validationError *contextmanifest.ValidationError
	switch {
	case errors.As(err, &validationError):
		jsonError(w, http.StatusBadRequest, validationError.Error())
	case errors.Is(err, contextmanifest.ErrManifestNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		jsonError(w, http.StatusRequestTimeout, "Context discovery was cancelled.")
	default:
		jsonError(w, http.StatusInternalServerError, "Context discovery failed.")
	}
}
