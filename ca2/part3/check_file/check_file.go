package main

import (
	"context"
	"fmt"
	"os"

	filepb "grpc/file"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_file.go <filename>")
		return
	}
	
	filename := os.Args[1]

	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()
	
	client := filepb.NewFileServiceClient(conn)

	res, err := client.DownloadFile(context.Background(), &filepb.FileRequest{Filename: filename})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	outputFilename := "downloaded_" + filename
	err = os.WriteFile(outputFilename, res.Data, 0644)
	if err == nil {
		fmt.Printf("Success! File saved as %s. Check it now!\n", outputFilename)
	} else {
		fmt.Printf("Failed to save file: %v\n", err)
	}
}