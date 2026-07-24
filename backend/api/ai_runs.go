package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"knot-backend/annualsummary"

	"github.com/go-chi/chi/v5"
)

type aiRunHandler struct {
	repository *annualsummary.Repository
}

func registerAIRunRoutes(
	router chi.Router,
	repository *annualsummary.Repository,
) {
	handler := aiRunHandler{repository: repository}
	router.Get("/ai-runs/{id}", handler.get)
	router.Get("/ai-runs/{id}/sources", handler.sources)
}

func (handler aiRunHandler) get(
	w http.ResponseWriter,
	request *http.Request,
) {
	if !handler.available(w) {
		return
	}
	run, err := handler.repository.Get(
		request.Context(),
		strings.TrimSpace(chi.URLParam(request, "id")),
	)
	if err != nil {
		writeAIRunError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, run)
}

func (handler aiRunHandler) sources(
	w http.ResponseWriter,
	request *http.Request,
) {
	if !handler.available(w) {
		return
	}
	runID := strings.TrimSpace(chi.URLParam(request, "id"))
	sources, err := handler.repository.ListSources(
		request.Context(),
		runID,
	)
	if err != nil {
		writeAIRunError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"run_id":  runID,
		"sources": sources,
	})
}

func (handler aiRunHandler) available(w http.ResponseWriter) bool {
	if handler.repository != nil {
		return true
	}
	jsonError(w, http.StatusServiceUnavailable, "AI run audit is unavailable.")
	return false
}

func writeAIRunError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, annualsummary.ErrRunNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		jsonError(
			w,
			http.StatusRequestTimeout,
			"AI run audit request was cancelled.",
		)
	default:
		jsonError(
			w,
			http.StatusInternalServerError,
			"AI run audit request failed.",
		)
	}
}
