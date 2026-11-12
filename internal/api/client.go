package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ToolResultFile struct {
	Name         string    `json:"name"`
	Subdirectory string    `json:"subdirectory"`
	Timestamp    time.Time `json:"timestamp"`
	ContentType  string    `json:"content_type"`
	ToolName     string    `json:"tool_name"`
}

type MetaFile struct {
	Name         string    `json:"name"`
	Subdirectory string    `json:"subdirectory"`
	Timestamp    time.Time `json:"timestamp"`
	ContentType  string    `json:"content_type"`
}

type FileListJSON struct {
	ToolResultFiles []ToolResultFile `json:"tool_result_files"`
	MetaFiles       []MetaFile       `json:"meta_files"`
}

type UploadURLResponse struct {
	UploadedAt      string            `json:"upload_date"`
	UploadURLs      map[string]string `json:"upload_urls"`
	RequiredHeaders map[string]string `json:"required_headers"`
}

type UploadCompleteRequest struct {
	UploadedAt string `json:"upload_date"`
}

func GetSignedURLs(ctx context.Context, client *http.Client, statement string, apiURL string, apiKey string, payload *FileListJSON) (map[string]string, string, map[string]string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", nil, fmt.Errorf("Failed to marshal payload: %w", err)
	}

	apiURL = fmt.Sprintf("%s/api/result/statement/%s/files", apiURL, statement)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, "", nil, fmt.Errorf("Failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", nil, fmt.Errorf("Failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var result UploadURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", nil, fmt.Errorf("Failed to decode response: %w", err)
	}

	return result.UploadURLs, result.UploadedAt, result.RequiredHeaders, nil
}

func NotifyUploadComplete(ctx context.Context, client *http.Client, statement string, apiURL string, apiKey string, uploadedAt string) error {
	payload := UploadCompleteRequest{UploadedAt: uploadedAt}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Failed to marshal complete payload: %w", err)
	}

	apiURL = fmt.Sprintf("%s/api/result/statement/%s/upload-complete", apiURL, statement)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Failed to create complete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to send complete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}

	return nil
}
