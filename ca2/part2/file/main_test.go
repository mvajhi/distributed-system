package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	filepb "grpc/file"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestDownloadFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFileName := "test_image.jpg"
	testFileContent := []byte("fake image content")
	testFilePath := filepath.Join(tmpDir, testFileName)

	err := os.WriteFile(testFilePath, testFileContent, 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	filepb.RegisterFileServiceServer(grpcServer, &fileServer{basePath: tmpDir})

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

	client := filepb.NewFileServiceClient(conn)

	reqValid := &filepb.FileRequest{Filename: testFileName}
	resValid, err := client.DownloadFile(context.Background(), reqValid)
	
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	if string(resValid.Data) != string(testFileContent) {
		t.Errorf("Expected data mismatch")
	}

	if resValid.ContentType != "image/jpeg" {
		t.Errorf("Expected content type image/jpeg, got %s", resValid.ContentType)
	}

	reqInvalid := &filepb.FileRequest{Filename: "missing_file.png"}
	_, err = client.DownloadFile(context.Background(), reqInvalid)
	if err == nil {
		t.Errorf("Expected error for missing file, got nil")
	}
}