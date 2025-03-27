//go:build unzip
// +build unzip

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// unzip extracts a ZIP archive (including CBZ files) specified by src into a destination folder dest.
func unzip(src, dest string) error {
	// Open the zip archive for reading.
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	// Create the destination directory if it doesn't exist.
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Iterate through each file in the archive.
	for _, f := range r.File {
		// Construct the full file path.
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip vulnerability.
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			// Create the directory if it doesn't exist.
			if err := os.MkdirAll(fpath, f.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", fpath, err)
			}
			continue
		}

		// Create the directory for the file if necessary.
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for file %s: %w", fpath, err)
		}

		// Open the file inside the zip archive.
		inFile, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open file %s in zip: %w", f.Name, err)
		}
		defer inFile.Close()

		// Create the destination file.
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", fpath, err)
		}

		// Copy contents from the zip file to the destination file.
		if _, err := io.Copy(outFile, inFile); err != nil {
			outFile.Close()
			return fmt.Errorf("failed to copy contents to file %s: %w", fpath, err)
		}

		outFile.Close()
	}

	return nil
}

func main() {
	// Path to the CBZ file.
	src := "Player Who Returned 10,000 Years Later 1 - Chapter 1.cbz"
	// Destination directory where the files will be extracted.
	dest := "./unzip_chapter_1"

	if err := unzip(src, dest); err != nil {
		log.Fatalf("Error unzipping file: %v", err)
	}
	fmt.Println("Unzip successful!")
}
