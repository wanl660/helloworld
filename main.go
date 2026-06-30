package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type createUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type store struct {
	mu     sync.RWMutex
	users  []User
	nextID int
}

func newStore() *store {
	return &store{
		users:  []User{},
		nextID: 1,
	}
}

func (s *store) list() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]User, len(s.users))
	copy(result, s.users)
	return result
}

func (s *store) create(name, email string) User {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := User{
		ID:        s.nextID,
		Name:      name,
		Email:     email,
		CreatedAt: time.Now().UTC(),
	}
	s.nextID++
	s.users = append(s.users, user)
	return user
}

func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "ok",
		Data: map[string]string{
			"status": "healthy",
		},
	})
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

func listUsersHandler(s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    s.list(),
		})
	}
}

func createUserHandler(s *store) http.HandlerFunc {
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

		user := s.create(req.Name, req.Email)
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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	s := newStore()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/hello", helloHandler)
	mux.HandleFunc("GET /api/users", listUsersHandler(s))
	mux.HandleFunc("POST /api/users", createUserHandler(s))

	addr := ":" + port
	log.Printf("server listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, loggingMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}
