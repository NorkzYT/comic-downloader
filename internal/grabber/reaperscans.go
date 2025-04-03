// Package grabber provides implementations to download comics from different websites.
// This file implements support for reaperscans.com using its API.
package grabber

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/NorkzYT/comic-downloader/internal/http"
	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/PuerkitoBio/goquery"
)

// ReaperScans implements the Site interface for reaperscans.com.
type ReaperScans struct {
	*Grabber
}

// ReaperScansChapter represents a single chapter for ReaperScans.
// It embeds the common Chapter struct and adds the chapter URL.
type ReaperScansChapter struct {
	Chapter
	URL string
}

// seriesResponse represents the JSON response from the series endpoint.
type seriesResponse struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	SeriesSlug string `json:"series_slug"`
	// Other fields omitted.
}

// reaperscansFeed represents the JSON response for the chapters feed.
type reaperscansFeed struct {
	Meta struct {
		Total       int `json:"total"`
		PerPage     int `json:"per_page"`
		CurrentPage int `json:"current_page"`
		LastPage    int `json:"last_page"`
	} `json:"meta"`
	Data []struct {
		ID           int     `json:"id"`
		ChapterSlug  string  `json:"chapter_slug"`
		ChapterName  string  `json:"chapter_name"`
		ChapterTitle *string `json:"chapter_title"`
		SeriesID     int     `json:"series_id"`
		Index        string  `json:"index"` // e.g. "51.0"
		Series       struct {
			SeriesSlug string                 `json:"series_slug"`
			ID         int                    `json:"id"`
			Meta       map[string]interface{} `json:"meta"`
		} `json:"series"`
	} `json:"data"`
}

// Test checks if the provided URL belongs to reaperscans.com.
func (r *ReaperScans) Test() (bool, error) {
	logger.Debug("ReaperScans.Test: Checking if URL contains 'reaperscans.com': %s", r.URL)
	if !strings.Contains(r.URL, "reaperscans.com") {
		return false, nil
	}
	return true, nil
}

