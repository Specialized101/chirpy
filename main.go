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
	mux.Handle("/", fs)

	server := &http.Server{
		Addr:    ADDR + ":" + PORT,
		Handler: mux,
	}

	log.Println("Server running at http://localhost:8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
