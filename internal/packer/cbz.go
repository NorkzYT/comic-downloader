package packer

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NorkzYT/comic-downloader/internal/downloader"
)

// CBZArchiver creates a CBZ archive (.cbz file) from a set of images.
type CBZArchiver struct{}

// Archive creates a CBZ file by zipping all provided image files.
// Each file is named with a three-digit counter (e.g. "001.jpg").
func (a *CBZArchiver) Archive(outputDir, filename string, files []*downloader.File, progress func(page, progress int)) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no files to pack")
	}
	fullPath := filepath.Join(outputDir, filename+".cbz")
	outFile, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	for i, file := range files {
		entryName := fmt.Sprintf("%03d.jpg", i)
		writer, err := zipWriter.Create(entryName)
		if err != nil {
			return "", err
		}
		if _, err = writer.Write(file.Data); err != nil {
			return "", err
		}
		// Report progress: increment one page at a time.
		progress(1, 0)
	}
	if err = zipWriter.Close(); err != nil {
		return "", err
	}
	return fullPath, nil
}

// Extension returns the CBZ file extension.
func (a *CBZArchiver) Extension() string {
	return "cbz"
}
