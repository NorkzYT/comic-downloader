package downloader

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/NorkzYT/comic-downloader/src/grabber"
	"github.com/NorkzYT/comic-downloader/src/http"
	"github.com/NorkzYT/comic-downloader/src/logger"
)

// File represents a downloaded file.
type File struct {
	Data []byte
	Page uint
}

// ProgressCallback is a function type for progress updates with optional error.
type ProgressCallback func(page, progress int, err error)

// FetchChapter downloads all the pages of a chapter.
func FetchChapter(site grabber.Site, chapter *grabber.Chapter, onprogress ProgressCallback) (files []*File, err error) {
	logger.Debug("downloader.FetchChapter: Starting download for chapter %s", chapter.GetTitle())
	wg := sync.WaitGroup{}
	guard := make(chan struct{}, site.GetMaxConcurrency().Pages)
	errChan := make(chan error, 1)
	done := make(chan bool)
	files = make([]*File, len(chapter.Pages)) // Pre-allocate slice.

	for i, page := range chapter.Pages {
		guard <- struct{}{}
		wg.Add(1)
		go func(page grabber.Page, idx int) {
			defer wg.Done()
			file, err := FetchFile(http.RequestParams{
				URL:     page.URL,
				Referer: site.BaseUrl(),
			}, uint(page.Number))

			pn := int(page.Number)
			cp := pn * 100 / len(chapter.Pages)

			if err != nil {
				select {
				case errChan <- fmt.Errorf("page %d: %w", page.Number, err):
					onprogress(pn, cp, err)
				default:
				}
				<-guard
				return
			}

			files[idx] = file
			onprogress(pn, cp, nil)
			<-guard
		}(page, i)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errChan:
		close(guard)
		logger.Error("downloader.FetchChapter: Error downloading chapter: %v", err)
		return nil, err
	case <-done:
		close(guard)
	}

	sort.SliceStable(files, func(i, j int) bool {
		return files[i].Page < files[j].Page
	})
	logger.Debug("downloader.FetchChapter: Successfully downloaded chapter %s", chapter.GetTitle())
	return
}

// FetchFile gets an online file returning a new *File with its contents.
func FetchFile(params http.RequestParams, page uint) (file *File, err error) {
	var body io.ReadCloser
	maxAttempts := 2

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, err = http.Get(params)
		if err == nil {
			break
		}
		if attempt < maxAttempts {
			time.Sleep(500 * time.Millisecond)
		}
	}
	if err != nil {
		logger.Error("downloader.FetchFile: Error fetching file from URL %s: %v", params.URL, err)
		return nil, err
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		logger.Error("downloader.FetchFile: Error reading data from URL %s: %v", params.URL, err)
		return nil, err
	}

	file = &File{
		Data: data,
		Page: page,
	}
	logger.Debug("downloader.FetchFile: Successfully fetched file for page %d", page)
	return file, nil
}
