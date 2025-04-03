// TODO
// Currently not working properly as it does not use Webtoon mode as https://github.com/ciromattia/kcc has.
// This is to cover comics.
// `go-comic-converter` does not have the Webtoon mode functionality as advised here:
// https://github.com/celogeek/go-comic-converter/issues/30

package packer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/NorkzYT/comic-downloader/internal/downloader"
)

// EPUBArchiver converts a chapter into an EPUB file.
// It first creates a temporary CBZ archive (using CBZArchiver from packer/cbz.go)
// then calls the external "go-comic-converter" tool to produce an EPUB.
// Ensure that "go-comic-converter" is installed and available in your PATH.
type EPUBArchiver struct{}

// Archive creates a temporary CBZ archive from the provided files and then
// converts that archive into an EPUB using go-comic-converter.
// The output EPUB is saved at outputDir with the given filename.
func (a *EPUBArchiver) Archive(outputDir, filename string, files []*downloader.File, progress func(page, progress int)) (string, error) {
	// Ensure that the output directory exists.
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if "go-comic-converter" is available in PATH.
	if _, err := exec.LookPath("go-comic-converter"); err != nil {
		return "", fmt.Errorf("failed to convert CBZ to EPUB: go-comic-converter executable not found in $PATH")
	}

	// Create a temporary CBZ file.
	tempFile, err := os.CreateTemp("", "temp_*.cbz")
	if err != nil {
		return "", err
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// Use the CBZArchiver (declared in packer/cbz.go) to generate the temporary CBZ archive.
	cbzArchiver := &CBZArchiver{}
	// We use the directory of the temporary file and strip the ".cbz" extension from its name.
	tempFilename := strings.TrimSuffix(filepath.Base(tempPath), ".cbz")
	_, err = cbzArchiver.Archive(filepath.Dir(tempPath), tempFilename, files, progress)
	if err != nil {
		return "", err
	}

	// Define the output EPUB file path.
	epubPath := filepath.Join(outputDir, filename+".epub")

	// Convert the temporary CBZ to EPUB using go-comic-converter.
	// This command assumes the usage:
	//   go-comic-converter -profile SR -input <tempPath> -output <epubPath>
	cmd := exec.Command("go-comic-converter", "-profile", "SR", "-input", tempPath, "-output", epubPath)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up temporary CBZ file before returning.
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to convert CBZ to EPUB: %s, output: %s", err, string(outputBytes))
	}

	// Clean up the temporary CBZ file.
	os.Remove(tempPath)
	return epubPath, nil
}

// Extension returns the EPUB file extension.
func (a *EPUBArchiver) Extension() string {
	return "epub"
}
