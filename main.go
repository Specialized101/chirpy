package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Specialized101/chirpy/internal/auth"
	"github.com/Specialized101/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

const (
	ADDR = "127.0.0.1"
	PORT = "8080"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	secret         string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
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
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	cfg.fileserverHits.Store(0)
	if err := cfg.db.DeleteUsers(r.Context()); err != nil {
		log.Printf("reset error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	type reqParams struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type returnVals struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	params := reqParams{}
	if err := decoder.Decode(&params); err != nil {
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	if strings.TrimSpace(params.Email) == "" {
		_ = respondWithError(w, http.StatusBadRequest, "email is required")
		return
	}
	if strings.TrimSpace(params.Password) == "" {
		_ = respondWithError(w, http.StatusBadRequest, "password is required")
		return
	}
	hashedPwd, err := auth.HashPassword(params.Password)
	if err != nil {
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		ID:             uuid.New(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		Email:          params.Email,
		HashedPassword: hashedPwd,
	})
	if err != nil {
		log.Printf("failed to create user: %v\n", err)
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	_ = respondWithJSON(w, http.StatusCreated, returnVals{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})
}

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type reqParams struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type returnVals struct {
		ID           uuid.UUID `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
	}
	decoder := json.NewDecoder(r.Body)
	params := reqParams{}
	if err := decoder.Decode(&params); err != nil {
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	user, err := cfg.db.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		_ = respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	if auth.CheckPasswordHash(params.Password, user.HashedPassword) != nil {
		_ = respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	token, err := auth.MakeJWT(user.ID, cfg.secret)
	if err != nil {
		log.Printf("failed to create jwt token: %v\n", err)
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	var refreshToken string
	rt, err := cfg.db.GetRefreshTokenByUserId(r.Context(), user.ID)
	if err != nil && err == sql.ErrNoRows {
		if err == sql.ErrNoRows {
			refreshToken, err = auth.MakeRefreshToken()
			if err != nil {
				log.Printf("failed to create refresh token: %v\n", err)
				_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
				return
			}
			_, err := cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
				Token:     refreshToken,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
				ExpiresAt: time.Now().Add(time.Hour).UTC(),
				RevokedAt: sql.NullTime{},
				UserID:    user.ID,
			})
			if err != nil {
				log.Printf("failed to create refresh token in db: %v\n", err)
				_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
				return
			}
		} else {
			log.Printf("failed to get refresh token from db: %v", err)
			_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}
	} else {
		refreshToken = rt.Token
	}

	_ = respondWithJSON(w, http.StatusOK, returnVals{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken,
	})

}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	type returnVals struct {
		Token string `json:"token"`
	}
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		_ = respondWithError(w, http.StatusUnauthorized, "refresh token is required in the authorization header")
		return
	}
	rt, err := cfg.db.GetRefreshToken(r.Context(), refreshToken)
	fmt.Println(refreshToken)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("failed to get refresh token from db: %v", err)
			_ = respondWithError(w, http.StatusUnauthorized, "refresh token has expired, revoked or does not exist")
		} else {
			log.Printf("failed to get refresh token from db: %v", err)
			_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		}
		return

	}
	if rt.RevokedAt.Valid {
		_ = respondWithError(w, http.StatusUnauthorized, "refresh token has been revoked")
		return
	}
	if rt.ExpiresAt.Before(time.Now().UTC()) {
		_ = respondWithError(w, http.StatusUnauthorized, "refresh token expired")
		return
	}
	accessToken, err := auth.MakeJWT(rt.UserID, cfg.secret)
	if err != nil {
		log.Printf("failed to create access token: %v", err)
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	_ = respondWithJSON(w, http.StatusOK, returnVals{
		Token: accessToken,
	})
}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	rt, err := auth.GetBearerToken(r.Header)
	if err != nil {
		_ = respondWithError(w, http.StatusUnauthorized, "refresh token is required in the authorization header")
		return
	}
	err = cfg.db.RevokeRefreshToken(r.Context(), rt)
	if err != nil {
		log.Printf("failed to revoke refresh token: %v", err)
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, r *http.Request) {
	type reqParams struct {
		Body string `json:"body"`
	}
	type returnVals struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	decoder := json.NewDecoder(r.Body)
	params := reqParams{}
	if err := decoder.Decode(&params); err != nil {
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		_ = respondWithError(w, http.StatusUnauthorized, "token is missing")
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		_ = respondWithError(w, http.StatusUnauthorized, "token is invalid or expired")
		return
	}
	if len(params.Body) > 140 {
		_ = respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}
	if strings.TrimSpace(params.Body) == "" {
		_ = respondWithError(w, http.StatusBadRequest, "Body is required and must not be empty")
		return
	}
	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Body:      censorBadWords(params.Body),
		UserID:    userID,
	})
	if err != nil {
		log.Printf("failed to create chirp: %v\n", err)
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	_ = respondWithJSON(w, http.StatusCreated, returnVals{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}

func (cfg *apiConfig) handlerGetChirps(w http.ResponseWriter, r *http.Request) {
	type returnVals struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	chirps, err := cfg.db.GetChirps(r.Context())
	if err != nil {
		log.Printf("failed to get all chirps from db: %v\n", err)
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	var data []returnVals
	for _, c := range chirps {
		data = append(data, returnVals{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			UserID:    c.UserID,
		})
	}

	_ = respondWithJSON(w, http.StatusOK, data)
}

func (cfg *apiConfig) handlerGetChirpByID(w http.ResponseWriter, r *http.Request) {
	type returnVals struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	chirpID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		_ = respondWithError(w, http.StatusBadRequest, "Chirp id is not valid")
		return
	}
	chirp, err := cfg.db.GetChirpByID(r.Context(), chirpID)
	if err != nil {
		if err == sql.ErrNoRows {
			_ = respondWithError(w, http.StatusNotFound, "the chirp does not exist")
			return
		}
		log.Printf("Failed to get chirp by id: %v\n", err)
		_ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	_ = respondWithJSON(w, http.StatusOK, returnVals{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
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
	apiCfg := &apiConfig{}
	mux := http.NewServeMux()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("database not responding: %v", err)
	}
	apiCfg.platform = os.Getenv("PLATFORM")
	apiCfg.secret = os.Getenv("SECRET_KEY")
	apiCfg.db = database.New(db)

	fs := apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))
	mux.Handle("/app/", http.StripPrefix("/app", fs))

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("GET /api/chirps", apiCfg.handlerGetChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handlerGetChirpByID)
	mux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser)
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerCreateChirp)
	mux.HandleFunc("POST /api/login", apiCfg.handlerLogin)
	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevoke)

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
