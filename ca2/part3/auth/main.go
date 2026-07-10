package main

import (
    "context"
    "encoding/json"
    "log"
    "net"
    "os"

    authpb "grpc/auth"

    "golang.org/x/crypto/bcrypt"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
)

type User struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type authServer struct {
    authpb.UnimplementedAuthServiceServer
    users []User
}

func authenticate(username, password string, users []User) bool {
    log.Printf("🔍 Attempting authentication for user: %s\n", username)
    
    for _, u := range users {
        if u.Username == username {
            err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
            if err == nil {
                log.Printf("✅ Password match successful for user: %s\n", username)
                return true
            }
            log.Printf("❌ Password mismatch for user: %s\n", username)
            return false
        }
    }
    
    log.Printf("⚠️ User not found: %s\n", username)
    return false
}

func loadUsers(filename string) ([]User, error) {
    log.Printf("📂 Opening file to load users: %s\n", filename)
    
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var users []User
    decoder := json.NewDecoder(file)
    err = decoder.Decode(&users)
    if err != nil {
        return nil, err
    }

    log.Printf("📊 Successfully loaded %d users from database\n", len(users))
    return users, nil
}

func (s *authServer) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
    log.Printf("📥 Processing login request for user: %s\n", req.GetUsername())
    
    success := authenticate(req.GetUsername(), req.GetPassword(), s.users)
    
    msg := "Login failed"
    if success {
        msg = "Login successful"
        log.Printf("🎉 Login completed successfully for user: %s\n", req.GetUsername())
    } else {
        log.Printf("🚫 Login rejected for user: %s\n", req.GetUsername())
    }

    return &authpb.LoginResponse{
        Success: success,
        Message: msg,
    }, nil
}

func main() {
    log.Println("⚙️ Starting authentication server initialization...")
    
    users, err := loadUsers("users.json")
    if err != nil {
        log.Fatalf("🚨 Critical Error loading users: %v\n", err)
    }

    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("🚨 Failed to bind port: %v\n", err)
    }

    grpcServer := grpc.NewServer()
    authpb.RegisterAuthServiceServer(grpcServer, &authServer{users: users})

    reflection.Register(grpcServer)

    log.Println("🚀 gRPC server is active and listening on port 50051...")
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("💥 Server crashed: %v\n", err)
    }
}