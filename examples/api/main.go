package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type StatusResponse struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

type UserResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/users", handleUsers)

	port := ":8080"
	log.Printf("🔌 API server starting on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := StatusResponse{
		Status:    "ok",
		Message:   "API is running!",
		Timestamp: time.Now(),
		Version:   "1.0.0",
	}
	json.NewEncoder(w).Encode(resp)
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	users := []UserResponse{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
		{ID: 3, Name: "Charlie", Email: "charlie@example.com"},
	}

	json.NewEncoder(w).Encode(users)
}
