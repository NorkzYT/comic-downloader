package packer

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NorkzYT/comic-downloader/internal/downloader"
	"github.com/NorkzYT/comic-downloader/internal/grabber"
)

// DownloadedChapter represents a downloaded chapter (info + files).
type DownloadedChapter struct {
	*grabber.Chapter
	Files []*downloader.File
}

// getSiteFormat extracts the archive format from the Site settings.
func getSiteFormat(s grabber.Site) (string, error) {
	type formatGetter interface {
		GetFormat() string
	}
	if fg, ok := s.(formatGetter); ok {
		return fg.GetFormat(), nil
	}
	return "", fmt.Errorf("site does not implement GetFormat")
}

// PackSingle packages one chapter. If format=="raw" it creates a folder;
// otherwise it creates a .cbz/.zip via the archiver.
func PackSingle(
	outputDir string,
	s grabber.Site,
	chapter *DownloadedChapter,
	progress func(page, total int),
) (string, error) {
	// 1) Determine the base name (no extension)
	title, _ := s.FetchTitle()
	format, _ := getSiteFormat(s)

	var baseName string
	if format == "raw" {
		baseName = fmt.Sprintf("chapter-%d", int(chapter.Number))
	} else {
		parts := NewChapterFileTemplateParts(title, chapter.Chapter)
		baseName, _ = NewFilenameFromTemplate(s.GetFilenameTemplate(), parts)
	}

	// 2) Pick the archiver interface (used only for zip/cbz)
	archiver, err := NewArchiver(format)
	if err != nil {
		return "", err
	}

	// 3) Resolve duplicates: chapter-1, chapter-1_v1, chapter-1_v2, ...
	target, err := resolveDuplicate(outputDir, baseName, format)
	if err != nil {
		return "", err
	}

	// 4a) RAW mode: write files into a folder named by `target`
	if format == "raw" {
		folder := filepath.Join(outputDir, target)
		if err := os.MkdirAll(folder, 0755); err != nil {
			return "", err
		}
		for i, file := range chapter.Files {
			fname := fmt.Sprintf("%03d.jpg", i+1)
			dest := filepath.Join(folder, fname)
			if err := os.WriteFile(dest, file.Data, 0644); err != nil {
				return "", err
			}
			progress(i+1, len(chapter.Files))
		}
		return folder, nil
	}

	// 4b) ZIP/CBZ mode: hand off to the archiver (it will append .cbz/.zip)
	return archiver.Archive(outputDir, target, chapter.Files, progress)
}

// PackBundle packages multiple chapters into one archive or raw-folder bundle.
func PackBundle(
	outputDir string,
	s grabber.Site,
	chapters []*DownloadedChapter,
	rng string,
	progress func(page, total int),
) (string, error) {
	// build baseName from title + range
	title, _ := s.FetchTitle()
	var prefix string
	if strings.Contains(rng, "-") || strings.Contains(rng, ",") {
		prefix = "Chapters "
	} else {
		prefix = "Chapter "
	}
	parts := FilenameTemplateParts{Series: title, Number: prefix + rng, Title: "bundle"}
	baseName, err := NewFilenameFromTemplate(s.GetFilenameTemplate(), parts)
	if err != nil {
		return "", fmt.Errorf("error creating bundle filename: %w", err)
	}

	format, err := getSiteFormat(s)
	if err != nil {
		return "", err
	}

	// ensure no overwrite on bundle name too
	target, err := resolveDuplicate(outputDir, baseName, format)
	if err != nil {
		return "", err
	}

	switch format {
	case "cbz", "zip":
		return packBundleToZip(outputDir, target, chapters, progress, format)
	case "raw":
		return packBundleToRaw(outputDir, target, chapters, progress)
	default:
		return "", fmt.Errorf("unsupported bundle format: %s", format)
	}
}

// packBundleToZip creates a CBZ or ZIP archive where each chapter is placed in its own folder.
// For example, the archive structure will be:
//
//	Chapter 01/
//	    001.jpg
//	    002.jpg
//	    ...
//	Chapter 02/
//	    001.jpg
//	    002.jpg
//	    ...
func packBundleToZip(
	outputDir, filename string,
	chapters []*DownloadedChapter,
	progress func(page, total int),
	format string,
) (string, error) {
	ext := format // "cbz" or "zip"
	fullPath := filepath.Join(outputDir, filename+"."+ext)
	outFile, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	for _, chapter := range chapters {
		folderName := fmt.Sprintf("Chapter %02d", int(chapter.Number))
		for i, file := range chapter.Files {
			entry := fmt.Sprintf("%s/%03d.jpg", folderName, i+1)
			w, err := zipWriter.Create(entry)
			if err != nil {
				return "", err
			}
			if _, err := w.Write(file.Data); err != nil {
				return "", err
			}
			progress(i+1, len(chapter.Files))
		}
	}

	return fullPath, nil
}

// packBundleToRaw creates a directory structure for raw output where each chapter gets its own subfolder.
// The resulting folder will contain subfolders like:
//
//	Chapter 01/
//	    001.jpg
//	    002.jpg
//	Chapter 02/
//	    001.jpg
//	    002.jpg
func packBundleToRaw(
	outputDir, filename string,
	chapters []*DownloadedChapter,
	progress func(page, total int),
) (string, error) {
	bundleFolder := filepath.Join(outputDir, filename+"_bundle")
	if err := os.MkdirAll(bundleFolder, 0755); err != nil {
		return "", err
	}

	for _, chapter := range chapters {
		chapFolder := filepath.Join(bundleFolder, fmt.Sprintf("Chapter %02d", int(chapter.Number)))
		if err := os.MkdirAll(chapFolder, 0755); err != nil {
			return "", err
		}
		for i, file := range chapter.Files {
			filePath := filepath.Join(chapFolder, fmt.Sprintf("%03d.jpg", i+1))
			if err := os.WriteFile(filePath, file.Data, 0644); err != nil {
				return "", err
			}
			progress(i+1, len(chapter.Files))
		}
	}

	return bundleFolder, nil
}

// resolveDuplicate checks for an existing file/folder and appends "_vN".
func resolveDuplicate(dir, baseName, format string) (string, error) {
	// extension only for zip/cbz; raw has no ext here
	ext := ""
	if format != "raw" {
		ext = "." + format
	}

	// first try the plain name
	candidate := filepath.Join(dir, baseName+ext)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return baseName, nil
	}

	// else find the next free suffix
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s_v%d%s", baseName, i, ext)
		tryPath := filepath.Join(dir, name)
		if _, err := os.Stat(tryPath); os.IsNotExist(err) {
			// return baseName_vN (no ext) so archiver adds it if needed
			if format == "raw" {
				return fmt.Sprintf("%s_v%d", baseName, i), nil
			}
			return fmt.Sprintf("%s_v%d", baseName, i), nil
		}
	}
}
