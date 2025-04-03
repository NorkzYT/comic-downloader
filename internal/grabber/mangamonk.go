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

// Mangamonk implements the Site interface for mangamonk.com using browserless/chromedp.
type Mangamonk struct {
	*Grabber
	// Additional fields can be added if needed.
}

// MangamonkChapter represents a chapter for mangamonk.com scraped via chromedp.
type MangamonkChapter struct {
	Chapter // Embeds the common Chapter struct
	URL     string
}

// Test verifies if the URL is for mangamonk.com.
func (m *Mangamonk) Test() (bool, error) {
	logger.Debug("Mangamonk.Test: Checking if URL contains 'mangamonk.com': %s", m.URL)
	return strings.Contains(m.URL, "mangamonk.com"), nil
}

func (a *Mangamonk) UsesBrowser() bool {
	return true
}

// FetchTitle navigates to the series URL and extracts the comic title.
func (m *Mangamonk) FetchTitle() (string, error) {
	var title string
	jsTitle := `document.querySelector("div.name.box h1") ? document.querySelector("div.name.box h1").innerText : ""`
	logger.Debug("Mangamonk.FetchTitle: Running JS for title extraction on %s", m.URL)
	err := browserless.RunJS(m.URL, "div.name.box h1", 0, jsTitle, &title)
	if err != nil {
		logger.Error("Mangamonk.FetchTitle: Error fetching title: %v", err)
		return "", fmt.Errorf("error fetching title: %w", err)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		// Fallback to document.title if necessary.
		jsDocTitle := `document.title`
		logger.Debug("Mangamonk.FetchTitle: Title empty, falling back to document.title on %s", m.URL)
		err = browserless.RunJS(m.URL, "body", 0, jsDocTitle, &title)
		if err != nil {
			logger.Error("Mangamonk.FetchTitle: Error fetching document.title: %v", err)
			return "", fmt.Errorf("error fetching document.title: %w", err)
		}
		title = strings.TrimSpace(title)
	}
	logger.Debug("Mangamonk.FetchTitle: Fetched title: %s", title)
	return title, nil
}

// FetchChapters uses JavaScript to extract chapter data from the series page.
func (m *Mangamonk) FetchChapters() (Filterables, []error) {
	var chaptersJSON string
	jsChapters := `(function(){
		var chapters = [];
		var items = document.querySelectorAll("ul.chapter-list li");
		for(var i = 0; i < items.length; i++){
			var link = items[i].querySelector("a");
			if(!link) continue;
			var titleElem = link.querySelector("strong.chapter-title");
			var rawTitle = titleElem ? titleElem.innerText.trim() : link.getAttribute("title");
			var match = rawTitle.match(/Chapter\s*(\d+(?:\.\d+)?)/i);
			var num = match ? parseFloat(match[1]) : 0;
			var href = link.getAttribute("href") || "";
			if(href && !href.startsWith("http")){
				if(href[0] != '/'){
					href = "/" + href;
				}
				href = window.location.origin + href;
			}
			chapters.push({title: rawTitle, number: num, url: href});
		}
		return JSON.stringify(chapters);
	})();`
	logger.Debug("Mangamonk.FetchChapters: Executing JS to fetch chapters on %s", m.URL)
	err := browserless.RunJS(m.URL, "ul.chapter-list", 5*time.Second, jsChapters, &chaptersJSON)
	if err != nil {
		logger.Error("Mangamonk.FetchChapters: Error extracting chapters: %v", err)
		return nil, []error{fmt.Errorf("error extracting chapters: %w", err)}
	}
	var rawChapters []struct {
		Title  string  `json:"title"`
		Number float64 `json:"number"`
		URL    string  `json:"url"`
	}
	if err = json.Unmarshal([]byte(chaptersJSON), &rawChapters); err != nil {
		logger.Error("Mangamonk.FetchChapters: Error parsing chapters JSON: %v", err)
		return nil, []error{fmt.Errorf("error parsing chapters JSON: %w", err)}
	}
	chapters := make(Filterables, 0, len(rawChapters))
	for _, c := range rawChapters {
		if c.URL == "" {
			logger.Debug("Mangamonk.FetchChapters: Skipping chapter with empty URL.")
			continue
		}
		mc := &MangamonkChapter{
			Chapter: Chapter{
				Title:  c.Title,
				Number: c.Number,
			},
			URL: c.URL,
		}
		logger.Debug("Mangamonk.FetchChapters: Found chapter: %s", mc.Title)
		chapters = append(chapters, mc)
	}
	return chapters, nil
}

