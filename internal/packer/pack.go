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

// DownloadedChapter represents a downloaded chapter (the chapter info along with its downloaded files).
type DownloadedChapter struct {
	*grabber.Chapter
	Files []*downloader.File
}

// getSiteFormat extracts the archive format from the site's settings.
// It expects the site to implement a GetFormat() string method.
func getSiteFormat(s grabber.Site) (string, error) {
	type formatGetter interface {
		GetFormat() string
	}
	if fg, ok := s.(formatGetter); ok {
		return fg.GetFormat(), nil
	}
	return "", fmt.Errorf("site does not implement GetFormat")
}

// PackSingle packages a single downloaded chapter using the selected archive format.
// It uses the filename template from the Site settings.
func PackSingle(outputDir string, s grabber.Site, chapter *DownloadedChapter, progress func(page, progress int)) (string, error) {
	title, _ := s.FetchTitle()
	parts := NewChapterFileTemplateParts(title, chapter.Chapter)
	filename, err := NewFilenameFromTemplate(s.GetFilenameTemplate(), parts)
	if err != nil {
		return "", fmt.Errorf("- error creating filename for chapter %s: %s", title, err.Error())
	}
	// Retrieve the desired format from settings.
	format, err := getSiteFormat(s)
	if err != nil {
		return "", err
	}
	archiver, err := NewArchiver(format)
	if err != nil {
		return "", err
	}
	return pack(outputDir, filename, chapter.Files, progress, archiver)
}

// PackBundle packages multiple downloaded chapters into a single archive (bundle)
// with each chapter placed in its own folder inside the archive.
func PackBundle(outputDir string, s grabber.Site, chapters []*DownloadedChapter, rng string, progress func(page, progress int)) (string, error) {
	title, _ := s.FetchTitle()
	// Determine appropriate prefix based on the range.
	// For a single chapter, use "Chapter "; for multiple, use "Chapters ".
	var prefix string
	if strings.Contains(rng, "-") || strings.Contains(rng, ",") {
		prefix = "Chapters "
	} else {
		prefix = "Chapter "
	}
	parts := FilenameTemplateParts{
		Series: title,
		Number: prefix + rng,
		Title:  "bundle",
	}
	filename, err := NewFilenameFromTemplate(s.GetFilenameTemplate(), parts)
	if err != nil {
		return "", fmt.Errorf("- error creating bundle filename for %s: %s", title, err.Error())
	}
	format, err := getSiteFormat(s)
	if err != nil {
		return "", err
	}
	return packBundleChapters(outputDir, filename, chapters, progress, format)
}

// packBundleChapters selects the bundling method based on the archive format.
func packBundleChapters(outputDir, filename string, chapters []*DownloadedChapter, progress func(page, progress int), format string) (string, error) {
	switch format {
	case "cbz", "zip":
		return packBundleToZip(outputDir, filename, chapters, progress, format)
	case "raw":
		return packBundleToRaw(outputDir, filename, chapters, progress)
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
func packBundleToZip(outputDir, filename string, chapters []*DownloadedChapter, progress func(page, progress int), format string) (string, error) {
	ext := format // "cbz" or "zip"
	fullPath := filepath.Join(outputDir, filename+"."+ext)
	outFile, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	for _, chapter := range chapters {
		chapNum := int(chapter.Number)
		// Format chapter folder name (e.g., "Chapter 05")
		folderName := fmt.Sprintf("Chapter %02d", chapNum)
		for i, file := range chapter.Files {
			// Create entry path inside the zip archive: e.g., "Chapter 05/001.jpg"
			entryName := fmt.Sprintf("%s/%03d.jpg", folderName, i)
			writer, err := zipWriter.Create(entryName)
			if err != nil {
				return "", err
			}
			if _, err = writer.Write(file.Data); err != nil {
				return "", err
			}
			// Report progress per file added.
			progress(1, 0)
		}
	}
	if err = zipWriter.Close(); err != nil {
		return "", err
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
func packBundleToRaw(outputDir, filename string, chapters []*DownloadedChapter, progress func(page, progress int)) (string, error) {
	bundleFolder := filepath.Join(outputDir, filename+"_bundle")
	if err := os.MkdirAll(bundleFolder, 0755); err != nil {
		return "", err
	}
	for _, chapter := range chapters {
		chapNum := int(chapter.Number)
		chapFolder := filepath.Join(bundleFolder, fmt.Sprintf("Chapter %02d", chapNum))
		if err := os.MkdirAll(chapFolder, 0755); err != nil {
			return "", err
		}
		for i, file := range chapter.Files {
			filePath := filepath.Join(chapFolder, fmt.Sprintf("%03d.jpg", i))
			if err := os.WriteFile(filePath, file.Data, 0644); err != nil {
				return "", err
			}
			progress(1, 0)
		}
	}
	return bundleFolder, nil
}

// pack is a helper that uses the given Archiver to package the files.
func pack(outputDir, filename string, files []*downloader.File, progress func(page, progress int), archiver Archiver) (string, error) {
	return archiver.Archive(outputDir, filename, files, progress)
}
