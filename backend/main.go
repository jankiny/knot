package main

import (
	"context"
	"log"
	"net/http"

	"knot-backend/api"
	"knot-backend/appdata"
	"knot-backend/indexer"
	"knot-backend/sources"
	"knot-backend/storage"
)

func main() {
	dataDirectory, err := appdata.Resolve()
	if err != nil {
		log.Fatalf("Failed to resolve application data directory: %v", err)
	}
	log.Printf("Application data directory resolved from %s.", dataDirectory.Source)

	database, err := storage.Open(context.Background(), dataDirectory)
	if err != nil {
		log.Fatalf("Failed to initialize application database: %v", err)
	}
	defer database.Close()
	log.Printf("Application database ready at schema version %d.", storage.CurrentSchemaVersion)

	sourceRegistry := sources.NewRegistry(sources.NewRepository(database))
	router := api.SetupRoutesWithDependencies(api.Dependencies{
		SourceRegistry:  sourceRegistry,
		IndexRepository: indexer.NewRepository(database),
	})

	port := "18000"
	log.Printf("Starting Go backend server on port %s...", port)

	err = http.ListenAndServe("127.0.0.1:"+port, router)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
