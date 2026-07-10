package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"time"

	authpb "grpc/auth"
	filepb "grpc/file"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PageData struct {
	Error       string
	ImageBase64 string
	ContentType string
}

type MemoryEvent struct {
	EventType   string `json:"event_type"`
	Service     string `json:"service"`
	MemoryMB    uint64 `json:"memory_mb"`
	ThresholdMB uint64 `json:"threshold_mb"`
	Timestamp   string `json:"timestamp"`
}

var (
	tmpl           *template.Template
	authServerAddr = "auth-server:50051"
	fileServerAddr = "file-server:50052"
	redisClient    *redis.Client
	memoryHog      [][]byte // Prevents GC from freeing the allocated memory
)

const memoryThresholdMB uint64 = 300

func init() {
	var err error
	tmpl, err = template.ParseFiles("templates/login.html")
	if err != nil {
		log.Fatalf("Fatal: Failed to parse templates: %v\n", err)
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
	log.Println("Templates loaded and Redis client initialized.")
}

func main() {
	go monitorMemory()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/consume-memory", handleConsumeMemory)

	log.Println("Web server listening on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server stopped: %v\n", err)
	}
}

func monitorMemory() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	ctx := context.Background()

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		allocMB := m.Alloc / 1024 / 1024

		if allocMB > memoryThresholdMB {
			log.Printf("Alert: Memory usage reached %d MB\n", allocMB)

			event := MemoryEvent{
				EventType:   "HIGH_MEMORY_USAGE",
				Service:     "web-server",
				MemoryMB:    allocMB,
				ThresholdMB: memoryThresholdMB,
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
			}

			payload, _ := json.Marshal(event)
			if err := redisClient.Publish(ctx, "memory_alerts", payload).Err(); err != nil {
				log.Printf("Error publishing memory alert: %v\n", err)
			}
		}
	}
}

func handleConsumeMemory(w http.ResponseWriter, r *http.Request) {
	mbStr := r.URL.Query().Get("mb")
	mb, err := strconv.Atoi(mbStr)
	if err != nil || mb <= 0 {
		http.Error(w, "Invalid 'mb' parameter", http.StatusBadRequest)
		return
	}

	// Allocate memory and prevent optimization
	chunk := make([]byte, mb*1024*1024)
	for i := range chunk {
		chunk[i] = 1
	}
	memoryHog = append(memoryHog, chunk)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	currentMB := m.Alloc / 1024 / 1024

	log.Printf("Allocated %d MB. Total memory: %d MB\n", mb, currentMB)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Allocated " + mbStr + " MB. Current memory: " + strconv.FormatUint(currentMB, 10) + " MB\n"))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := tmpl.Execute(w, PageData{}); err != nil {
		log.Printf("Template execution error: %v\n", err)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	filename := r.FormValue("filename")

	if filename == "" {
		tmpl.Execute(w, PageData{Error: "Filename is required"})
		return
	}

	authConn, err := grpc.NewClient(authServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Auth service connection failed: %v\n", err)
		tmpl.Execute(w, PageData{Error: "Authentication service unavailable"})
		return
	}
	defer authConn.Close()

	authClient := authpb.NewAuthServiceClient(authConn)
	authRes, err := authClient.Login(context.Background(), &authpb.LoginRequest{
		Username: username,
		Password: password,
	})

	if err != nil || !authRes.GetSuccess() {
		tmpl.Execute(w, PageData{Error: "Invalid credentials"})
		return
	}

	fileConn, err := grpc.NewClient(fileServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("File service connection failed: %v\n", err)
		tmpl.Execute(w, PageData{Error: "File service unavailable"})
		return
	}
	defer fileConn.Close()

	fileClient := filepb.NewFileServiceClient(fileConn)
	fileRes, err := fileClient.DownloadFile(context.Background(), &filepb.FileRequest{
		Filename: filename,
	})

	if err != nil {
		log.Printf("File download failed: %v\n", err)
		tmpl.Execute(w, PageData{Error: "Failed to download image"})
		return
	}

	encodedImage := base64.StdEncoding.EncodeToString(fileRes.GetData())
	err = tmpl.Execute(w, PageData{
		ImageBase64: encodedImage,
		ContentType: fileRes.GetContentType(),
	})
	if err != nil {
		log.Printf("Template execution error: %v\n", err)
	}
}