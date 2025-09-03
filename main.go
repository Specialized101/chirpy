package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/Specialized101/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
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

func respondWithJSON(w http.ResponseWriter, statusCode int, payload any) error {
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

func censorBadWords(s string) string {
	words := strings.Split(s, " ")
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	censoredWords := []string{}
	for _, w := range words {
		if slices.Contains(badWords, strings.ToLower(w)) {
			w = "****"
		}
		censoredWords = append(censoredWords, w)
	}
	return strings.Join(censoredWords, " ")
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)
	dbQueries.CreateUser(context.Background(), database.CreateUserParams{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	apiCfg := &apiConfig{}
	mux := http.NewServeMux()

	fs := apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))
	mux.Handle("/app/", http.StripPrefix("/app", fs))

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
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
		if err := respondWithJSON(w, 200, map[string]string{"cleaned_body": censorBadWords(params.Body)}); err != nil {
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
