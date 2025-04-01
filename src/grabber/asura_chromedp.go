package grabber

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/NorkzYT/comic-downloader/src/browserless"
	"github.com/NorkzYT/comic-downloader/src/logger"
)

// AsuraChromedp implements the Site interface for asuracomic.net using chromedp.
type AsuraChromedp struct {
	*Grabber
	// Additional fields if needed.
}

// AsuraChapter represents a chapter for asuracomic.net scraped via chromedp.
type AsuraChapter struct {
	Chapter // Embeds the common Chapter struct
	URL     string
}

// Test verifies if the URL is from asuracomic.net.
func (a *AsuraChromedp) Test() (bool, error) {
	logger.Debug("AsuraChromedp.Test: Checking if URL contains 'asuracomic.net': %s", a.URL)
	return strings.Contains(a.URL, "asuracomic.net"), nil
}

// FetchTitle navigates to the series URL and extracts the comic title.
func (a *AsuraChromedp) FetchTitle() (string, error) {
	var title string
	jsTitle := `document.querySelector("div.text-center.sm\\:text-left span.text-xl.font-bold") ? document.querySelector("div.text-center.sm\\:text-left span.text-xl.font-bold").innerText : ""`
	logger.Debug("AsuraChromedp.FetchTitle: Running JS for title extraction on %s", a.URL)
	err := browserless.RunJS(a.URL, "body", 0, jsTitle, &title)
	if err != nil {
		logger.Error("AsuraChromedp.FetchTitle: Error fetching title with selector: %v", err)
		return "", fmt.Errorf("error fetching title with selector: %w", err)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		jsDocTitle := `document.title`
		logger.Debug("AsuraChromedp.FetchTitle: Title empty, falling back to document.title on %s", a.URL)
		err = browserless.RunJS(a.URL, "body", 0, jsDocTitle, &title)
		if err != nil {
			logger.Error("AsuraChromedp.FetchTitle: Error fetching document.title: %v", err)
			return "", fmt.Errorf("error fetching document.title: %w", err)
		}
		title = strings.TrimSpace(title)
	}
	logger.Debug("AsuraChromedp.FetchTitle: Fetched title: %s", title)
	return title, nil
}

// FetchChapters uses a JavaScript snippet to extract chapter data from the series page.
func (a *AsuraChromedp) FetchChapters() (Filterables, []error) {
	var chaptersJSON string
	jsChapters := `(function(){
		var chapters = [];
		var links = document.querySelectorAll("div.overflow-y-auto a");
		for(var i = 0; i < links.length; i++){
			var rawTitle = links[i].textContent.trim();
			var match = rawTitle.match(/(Chapter\s*\d+(?:\.\d+)?)/i);
			var title = match ? match[0] : rawTitle;
			var href = links[i].getAttribute("href");
			var num = 0;
			if(match){
				num = parseFloat(match[0].replace(/[^0-9.]/g, ""));
			}
			if(href && !href.startsWith("http")){
				if(href[0] != '/'){
					href = "/" + href;
				}
				href = window.location.origin + "/series" + href;
			}
			chapters.push({title: title, number: num, url: href});
		}
		return JSON.stringify(chapters);
	})();`
	logger.Debug("AsuraChromedp.FetchChapters: Executing JS to fetch chapters on %s", a.URL)
	err := browserless.RunJS(a.URL, "div.overflow-y-auto", 5*time.Second, jsChapters, &chaptersJSON)
	if err != nil {
		logger.Error("AsuraChromedp.FetchChapters: Error extracting chapters: %v", err)
		return nil, []error{fmt.Errorf("error extracting chapters: %w", err)}
	}
	var rawChapters []struct {
		Title  string  `json:"title"`
		Number float64 `json:"number"`
		URL    string  `json:"url"`
	}
	if err = json.Unmarshal([]byte(chaptersJSON), &rawChapters); err != nil {
		logger.Error("AsuraChromedp.FetchChapters: Error parsing chapters JSON: %v", err)
		return nil, []error{fmt.Errorf("error parsing chapters JSON: %w", err)}
	}
	chapters := make(Filterables, 0, len(rawChapters))
	for _, c := range rawChapters {
		if c.URL == "" {
			logger.Debug("AsuraChromedp.FetchChapters: Skipping chapter with empty URL.")
			continue
		}
		ac := &AsuraChapter{
			Chapter: Chapter{
				Title:  c.Title,
				Number: c.Number,
			},
			URL: c.URL,
		}
		logger.Debug("AsuraChromedp.FetchChapters: Found chapter: %s", ac.Title)
		chapters = append(chapters, ac)
	}
	return chapters, nil
}

