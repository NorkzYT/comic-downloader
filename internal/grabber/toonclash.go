package grabber

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/NorkzYT/comic-downloader/internal/browserless"
	"github.com/NorkzYT/comic-downloader/internal/logger"
)

// ToonClash implements the Site interface for toonclash.com using chromedp.
type ToonClash struct {
	*Grabber
}

// ToonClashChapter represents a chapter from toonclash.com.
type ToonClashChapter struct {
	Chapter
	URL string
}

// Test verifies if the URL is from toonclash.com.
func (t *ToonClash) Test() (bool, error) {
	logger.Debug("ToonClash.Test: Checking if URL contains 'toonclash.com': %s", t.URL)
	return strings.Contains(t.URL, "toonclash.com"), nil
}

// UsesBrowser indicates that this grabber uses a browser context.
func (t *ToonClash) UsesBrowser() bool {
	return true
}

// FetchTitle extracts the comic title by running JavaScript in the page context.
func (t *ToonClash) FetchTitle() (string, error) {
	var title string
	jsTitle := `
    (function(){
      const el = document.querySelector("div.post-title h1");
      return el ? el.innerText.trim() : "";
    })();`
	logger.Debug("ToonClash.FetchTitle: Running JS for title extraction on %s", t.URL)
	err := browserless.RunJS(t.URL, "div.post-title", 0, jsTitle, &title)
	if err != nil {
		logger.Error("ToonClash.FetchTitle: Error executing JS: %v", err)
		return "", fmt.Errorf("error fetching title: %w", err)
	}
	return strings.TrimSpace(title), nil
}

// FetchChapters extracts the list of chapters from the series page.
func (t *ToonClash) FetchChapters() (Filterables, []error) {
	var chaptersJSON string
	jsChapters := `
    (function(){
      const items = document.querySelectorAll("div.listing-chapters_wrap ul.main.version-chap.no-volumn.active li.wp-manga-chapter");
      const chapters = [];
      items.forEach(item => {
        const a = item.querySelector("a");
        const raw = a.textContent.trim();
        const match = raw.match(/Chapter\s*(\d+(?:\.\d+)?)/i);
        const title = match ? match[0] : raw;
        const num = match ? parseFloat(match[1]) : 0;
        chapters.push({title, number: num, url: a.href});
      });
      return JSON.stringify(chapters);
    })();`
	logger.Debug("ToonClash.FetchChapters: Executing JS to fetch chapters on %s", t.URL)
	err := browserless.RunJS(t.URL, "div.listing-chapters_wrap", 5*time.Second, jsChapters, &chaptersJSON)
	if err != nil {
		logger.Error("ToonClash.FetchChapters: Error extracting chapters: %v", err)
		return nil, []error{fmt.Errorf("error extracting chapters: %w", err)}
	}
	var rawData []struct {
		Title  string  `json:"title"`
		Number float64 `json:"number"`
		URL    string  `json:"url"`
	}
	if err := json.Unmarshal([]byte(chaptersJSON), &rawData); err != nil {
		logger.Error("ToonClash.FetchChapters: Error parsing JSON: %v", err)
		return nil, []error{fmt.Errorf("error parsing chapters JSON: %w", err)}
	}
	chapters := make(Filterables, 0, len(rawData))
	for _, c := range rawData {
		if c.URL == "" {
			logger.Debug("ToonClash.FetchChapters: Skipping chapter with empty URL.")
			continue
		}
		chapters = append(chapters, &ToonClashChapter{
			Chapter: Chapter{
				Title:  c.Title,
				Number: c.Number,
			},
			URL: c.URL,
		})
	}
	return chapters, nil
}

// FetchChapterWithProgress loads a specific chapter page and extracts image URLs.
func (t *ToonClash) FetchChapterWithProgress(f Filterable, progressCallback func()) (*Chapter, error) {
	tc, ok := f.(*ToonClashChapter)
	if !ok {
		logger.Error("ToonClash.FetchChapterWithProgress: Invalid chapter type")
		return nil, fmt.Errorf("invalid chapter type")
	}
	logger.Debug("ToonClash.FetchChapterWithProgress: Fetching chapter: %s", tc.URL)
	_, err := browserless.FetchStringWithProgress(tc.URL, "div.reading-content", `
      document.documentElement.outerHTML
    `, 10*time.Second, progressCallback)
	if err != nil {
		logger.Error("ToonClash.FetchChapterWithProgress: Failed to load page: %v", err)
		return nil, fmt.Errorf("failed to fetch chapter page: %w", err)
	}
	var imageSrcs []string
	// inside FetchChapterWithProgress, replace the jsImages assignment with:
	jsImages := `
	(function(){
	window.scrollTo(0, document.body.scrollHeight);
	const start = Date.now();
	while (Date.now() - start < 500) {}
	return Array.from(
		document.querySelectorAll("div.reading-content img.wp-manga-chapter-img")
	)
	.map(img => {
		const raw = img.getAttribute("data-src") || img.src;
		return raw.trim();
	})
	.filter(src => src.startsWith("http"));
	})();
	`
	imageSrcs, err = browserless.FetchStringSliceWithProgress(tc.URL, "div.reading-content", jsImages, 10*time.Second, progressCallback)
	if err != nil {
		logger.Error("ToonClash.FetchChapterWithProgress: Error extracting images: %v", err)
		return nil, fmt.Errorf("failed to extract image URLs: %w", err)
	}
	if len(imageSrcs) == 0 {
		logger.Error("ToonClash.FetchChapterWithProgress: No images found")
		return nil, fmt.Errorf("no images found on chapter page")
	}
	pages := make([]Page, len(imageSrcs))
	for i, src := range imageSrcs {
		pages[i] = Page{Number: int64(i + 1), URL: src}
	}
	chapter := &Chapter{
		Title:      tc.Title,
		Number:     tc.Number,
		PagesCount: int64(len(pages)),
		Pages:      pages,
		Language:   "en",
	}
	return chapter, nil
}

// FetchChapter calls FetchChapterWithProgress without a callback.
func (t *ToonClash) FetchChapter(f Filterable) (*Chapter, error) {
	return t.FetchChapterWithProgress(f, func() {})
}

// BaseUrl returns the base URL for constructing relative links.
func (t *ToonClash) BaseUrl() string {
	u, err := url.Parse(t.URL)
	if err != nil {
		logger.Error("ToonClash.BaseUrl: Error parsing URL: %v", err)
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// GetFilenameTemplate returns the configured filename template.
func (t *ToonClash) GetFilenameTemplate() string {
	return t.Settings.FilenameTemplate
}

// GetMaxConcurrency returns the maximum number of concurrent downloads.
func (t *ToonClash) GetMaxConcurrency() MaxConcurrency {
	return t.Settings.MaxConcurrency
}