// FetchChapterWithProgress navigates to a chapter URL and extracts image URLs,
// using a progress callback during long-running evaluations.
func (m *Mangamonk) FetchChapterWithProgress(f Filterable, progressCallback func()) (*Chapter, error) {
	mc, ok := f.(*MangamonkChapter)
	if !ok {
		logger.Error("Mangamonk.FetchChapterWithProgress: Invalid chapter type")
		return nil, fmt.Errorf("invalid chapter type")
	}
	logger.Debug("Mangamonk.FetchChapterWithProgress: Fetching chapter from URL: %s", mc.URL)
	// Ensure the chapter page is loaded.
	_, err := browserless.FetchStringWithProgress(mc.URL, "body", `document.documentElement.outerHTML`, 10*time.Second, progressCallback)
	if err != nil {
		logger.Error("Mangamonk.FetchChapterWithProgress: Failed to fetch chapter page: %v", err)
		return nil, fmt.Errorf("failed to fetch chapter page: %w", err)
	}

	var imageSrcs []string
	jsImages := `(function(){
		// Scroll down to trigger lazy-loading.
		window.scrollTo(0, document.body.scrollHeight);
		var start = Date.now();
		while(Date.now() - start < 1000){}
		// Only select images that are inside elements with the "chapter-image" class.
		var imgs = document.querySelectorAll("#chapter-images .chapter-image img");
		var srcs = [];
		for (var i = 0; i < imgs.length; i++){
			var src = imgs[i].getAttribute("src");
			if(src && src.startsWith("http")){
				srcs.push(src);
			}
		}
		return srcs;
	})();`
	imageSrcs, err = browserless.FetchStringSliceWithProgress(mc.URL, "body", jsImages, 10*time.Second, progressCallback)
	if err != nil {
		logger.Error("Mangamonk.FetchChapterWithProgress: Failed to extract image URLs: %v", err)
		return nil, fmt.Errorf("failed to extract image URLs: %w", err)
	}
	if len(imageSrcs) == 0 {
		logger.Error("Mangamonk.FetchChapterWithProgress: No images found on chapter page")
		return nil, fmt.Errorf("no images found on chapter page")
	}

	pages := make([]Page, len(imageSrcs))
	for i, src := range imageSrcs {
		pages[i] = Page{
			Number: int64(i + 1),
			URL:    src,
		}
	}
	chapter := &Chapter{
		Title:      mc.Title,
		Number:     mc.Number,
		PagesCount: int64(len(pages)),
		Pages:      pages,
		Language:   "en",
	}
	logger.Debug("Mangamonk.FetchChapterWithProgress: Successfully fetched chapter: %s", chapter.Title)
	return chapter, nil
}

// FetchChapter implements the Site interface by calling FetchChapterWithProgress with a no-op callback.
func (m *Mangamonk) FetchChapter(f Filterable) (*Chapter, error) {
	return m.FetchChapterWithProgress(f, func() {})
}

// BaseUrl returns the base URL for mangamonk.com derived from the chapter URL.
func (m *Mangamonk) BaseUrl() string {
	u, err := url.Parse(m.URL)
	if err != nil {
		logger.Error("Mangamonk.BaseUrl: Error parsing URL: %v", err)
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// GetFilenameTemplate returns the filename template from settings.
func (m *Mangamonk) GetFilenameTemplate() string {
	return m.Settings.FilenameTemplate
}

// GetMaxConcurrency returns the max concurrency settings.
func (m *Mangamonk) GetMaxConcurrency() MaxConcurrency {
	return m.Settings.MaxConcurrency
}

// GetPreferredLanguage returns the preferred language for the site.
func (m *Mangamonk) GetPreferredLanguage() string {
	return m.Settings.Language
}
