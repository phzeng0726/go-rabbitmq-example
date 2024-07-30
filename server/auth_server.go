package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// LoginRequest represents the expected request payload
type LoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// LoginHandler handles the login requests
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var loginReq LoginRequest
	err := json.NewDecoder(r.Body).Decode(&loginReq)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Process login (this is just a placeholder, implement your logic here)
	if loginReq.Account == "your_account" && loginReq.Password == "your_password" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("generate_a_token_for_user"))
	} else {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
	}
}

func main() {
	http.HandleFunc("/api/v1/auth/login", LoginHandler)

	log.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