// FetchTitle derives the title from the URL slug.
// For example, "the-100th-regression-of-the-max-level-player" becomes "The 100th Regression Of The Max Level Player".
func (r *ReaperScans) FetchTitle() (string, error) {
	parts := strings.Split(strings.Trim(r.URL, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("unable to extract series slug from URL")
	}
	titleSlug := parts[len(parts)-1]
	title := strings.Title(strings.ReplaceAll(titleSlug, "-", " "))
	logger.Debug("ReaperScans.FetchTitle: Derived title: %s", title)
	return title, nil
}

// getSeriesID looks up the series ID using the series endpoint.
// For example, calling https://api.reaperscans.com/series/the-100th-regression-of-the-max-level-player
// returns a JSON object with an "id" field.
func (r *ReaperScans) getSeriesID(slug string) (int, error) {
	apiURL := fmt.Sprintf("https://api.reaperscans.com/series/%s", url.QueryEscape(slug))
	logger.Debug("ReaperScans.getSeriesID: Fetching series data from URL: %s", apiURL)
	rbody, err := http.Get(http.RequestParams{
		URL:     apiURL,
		Referer: "https://reaperscans.com/",
	})
	if err != nil {
		logger.Error("ReaperScans.getSeriesID: Error fetching series data: %v", err)
		return 0, err
	}
	defer rbody.Close()

	var series seriesResponse
	if err := json.NewDecoder(rbody).Decode(&series); err != nil {
		logger.Error("ReaperScans.getSeriesID: Error decoding series JSON: %v", err)
		return 0, err
	}
	logger.Debug("ReaperScans.getSeriesID: Found series ID: %d", series.ID)
	return series.ID, nil
}

// FetchChapters uses the ReaperScans API to retrieve a paginated chapter list.
func (r *ReaperScans) FetchChapters() (Filterables, []error) {
	logger.Debug("ReaperScans.FetchChapters: Fetching chapters for URL: %s", r.URL)
	var errs []error

	// Extract the series slug from the URL.
	parts := strings.Split(strings.Trim(r.URL, "/"), "/")
	if len(parts) < 2 {
		return nil, []error{fmt.Errorf("unable to extract series slug from URL")}
	}
	slug := parts[len(parts)-1]

	seriesID, err := r.getSeriesID(slug)
	if err != nil {
		return nil, []error{err}
	}

	chapters := make(Filterables, 0)
	perPage := 100
	page := 1

	for {
		apiURL := fmt.Sprintf("https://api.reaperscans.com/chapters/%d?page=%d&perPage=%d&order=desc", seriesID, page, perPage)
		logger.Debug("ReaperScans.FetchChapters: Fetching chapters with page %d from URI: %s", page, apiURL)
		rbody, err := http.Get(http.RequestParams{
			URL:     apiURL,
			Referer: "https://reaperscans.com/",
		})
		if err != nil {
			logger.Error("ReaperScans.FetchChapters: Error fetching chapters: %v", err)
			errs = append(errs, err)
			break
		}
		var feed reaperscansFeed
		if err = json.NewDecoder(rbody).Decode(&feed); err != nil {
			logger.Error("ReaperScans.FetchChapters: Error decoding JSON: %v", err)
			errs = append(errs, err)
			rbody.Close()
			break
		}
		rbody.Close()

		// If no data returned, break the loop.
		if len(feed.Data) == 0 {
			break
		}

		for _, ch := range feed.Data {
			num, _ := strconv.ParseFloat(ch.Index, 64)
			title := ch.ChapterName
			if ch.ChapterTitle != nil && *ch.ChapterTitle != "" {
				title = *ch.ChapterTitle
			}
			// Construct chapter URL using the base URL, series slug and chapter_slug.
			chURL := fmt.Sprintf("%s/series/%s/%s", r.BaseUrl(), slug, ch.ChapterSlug)
			chapter := &ReaperScansChapter{
				Chapter: Chapter{
					Number:     num,
					Title:      title,
					Language:   "en",
					PagesCount: 0, // To be set when fetching chapter pages
				},
				URL: chURL,
			}
			logger.Debug("ReaperScans.FetchChapters: Added chapter: %s", chapter.GetTitle())
			chapters = append(chapters, chapter)
		}

		// If current page equals or exceeds last_page, exit.
		if page >= feed.Meta.LastPage {
			break
		}
		page++
	}

	logger.Debug("ReaperScans.FetchChapters: Total chapters fetched: %d", len(chapters))
	return chapters, errs
}

// FetchChapter downloads a chapter page and extracts all image URLs as pages.
func (r *ReaperScans) FetchChapter(f Filterable) (*Chapter, error) {
	rsChap, ok := f.(*ReaperScansChapter)
	if !ok {
		return nil, fmt.Errorf("ReaperScans.FetchChapter: invalid chapter type")
	}
	logger.Debug("ReaperScans.FetchChapter: Fetching chapter page from URL: %s", rsChap.URL)
	body, err := http.Get(http.RequestParams{
		URL:     rsChap.URL,
		Referer: r.BaseUrl(),
	})
	if err != nil {
		logger.Error("ReaperScans.FetchChapter: Error fetching chapter URL: %v", err)
		return nil, err
	}
	defer body.Close()

	// Parse the chapter page.
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		logger.Error("ReaperScans.FetchChapter: Error parsing chapter document: %v", err)
		return nil, err
	}

	// Initialize the chapter.
	chapter := &Chapter{
		Title:    rsChap.GetTitle(),
		Number:   rsChap.GetNumber(),
		Language: "en",
	}

	// For each <img> element in the container, extract the src.
	doc.Find("div.container div.flex.flex-col.justify-center.items-center img").Each(func(i int, s *goquery.Selection) {
		src := strings.TrimSpace(s.AttrOr("src", ""))
		if src == "" {
			logger.Info("ReaperScans.FetchChapter: Image at index %d has empty src", i)
			return
		}
		// If the URL is relative, make it absolute.
		if !strings.HasPrefix(src, "http") {
			src = r.BaseUrl() + src
		}
		// Create a page and add it to the chapter.
		page := Page{
			Number: int64(i + 1),
			URL:    src,
		}
		chapter.Pages = append(chapter.Pages, page)
		logger.Debug("ReaperScans.FetchChapter: Added page %d with URL: %s", i+1, src)
	})

	chapter.PagesCount = int64(len(chapter.Pages))
	if chapter.PagesCount == 0 {
		return nil, fmt.Errorf("ReaperScans.FetchChapter: no images found on chapter page")
	}
	logger.Debug("ReaperScans.FetchChapter: Fetched %d pages", chapter.PagesCount)
	return chapter, nil
}

// BaseUrl returns the official base URL for ReaperScans.
func (r *ReaperScans) BaseUrl() string {
	return "https://reaperscans.com"
}

// GetFilenameTemplate returns the filename template from settings.
func (r *ReaperScans) GetFilenameTemplate() string {
	return r.Settings.FilenameTemplate
}

// GetMaxConcurrency returns the maximum concurrency settings.
func (r *ReaperScans) GetMaxConcurrency() MaxConcurrency {
	return r.Settings.MaxConcurrency
}

// GetPreferredLanguage returns the preferred language setting.
func (r *ReaperScans) GetPreferredLanguage() string {
	return r.Settings.Language
}
