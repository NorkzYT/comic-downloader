package packer

import (
	"fmt"

	"github.com/NorkzYT/comic-downloader/src/downloader"
)

// Archiver defines the interface for packaging downloaded files.
type Archiver interface {
	// Archive packages the given files into an archive (or folder) at outputDir
	// using the provided base filename. It reports progress via the callback.
	// It returns the full path to the created archive.
	Archive(outputDir, filename string, files []*downloader.File, progress func(page, progress int)) (string, error)
	// Extension returns the file extension (without the dot) for this archive type.
	Extension() string
}

// NewArchiver returns an Archiver implementation based on the provided format.
// Supported formats are: "cbz", "zip", and "raw".
func NewArchiver(format string) (Archiver, error) {
	switch format {
	case "cbz":
		return &CBZArchiver{}, nil
	case "zip":
		return &ZIPArchiver{}, nil
	case "raw":
		return &RAWArchiver{}, nil
	// case "epub":
	// 	return &EPUBArchiver{}, nil
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", format)
	}
}
