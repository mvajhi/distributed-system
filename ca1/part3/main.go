package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

type Response struct {
	Operation string  `json:"operation,omitempty"`
	A         float64 `json:"a,omitempty"`
	B         float64 `json:"b,omitempty"`
	Result    float64 `json:"result,omitempty"`
	Error     string  `json:"error,omitempty"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func computeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{Error: "Method not allowed"})
		return
	}

	op := r.URL.Query().Get("op")
	aStr := r.URL.Query().Get("a")
	bStr := r.URL.Query().Get("b")

	if op == "" || aStr == "" || bStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Error: "Missing parameters"})
		return
	}

	a, errA := strconv.ParseFloat(aStr, 64)
	b, errB := strconv.ParseFloat(bStr, 64)

	if errA != nil || errB != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Error: "Invalid number format for a or b"})
		return
	}

	var result float64

	switch op {
	case "add":
		result = a + b
	case "sub":
		result = a - b
	case "mul":
		result = a * b
	case "div":
		if b == 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{Error: "Division by zero"})
			return
		}
		result = a / b
	case "mod":
		if int(b) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{Error: "Modulo by zero"})
			return
		}
		result = float64(int(a) % int(b))
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Error: "Invalid operation"})
		return
	}

	log.Printf("Successfully computed: %s %f %f = %f", op, a, b, result)

	json.NewEncoder(w).Encode(Response{
		Operation: op,
		A:         a,
		B:         b,
		Result:    result,
	})
}

func main() {
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/compute", computeHandler)

	port := "8080"
	log.Printf("Server is starting and listening on port %s", port)
	
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}