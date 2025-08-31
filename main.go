package main

import (
	"log"
	"net/http"
)

const (
	ADDR = "127.0.0.1"
	PORT = "8080"
)

func main() {
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir("."))
	mux.Handle("/app/", http.StripPrefix("/app", fs))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    ADDR + ":" + PORT,
		Handler: mux,
	}

	log.Println("Server running at http://localhost:8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
