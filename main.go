package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

const (
	ADDR = "127.0.0.1"
	PORT = "8080"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(200)
	body := fmt.Sprintf(`
<html>
	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
	</body>
</html>
	`, cfg.fileserverHits.Load())
	w.Write([]byte(body))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	cfg.fileserverHits.Store(0)
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) error {
	response, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(response)
	return nil
}

func respondWithError(w http.ResponseWriter, statusCode int, msg string) error {
	return respondWithJSON(w, statusCode, map[string]string{"error": msg})
}

func respondWithValid(w http.ResponseWriter, statusCode int) error {
	return respondWithJSON(w, statusCode, map[string]bool{"valid": true})
}

func main() {
	apiCfg := &apiConfig{}
	mux := http.NewServeMux()

	fs := apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))
	mux.Handle("/app/", http.StripPrefix("/app", fs))

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		type reqParams struct {
			Body string `json:"body"`
		}
		decoder := json.NewDecoder(r.Body)
		params := reqParams{}
		if err := decoder.Decode(&params); err != nil {
			err = respondWithError(w, 500, "Something went wrong")
			if err != nil {
				log.Printf("failed to marshal response: %v", err)
				w.WriteHeader(500)
				return
			}
		}
		if len(params.Body) > 140 {
			err := respondWithError(w, 400, "Chirp is too long")
			if err != nil {
				log.Printf("failed to marshal response: %v", err)
				w.WriteHeader(500)
			}
			return
		}
		if err := respondWithValid(w, 200); err != nil {
			log.Printf("failed to marshal response: %v", err)
			w.WriteHeader(500)
		}
	})

	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

	server := &http.Server{
		Addr:    ADDR + ":" + PORT,
		Handler: mux,
	}

	log.Println("Server running at http://localhost:8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
