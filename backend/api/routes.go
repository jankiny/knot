package api

import (
	"encoding/json"
	"net/http"

	"knot-backend/safepath"
	"knot-backend/sources"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

const (
	workRecordFileName   = "\u5de5\u4f5c\u8bb0\u5f55.md"
	taskSourceDirName    = "00_\u6765\u6e90\u8d44\u6599"
	taskProcessDirName   = "10_\u8fc7\u7a0b\u6587\u4ef6"
	taskOutputDirName    = "20_\u6210\u679c\u8f93\u51fa"
	taskAttachmentDir    = "\u9644\u4ef6"
	defaultSOPTemplateID = "default-task"
)

// Dependencies contains optional persistent services used by API handlers.
type Dependencies struct {
	SourceRegistry *sources.Registry
}

// SetupRoutes initializes routes without persistent services. It remains
// available for focused legacy handler tests.
func SetupRoutes() *chi.Mux {
	return SetupRoutesWithDependencies(Dependencies{})
}

// SetupRoutesWithDependencies initializes the chi router with common
// middleware and configures endpoints.
func SetupRoutesWithDependencies(dependencies Dependencies) *chi.Mux {
	r := chi.NewRouter()
	var safePathResolver *safepath.Resolver
	if dependencies.SourceRegistry != nil {
		safePathResolver = safepath.NewResolver(dependencies.SourceRegistry)
	}

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173", "null"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		registerSourceRootRoutes(r, dependencies.SourceRegistry)

		r.Post("/mail/connect", handleConnectMail)
		r.Get("/mail/list", handleGetMailList)
		r.Get("/mail/{mail_id}/attachments", handleGetAttachments)
		r.Get("/mail/{mail_id}/detail", handleGetMailDetail)

		r.Post("/folder/create", handleCreateFolder)
		r.Post("/folder/create-with-attachments", handleCreateFolderWithAttachments)
		r.Get("/folder/check-hash", handleCheckHash)

		r.Get("/archive/scan", func(w http.ResponseWriter, req *http.Request) {
			handleScanWorkFoldersWithResolver(w, req, safePathResolver)
		})
		r.Post("/archive/move", func(w http.ResponseWriter, req *http.Request) {
			handleArchiveMoveWithResolver(w, req, safePathResolver)
		})
		r.Post("/archive/batch-move", func(w http.ResponseWriter, req *http.Request) {
			handleArchiveBatchMoveWithResolver(w, req, safePathResolver)
		})
		r.Post("/archive/update-work-record", func(w http.ResponseWriter, req *http.Request) {
			handleUpdateWorkRecordWithResolver(w, req, safePathResolver)
		})
		r.Get("/archive/list", func(w http.ResponseWriter, req *http.Request) {
			handleArchiveListWithResolver(w, req, safePathResolver)
		})
		r.Post("/archive/restore", func(w http.ResponseWriter, req *http.Request) {
			handleArchiveRestoreWithResolver(w, req, safePathResolver)
		})
		r.Post("/archive/ai-search", handleArchiveAISearch)

		r.Get("/sop/templates", handleListSOPTemplates)

		r.Post("/report/work/scan", handleScanWorkReport)
		r.Post("/report/work/generate", handleGenerateWorkReport)
		r.Post("/report/daily/generate", handleGenerateDailyReport)
		r.Post("/report/weekly/generate", handleGenerateWeeklyReport)
	})

	return r
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]interface{}{
		"detail": message,
	})
}
