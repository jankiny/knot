package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"knot-backend/annualsummary"
	"knot-backend/contextmanifest"

	"github.com/go-chi/chi/v5"
)

type contextManifestHandler struct {
	resolver             *contextmanifest.Resolver
	annualSummaryService *annualsummary.Service
}

func registerContextManifestRoutes(
	router chi.Router,
	resolver *contextmanifest.Resolver,
	annualSummaryService *annualsummary.Service,
) {
	handler := contextManifestHandler{
		resolver:             resolver,
		annualSummaryService: annualSummaryService,
	}
	router.Post("/context/discover", handler.discover)
	router.Get("/context/{id}", handler.get)
	router.Post("/context/{id}/generate", handler.generate)
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

func (handler contextManifestHandler) generate(
	w http.ResponseWriter,
	request *http.Request,
) {
	if handler.annualSummaryService == nil {
		jsonError(
			w,
			http.StatusServiceUnavailable,
			"Annual summary generation is unavailable.",
		)
		return
	}
	var input annualsummary.GenerateRequest
	if err := decodeContextJSON(w, request, &input); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	response, err := handler.annualSummaryService.Generate(
		request.Context(),
		chi.URLParam(request, "id"),
		input,
	)
	if err != nil {
		writeAnnualSummaryError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, response)
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

func writeAnnualSummaryError(w http.ResponseWriter, err error) {
	var validationError *annualsummary.ValidationError
	var manifestValidationError *contextmanifest.ValidationError
	var generationError *annualsummary.GenerationError
	switch {
	case errors.As(err, &validationError),
		errors.As(err, &manifestValidationError):
		jsonError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, contextmanifest.ErrManifestNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, annualsummary.ErrManifestInvalid):
		jsonError(
			w,
			http.StatusConflict,
			"Context manifest is invalid; discover evidence again.",
		)
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		jsonError(
			w,
			http.StatusRequestTimeout,
			"Annual summary generation was cancelled.",
		)
	case errors.As(err, &generationError):
		jsonResponse(w, http.StatusBadGateway, map[string]any{
			"detail":     "Annual summary generation failed.",
			"ai_run_id":  generationError.RunID,
			"error_code": generationError.Code,
		})
	default:
		jsonError(
			w,
			http.StatusInternalServerError,
			"Annual summary generation failed.",
		)
	}
}
