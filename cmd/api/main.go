package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// healthResponse is used for probe-style JSON bodies (livez, readyz).
type healthResponse struct {
	Status string `json:"status"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		resp := healthResponse{Status: "ok"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("livez: encode response: %v", err)
		}
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		resp := healthResponse{Status: "ok"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("livez: encode response: %v", err)
		}
	})

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
