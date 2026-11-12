package scanner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"testofthings.com/result-uploader/internal/api"
)

type metaFileContent struct {
	FileType string `json:"file_type"`
}

// File types given in the file name, e.g. SW@Device.trivy.json, instead of in a 00meta.json
var fileNameFileTypes = []string{"trivy"}

// Returns the file type of a file named <name>.<file type>.json, or "" when it is not one of fileNameFileTypes
func getFileTypeFromFileName(fileName string) string {
	_, extensions, found := strings.Cut(fileName, ".")
	if !found {
		return ""
	}
	for _, fileType := range fileNameFileTypes {
		if extensions == fileType+".json" {
			return fileType
		}
	}
	return ""
}

func getToolName(basePath, dir, fileName string) (string, error) {
	if fileType := getFileTypeFromFileName(fileName); fileType != "" {
		return fileType, nil
	}

	metaPath := filepath.Join(basePath, filepath.FromSlash(dir), "00meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return "", fmt.Errorf("Failed to tool type from meta file %s: %w", metaPath, err)
	}

	var meta metaFileContent
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", fmt.Errorf("Failed to unmarshal meta file %s: %w", metaPath, err)
	}

	return meta.FileType, nil
}

// Scans the given directory and constructs FileItems for uploading
func ScanDirectory(basePath string) (*api.FileListJSON, error) {
	payload := &api.FileListJSON{
		ToolResultFiles: make([]api.ToolResultFile, 0),
		MetaFiles:       make([]api.MetaFile, 0),
	}

	basePath = filepath.Clean(basePath)

	err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		fileName := d.Name()
		ext := strings.ToLower(filepath.Ext(fileName))

		// Get latest metadata modification time
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("Failed to get file info for %s: %w", path, err)
		}
		modTimeUTC := info.ModTime().UTC()

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)
		dir, file := filepath.Split(relPath)
		dir = strings.TrimSuffix(dir, "/")

		// Route to the correct slice
		if strings.EqualFold(file, "00meta.json") {
			payload.MetaFiles = append(payload.MetaFiles, api.MetaFile{
				Name:         file,
				Subdirectory: dir,
				Timestamp:    modTimeUTC,
				ContentType:  "application/json",
			})
		} else {
			contentType := mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			toolName, err := getToolName(basePath, dir, file)
			if errors.Is(err, fs.ErrNotExist) {
				// *.trivy.json files are uploaded without a 00meta.json
				log.Printf("Skipping %s, no 00meta.json in its directory", path)
				return nil
			}
			if err != nil {
				return fmt.Errorf("Failed to get tool name for %s: %w", path, err)
			}

			payload.ToolResultFiles = append(payload.ToolResultFiles, api.ToolResultFile{
				Name:         file,
				Subdirectory: dir,
				Timestamp:    modTimeUTC,
				ContentType:  contentType,
				ToolName:     toolName,
			})
		}

		return nil
	})

	return payload, err
}
