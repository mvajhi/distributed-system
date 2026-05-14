package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	filepb "grpc/file"

	"google.golang.org/grpc"
)

type fileServer struct {
	filepb.UnimplementedFileServiceServer
	basePath string
}

func (s *fileServer) DownloadFile(ctx context.Context, req *filepb.FileRequest) (*filepb.FileResponse, error) {
	filename := req.GetFilename()
	log.Printf("📥 Received download request for file: %s\n", filename)

	filePath := filepath.Join(s.basePath, filename)
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("❌ Failed to read file %s: %v\n", filename, err)
		return nil, fmt.Errorf("could not read file: %v", err)
	}

	contentType := "application/octet-stream"
	if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(filename, ".png") {
		contentType = "image/png"
	}

	log.Printf("🏷️ Content type determined as '%s' for file: %s\n", contentType, filename)
	log.Printf("✅ Successfully prepared file %s for transfer (%d bytes)\n", filename, len(data))

	return &filepb.FileResponse{
		Data:        data,
		ContentType: contentType,
	}, nil
}

func main() {
	log.Println("⚙️ Initializing file storage server...")

	storagePath := "./files"
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		log.Printf("📁 Storage directory missing. Creating new directory at: %s\n", storagePath)
		err := os.Mkdir(storagePath, 0755)
		if err != nil {
			log.Fatalf("🚨 Critical Error: Could not create storage directory: %v\n", err)
		}
	} else {
		log.Printf("📂 Storage directory verified at: %s\n", storagePath)
	}

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("🚨 Failed to bind port: %v\n", err)
	}

	grpcServer := grpc.NewServer()
	filepb.RegisterFileServiceServer(grpcServer, &fileServer{basePath: storagePath})

	log.Println("🚀 File gRPC server is running and listening on port 50052...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("💥 Server encountered a fatal error and stopped: %v\n", err)
	}
}