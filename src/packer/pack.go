package packer

import (
	"fmt"

	"github.com/NorkzYT/comic-downloader/src/downloader"
	"github.com/NorkzYT/comic-downloader/src/grabber"
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

// PackBundle packages multiple downloaded chapters into a single archive (bundle).
func PackBundle(outputDir string, s grabber.Site, chapters []*DownloadedChapter, rng string, progress func(page, progress int)) (string, error) {
	title, _ := s.FetchTitle()
	// Concatenate all files from the chapters.
	var files []*downloader.File
	for _, chapter := range chapters {
		files = append(files, chapter.Files...)
	}
	parts := FilenameTemplateParts{
		Series: title,
		Number: rng,
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
	archiver, err := NewArchiver(format)
	if err != nil {
		return "", err
	}
	return pack(outputDir, filename, files, progress, archiver)
}

// pack is a helper that uses the given Archiver to package the files.
func pack(outputDir, filename string, files []*downloader.File, progress func(page, progress int), archiver Archiver) (string, error) {
	return archiver.Archive(outputDir, filename, files, progress)
}
