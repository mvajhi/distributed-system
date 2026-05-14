package main

import (
	"context"
	"encoding/base64"
	"html/template"
	"log"
	"net/http"

	authpb "grpc/auth"
	filepb "grpc/file"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PageData struct {
	Error       string
	ImageBase64 string
	ContentType string
}

var (
	tmpl           *template.Template
	authServerAddr = "auth-server:50051"
	fileServerAddr = "file-server:50052"
)

func init() {
	log.Println("🛠️ Initializing and parsing templates...")
	var err error
	tmpl, err = template.ParseFiles("templates/login.html")
	if err != nil {
		log.Fatalf("🚨 Critical Error: Failed to parse templates: %v\n", err)
	}
	log.Println("✅ Templates loaded successfully")
}

func main() {
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/login", handleLogin)

	log.Println("🚀 Web server is starting and listening on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("💥 Server encountered a fatal error and stopped: %v\n", err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	log.Printf("🌐 Received request on index route: %s %s\n", r.Method, r.URL.Path)

	if r.Method != http.MethodGet {
		log.Printf("⚠️ Invalid HTTP method on index route: %s\n", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := tmpl.Execute(w, PageData{})
	if err != nil {
		log.Printf("❌ Failed to render index template: %v\n", err)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔐 Received login attempt: %s %s\n", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		log.Printf("⚠️ Invalid HTTP method on login route: %s\n", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	filename := r.FormValue("filename")

	log.Printf("📝 Processing login for user: '%s', requested file: '%s'\n", username, filename)

	if filename == "" {
		log.Println("⚠️ Filename is missing in the request payload")
		tmpl.Execute(w, PageData{Error: "Filename is required"})
		return
	}

	log.Printf("🔌 Dialing Authentication Service (%s)...\n", authServerAddr)
	authConn, err := grpc.NewClient(authServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("🚨 Failed to connect to Authentication Service: %v\n", err)
		tmpl.Execute(w, PageData{Error: "Authentication service is down"})
		return
	}
	defer authConn.Close()

	authClient := authpb.NewAuthServiceClient(authConn)
	authRes, err := authClient.Login(context.Background(), &authpb.LoginRequest{
		Username: username,
		Password: password,
	})

	if err != nil || !authRes.GetSuccess() {
		log.Printf("🚫 Authentication failed for user: '%s'\n", username)
		tmpl.Execute(w, PageData{Error: "Invalid username or password"})
		return
	}

	log.Printf("✅ User '%s' authenticated successfully\n", username)

	log.Printf("🔌 Dialing File Service (%s)...\n", fileServerAddr)
	fileConn, err := grpc.NewClient(fileServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("🚨 Failed to connect to File Service: %v\n", err)
		tmpl.Execute(w, PageData{Error: "File service is down"})
		return
	}
	defer fileConn.Close()

	fileClient := filepb.NewFileServiceClient(fileConn)
	log.Printf("📥 Requesting file '%s' from File Service...\n", filename)
	fileRes, err := fileClient.DownloadFile(context.Background(), &filepb.FileRequest{
		Filename: filename,
	})

	if err != nil {
		log.Printf("❌ Error downloading file '%s': %v\n", filename, err)
		tmpl.Execute(w, PageData{Error: "Failed to download image from file service (Check filename)"})
		return
	}

	log.Printf("📦 Successfully retrieved file '%s' (Content-Type: %s, Size: %d bytes)\n", filename, fileRes.GetContentType(), len(fileRes.GetData()))

	encodedImage := base64.StdEncoding.EncodeToString(fileRes.GetData())

	log.Println("🎨 Rendering login template with retrieved image data...")
	err = tmpl.Execute(w, PageData{
		ImageBase64: encodedImage,
		ContentType: fileRes.GetContentType(),
	})
	if err != nil {
		log.Printf("❌ Failed to render login template: %v\n", err)
	}
}