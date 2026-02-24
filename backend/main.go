package main

import (
	"log"
	"net/http"

	"knot-backend/api"
)

func main() {
	router := api.SetupRoutes()

	port := "18000"
	log.Printf("Starting Go backend server on port %s...", port)
	
	err := http.ListenAndServe("0.0.0.0:"+port, router)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
