package packer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/NorkzYT/comic-downloader/src/downloader"
)

// RAWArchiver simply writes each image file to a folder without archiving.
type RAWArchiver struct{}

// Archive exports each image to a directory named with the given filename plus a "_raw" suffix.
func (a *RAWArchiver) Archive(outputDir, filename string, files []*downloader.File, progress func(page, progress int)) (string, error) {
	folderPath := filepath.Join(outputDir, filename+"_raw")
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return "", err
	}
	for i, file := range files {
		filePath := filepath.Join(folderPath, fmt.Sprintf("%03d.jpg", i))
		if err := os.WriteFile(filePath, file.Data, 0644); err != nil {
			return "", err
		}
		progress(1, 0)
	}
	return folderPath, nil
}

// Extension returns a pseudo extension for raw export.
func (a *RAWArchiver) Extension() string {
	return "raw"
}
