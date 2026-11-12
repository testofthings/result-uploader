package uploader

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"testofthings.com/result-uploader/internal/api"
)

type uploadItem struct {
	Name         string
	Subdirectory string
	ContentType  string
}

// Upload files concurrently
func UploadFilesConcurrently(ctx context.Context, client *http.Client, basePath string, payload *api.FileListJSON, urls map[string]string, requiredHeaders map[string]string, maxWorkers int) error {
	// Flatten the payload into a single slice for the workers
	var items []uploadItem
	for _, f := range payload.ToolResultFiles {
		items = append(items, uploadItem{Name: f.Name, Subdirectory: f.Subdirectory, ContentType: f.ContentType})
	}
	for _, m := range payload.MetaFiles {
		items = append(items, uploadItem{Name: m.Name, Subdirectory: m.Subdirectory, ContentType: m.ContentType})
	}

	sem := make(chan struct{}, maxWorkers)
	errCh := make(chan error, len(items))
	var wg sync.WaitGroup
	attempted := 0

	for _, item := range items {
		filePath := item.Name
		if item.Subdirectory != "" {
			filePath = item.Subdirectory + "/" + item.Name
		}

		uploadURL, exists := urls[filePath]
		if !exists {
			fmt.Printf("Skipping %s, no URL was provided\n", filePath)
			continue
		}

		attempted++
		wg.Add(1)
		go func(it uploadItem, upURL string) {
			defer wg.Done()

			// Files still waiting for a worker are dropped without logging when interrupted
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			localPath := filepath.Join(basePath, filepath.FromSlash(it.Subdirectory), it.Name)
			if err := uploadSingleFile(ctx, client, localPath, upURL, requiredHeaders, it.ContentType); err != nil {
				log.Printf("Failed to upload %s: %v", localPath, err)
				errCh <- err
			}
		}(item, uploadURL)
	}

	wg.Wait()
	close(errCh)

	// Failed files are logged above and do not stop the other uploads
	if failed := len(errCh); failed > 0 {
		return fmt.Errorf("%d of %d files failed to upload", failed, attempted)
	}
	return nil
}

// Upload a single file
func uploadSingleFile(ctx context.Context, client *http.Client, localPath string, uploadURL string, requiredHeaders map[string]string, contentType string) error {
	log.Printf("Uploading %s\n", localPath)
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, file)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", contentType)
	req.ContentLength = stat.Size()
	for key, value := range requiredHeaders {
		if strings.EqualFold(key, "Content-Type") {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}
	return nil
}
