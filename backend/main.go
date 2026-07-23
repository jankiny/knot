package main

import (
	"log"
	"net/http"

	"knot-backend/api"
	"knot-backend/appdata"
)

func main() {
	dataDirectory, err := appdata.Resolve()
	if err != nil {
		log.Fatalf("Failed to resolve application data directory: %v", err)
	}
	log.Printf("Application data directory resolved from %s.", dataDirectory.Source)

	router := api.SetupRoutes()

	port := "18000"
	log.Printf("Starting Go backend server on port %s...", port)

	err = http.ListenAndServe("127.0.0.1:"+port, router)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