// FetchChapterWithProgress navigates to a chapter URL and extracts image URLs,
// calling the provided progressCallback during long-running evaluations.
func (a *AsuraChromedp) FetchChapterWithProgress(f Filterable, progressCallback func()) (*Chapter, error) {
	ac, ok := f.(*AsuraChapter)
	if !ok {
		logger.Error("AsuraChromedp.FetchChapterWithProgress: Invalid chapter type")
		return nil, fmt.Errorf("invalid chapter type")
	}
	logger.Debug("AsuraChromedp.FetchChapterWithProgress: Fetching chapter with URL: %s", ac.URL)
	_, err := browserless.FetchStringWithProgress(ac.URL, "body", `document.documentElement.outerHTML`, 10*time.Second, progressCallback)
	if err != nil {
		logger.Error("AsuraChromedp.FetchChapterWithProgress: Failed to fetch chapter page: %v", err)
		return nil, fmt.Errorf("failed to fetch chapter page: %w", err)
	}

	var imageSrcs []string
	jsImages := `(function(){
		var adOverlay = document.querySelector("div.fixed.inset-0.bg-gray-900");
		if(adOverlay && adOverlay.parentNode) {
			adOverlay.parentNode.removeChild(adOverlay);
		}
		window.scrollTo(0, document.body.scrollHeight);
		var start = Date.now();
		while(Date.now() - start < 1000) {}
		return Array.from(document.querySelectorAll("div.w-full.mx-auto.center img"))
			.map(img => img.src)
			.filter(src => src && src.startsWith("http"));
	})();`
	imageSrcs, err = browserless.FetchStringSliceWithProgress(ac.URL, "body", jsImages, 10*time.Second, progressCallback)
	if err != nil {
		logger.Error("AsuraChromedp.FetchChapterWithProgress: Failed to extract image URLs: %v", err)
		return nil, fmt.Errorf("failed to extract image URLs: %w", err)
	}
	if len(imageSrcs) == 0 {
		logger.Error("AsuraChromedp.FetchChapterWithProgress: No images found on chapter page")
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
		Title:      ac.Title,
		Number:     ac.Number,
		PagesCount: int64(len(pages)),
		Pages:      pages,
		Language:   "en",
	}
	logger.Debug("AsuraChromedp.FetchChapterWithProgress: Successfully fetched chapter: %s", chapter.Title)
	return chapter, nil
}

// FetchChapter implements the Site interface by calling FetchChapterWithProgress with a no-op callback.
func (a *AsuraChromedp) FetchChapter(f Filterable) (*Chapter, error) {
	return a.FetchChapterWithProgress(f, func() {})
}

// BaseUrl returns the base URL for asuracomic.net derived from the chapter URL.
func (a *AsuraChromedp) BaseUrl() string {
	u, err := url.Parse(a.URL)
	if err != nil {
		logger.Error("AsuraChromedp.BaseUrl: Error parsing URL: %v", err)
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// GetFilenameTemplate returns the filename template.
func (a *AsuraChromedp) GetFilenameTemplate() string {
	return a.Settings.FilenameTemplate
}

// GetMaxConcurrency returns the max concurrency settings.
func (a *AsuraChromedp) GetMaxConcurrency() MaxConcurrency {
	return a.Settings.MaxConcurrency
}

// GetPreferredLanguage returns the preferred language.
func (a *AsuraChromedp) GetPreferredLanguage() string {
	return a.Settings.Language
}
