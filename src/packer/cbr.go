package packer

import (
	"fmt"

	"github.com/NorkzYT/comic-downloader/src/downloader"
)

// CBRArchiver is a placeholder for future CBR (RAR-based) archiving support.
// Currently, no Go library is available for writing RAR archives.
type CBRArchiver struct{}

// Archive returns an error indicating that CBR support is not yet implemented.
func (a *CBRArchiver) Archive(outputDir, filename string, files []*downloader.File, progress func(page, progress int)) (string, error) {
	return "", fmt.Errorf("CBR archiving is not implemented")
}

// Extension returns the CBR file extension.
func (a *CBRArchiver) Extension() string {
	return "cbr"
}
