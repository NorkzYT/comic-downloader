package packer

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NorkzYT/comic-downloader/src/downloader"
)

// ZIPArchiver is functionally similar to CBZArchiver but uses a .zip extension.
type ZIPArchiver struct{}

// Archive creates a ZIP archive (.zip file) with the provided images.
func (a *ZIPArchiver) Archive(outputDir, filename string, files []*downloader.File, progress func(page, progress int)) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no files to pack")
	}
	fullPath := filepath.Join(outputDir, filename+".zip")
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
		progress(1, 0)
	}
	if err = zipWriter.Close(); err != nil {
		return "", err
	}
	return fullPath, nil
}

// Extension returns the ZIP file extension.
func (a *ZIPArchiver) Extension() string {
	return "zip"
}
