package main

import (
	"context"
	"net"
	"testing"

	authpb "grpc/auth"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestAuthenticate(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to generate hash: %v", err)
	}

	mockUsers := []User{
		{Username: "testuser", Password: string(hash)},
	}

	success := authenticate("testuser", "testpass123", mockUsers)
	if !success {
		t.Errorf("Expected authentication to succeed for valid credentials")
	}

	success = authenticate("testuser", "wrongpass", mockUsers)
	if success {
		t.Errorf("Expected authentication to fail for invalid password")
	}

	success = authenticate("unknown", "testpass123", mockUsers)
	if success {
		t.Errorf("Expected authentication to fail for unknown user")
	}
}

func TestGRPCLogin(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to generate hash: %v", err)
	}

	mockUsers := []User{
		{Username: "admin", Password: string(hash)},
	}

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	authpb.RegisterAuthServiceServer(grpcServer, &authServer{users: mockUsers})

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			panic("Failed to serve: " + err.Error())
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := authpb.NewAuthServiceClient(conn)

	reqValid := &authpb.LoginRequest{
		Username: "admin",
		Password: "secret123",
	}
	resValid, err := client.Login(context.Background(), reqValid)
	if err != nil {
		t.Fatalf("RPC request failed: %v", err)
	}
	if !resValid.Success {
		t.Errorf("Expected success for valid credentials, but got failure")
	}

	reqInvalid := &authpb.LoginRequest{
		Username: "admin",
		Password: "wrongpassword",
	}
	resInvalid, err := client.Login(context.Background(), reqInvalid)
	if err != nil {
		t.Fatalf("RPC request failed: %v", err)
	}
	if resInvalid.Success {
		t.Errorf("Expected failure for invalid credentials, but got success")
	}
}