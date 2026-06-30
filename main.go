package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"helloworld/internal/config"
	"helloworld/internal/db"
	"helloworld/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

type createUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := "healthy"
		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, apiResponse{
				Code:    503,
				Message: "database unavailable",
			})
			return
		}

		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "ok",
			Data: map[string]string{
				"status":   status,
				"database": "connected",
			},
		})
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "World"
	}
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data: map[string]string{
			"greeting": "Hello, " + name + "!",
		},
	})
}

func listUsersHandler(userStore *store.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := userStore.List(r.Context())
		if err != nil {
			log.Printf("list users: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiResponse{
				Code:    500,
				Message: "internal server error",
			})
			return
		}

		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    users,
		})
	}
}

func createUserHandler(userStore *store.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{
				Code:    400,
				Message: "invalid JSON body",
			})
			return
		}
		if req.Name == "" || req.Email == "" {
			writeJSON(w, http.StatusBadRequest, apiResponse{
				Code:    400,
				Message: "name and email are required",
			})
			return
		}

		user, err := userStore.Create(r.Context(), req.Name, req.Email)
		if err != nil {
			if errors.Is(err, store.ErrEmailExists) {
				writeJSON(w, http.StatusConflict, apiResponse{
					Code:    409,
					Message: "email already exists",
				})
				return
			}
			log.Printf("create user: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiResponse{
				Code:    500,
				Message: "internal server error",
			})
			return
		}

		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "created",
			Data:    user,
		})
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	configPath := flag.String("config", "", "config file path (default: config.yaml or CONFIG_PATH env)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}

	userStore := store.NewUserStore(pool)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler(pool))
	mux.HandleFunc("GET /api/hello", helloHandler)
	mux.HandleFunc("GET /api/users", listUsersHandler(userStore))
	mux.HandleFunc("POST /api/users", createUserHandler(userStore))

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: loggingMiddleware(mux),
	}

	go func() {
		log.Printf("server listening on http://localhost%s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}
