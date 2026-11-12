package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"testofthings.com/result-uploader/internal/api"
	"testofthings.com/result-uploader/internal/scanner"
	"testofthings.com/result-uploader/internal/uploader"
)

func main() {
	dirPtr := flag.String("dir", "", "Directory to upload")
	statementPtr := flag.String("statement", "", "Statement identifier / upload_tag")
	apiURLPtr := flag.String("api-url", "", "URL to upload to (e.g. http://localhost:3000)")
	apiKeyPtr := flag.String("api-key", "", "API key")
	workersPtr := flag.Int("workers", 4, "Max concurrent uploads")

	flag.Parse()

	if *dirPtr == "" || *statementPtr == "" || *apiURLPtr == "" || *apiKeyPtr == "" {
		log.Fatal("Missing required args. Usage: binary -dir <path> -statement <statement_url> -api-url <url> -api-key <key>")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Tune HTTP transport for our worker concurrency
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = *workersPtr
	transport.MaxIdleConnsPerHost = *workersPtr
	transport.IdleConnTimeout = 90 * time.Second

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Minute, // Applies per HTTP request
	}

	log.Printf("Scanning directory: %s", *dirPtr)
	payload, err := scanner.ScanDirectory(*dirPtr)
	if err != nil {
		log.Fatalf("Error scanning directory: %v", err)
	}

	totalFiles := len(payload.ToolResultFiles) + len(payload.MetaFiles)
	if totalFiles == 0 {
		log.Println("No files found to upload")
		return
	}

	log.Printf("Requesting signed URLs for %d files", totalFiles)
	urls, uploadedAt, requiredHeaders, err := api.GetSignedURLs(ctx, client, *statementPtr, *apiURLPtr, *apiKeyPtr, payload)
	if err != nil {
		log.Fatalf("Error getting signed URLs: %v", err)
	}

	log.Printf("Uploading files with %d workers", *workersPtr)
	uploadErr := uploader.UploadFilesConcurrently(ctx, client, *dirPtr, payload, urls, requiredHeaders, *workersPtr)
	if ctx.Err() != nil {
		log.Fatalf("Upload interrupted: %v", ctx.Err())
	}

	// Notify even if some files failed, so the uploaded ones get processed
	log.Println("Notifying server of upload completion")
	if err := api.NotifyUploadComplete(ctx, client, *statementPtr, *apiURLPtr, *apiKeyPtr, uploadedAt); err != nil {
		log.Fatalf("Error notifying server of upload completion: %v", err)
	}

	if uploadErr != nil {
		log.Fatalf("Upload process completed with errors: %v", uploadErr)
	}
	log.Println("Upload process completed successfully")
}
