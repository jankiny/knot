package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"knot-backend/indexer"
	"knot-backend/sources"

	"github.com/go-chi/chi/v5"
)

type indexScanHandler struct {
	scanner *indexer.Scanner
}

func registerIndexScanRoutes(router chi.Router, scanner *indexer.Scanner) {
	handler := indexScanHandler{scanner: scanner}
	router.Post("/source-roots/scan", handler.scanAll)
	router.Post("/source-roots/{id}/scan", handler.scanSource)
}

func (handler indexScanHandler) scanSource(
	w http.ResponseWriter,
	request *http.Request,
) {
	if !handler.available(w) {
		return
	}
	options, err := decodeScanOptions(w, request)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := handler.scanner.ScanSource(
		request.Context(),
		chi.URLParam(request, "id"),
		options,
	)
	if err != nil {
		writeIndexScanError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (handler indexScanHandler) scanAll(
	w http.ResponseWriter,
	request *http.Request,
) {
	if !handler.available(w) {
		return
	}
	options, err := decodeScanOptions(w, request)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := handler.scanner.ScanAll(request.Context(), options)
	if err != nil {
		writeIndexScanError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (handler indexScanHandler) available(w http.ResponseWriter) bool {
	if handler.scanner != nil {
		return true
	}
	jsonError(w, http.StatusServiceUnavailable, "Local index is unavailable.")
	return false
}

func decodeScanOptions(
	w http.ResponseWriter,
	request *http.Request,
) (indexer.ScanOptions, error) {
	request.Body = http.MaxBytesReader(w, request.Body, 1<<20)
	var options indexer.ScanOptions
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&options); err != nil {
		if errors.Is(err, io.EOF) {
			return options, nil
		}
		return indexer.ScanOptions{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return indexer.ScanOptions{}, errors.New(
			"only one JSON object is allowed",
		)
	}
	return options, nil
}

func writeIndexScanError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sources.ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		jsonError(w, http.StatusRequestTimeout, "Index scan was cancelled.")
	default:
		jsonError(w, http.StatusInternalServerError, "Local index scan failed.")
	}
}

func readIndexedWorkRecord(filePath string) (indexer.WorkRecordData, error) {
	parsed, err := readWorkRecord(filePath)
	if err != nil {
		return indexer.WorkRecordData{}, err
	}
	return indexer.WorkRecordData{
		RawContent: parsed.RawFile,
		Title:      parsed.Info.Title,
		TaskDate:   parsed.Info.TaskDate,
		Content:    parsed.Info.Content,
		AIAccess:   parsed.Info.AIAccess,
	}, nil
}
