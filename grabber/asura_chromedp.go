package grabber

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/NorkzYT/comic-downloader/browserless"
)

// AsuraChromedp implements the Site interface for asuracomic.net using chromedp.
// It uses a remote browser (e.g. Browserless) to scrape both the series page and chapter pages.
type AsuraChromedp struct {
	*Grabber
}

// AsuraChapter represents a chapter for asuracomic.net scraped via chromedp.
type AsuraChapter struct {
	Chapter // Embeds the common Chapter struct
	URL     string
}

// Test verifies if the URL is from asuracomic.net.
func (a *AsuraChromedp) Test() (bool, error) {
	return strings.Contains(a.URL, "asuracomic.net"), nil
}

// FetchTitle navigates to the series URL and extracts the comic title.
// It first attempts to extract the title using a specific DOM selector,
// and if that fails, falls back to document.title.
func (a *AsuraChromedp) FetchTitle() (string, error) {
	var title string

	// Attempt to extract title using a specific selector.
	// The JS snippet returns innerText of the targeted <span>.
	jsTitle := `document.querySelector("div.text-center.sm\\:text-left span.text-xl.font-bold") ? document.querySelector("div.text-center.sm\\:text-left span.text-xl.font-bold").innerText : ""`
	err := browserless.RunJS(a.URL, "body", 0, jsTitle, &title)
	if err != nil {
		return "", fmt.Errorf("error fetching title with selector: %w", err)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		// Fallback: use document.title if the selector did not yield a result.
		jsDocTitle := `document.title`
		err = browserless.RunJS(a.URL, "body", 0, jsDocTitle, &title)
		if err != nil {
			return "", fmt.Errorf("error fetching document.title: %w", err)
		}
		title = strings.TrimSpace(title)
	}
	return title, nil
}

// FetchChapters uses a JavaScript snippet to extract chapter data from the series page.
// The returned JSON is parsed into chapter structures.
func (a *AsuraChromedp) FetchChapters() (Filterables, []error) {
	var chaptersJSON string
	// Custom JS snippet to extract chapters.
	jsChapters := `(function(){
		var chapters = [];
		var links = document.querySelectorAll("div.overflow-y-auto a");
		for(var i = 0; i < links.length; i++){
			var rawTitle = links[i].textContent.trim();
			// Extract only "Chapter <number>" from the full text.
			var match = rawTitle.match(/(Chapter\s*\d+(?:\.\d+)?)/i);
			var title = match ? match[0] : rawTitle;
			var href = links[i].getAttribute("href");
			var num = 0;
			if(match){
				// Remove non-digit characters before parsing.
				num = parseFloat(match[0].replace(/[^0-9.]/g, ""));
			}
			// Adjust relative URLs.
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

	// Wait for the chapter container and allow extra time (5 seconds) for the list to load.
	err := browserless.RunJS(a.URL, "div.overflow-y-auto", 5*time.Second, jsChapters, &chaptersJSON)
	if err != nil {
		return nil, []error{fmt.Errorf("error extracting chapters: %w", err)}
	}

	// Parse the returned JSON.
	var rawChapters []struct {
		Title  string  `json:"title"`
		Number float64 `json:"number"`
		URL    string  `json:"url"`
	}
	if err = json.Unmarshal([]byte(chaptersJSON), &rawChapters); err != nil {
		return nil, []error{fmt.Errorf("error parsing chapters JSON: %w", err)}
	}

	// Convert the raw chapter data into the expected Filterables format.
	chapters := make(Filterables, 0, len(rawChapters))
	for _, c := range rawChapters {
		// Only add chapters with valid URLs.
		if c.URL == "" {
			continue
		}
		ac := &AsuraChapter{
			Chapter: Chapter{
				Title:  c.Title,
				Number: c.Number,
			},
			URL: c.URL,
		}
		chapters = append(chapters, ac)
	}

	return chapters, nil
}

// FetchChapter navigates to a chapter URL and extracts the image URLs via a JavaScript snippet.
// It then constructs a Chapter with its Pages populated.
func (a *AsuraChromedp) FetchChapter(f Filterable) (*Chapter, error) {
	ac, ok := f.(*AsuraChapter)
	if !ok {
		return nil, fmt.Errorf("invalid chapter type")
	}

	var chapterHTML string
	// Fetch the chapter page; wait for <body> and allow extra time for images to load.
	err := browserless.RunJS(ac.URL, "body", 5*time.Second, `document.documentElement.outerHTML`, &chapterHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chapter page: %w", err)
	}

	// Execute JS to extract image URLs from the designated container.
	var imageSrcs []string
	jsImages := `(function(){
		window.scrollTo(0, document.body.scrollHeight);
		return Array.from(document.querySelectorAll("div.w-full.mx-auto.center img"))
				.map(img => img.src)
				.filter(src => src && src.startsWith("http"));
	})();`
	err = browserless.RunJS(ac.URL, "body", 5*time.Second, jsImages, &imageSrcs)
	if err != nil {
		return nil, fmt.Errorf("failed to extract image URLs: %w", err)
	}
	if len(imageSrcs) == 0 {
		return nil, fmt.Errorf("no images found on chapter page")
	}

	// Create a slice of Page objects (one per image).
	pages := make([]Page, len(imageSrcs))
	for i, src := range imageSrcs {
		pages[i] = Page{
			Number: int64(i + 1),
			URL:    src,
		}
	}

	// Construct and return the Chapter.
	chapter := &Chapter{
		Title:      ac.Title,
		Number:     ac.Number,
		PagesCount: int64(len(pages)),
		Pages:      pages,
		Language:   "en",
	}
	return chapter, nil
}

// BaseUrl returns the base URL for asuracomic.net derived from the chapter URL.
func (a *AsuraChromedp) BaseUrl() string {
	// Parse a.URL to extract the scheme and host.
	u, err := url.Parse(a.URL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// GetFilenameTemplate and GetMaxConcurrency simply return the settings values.
func (a *AsuraChromedp) GetFilenameTemplate() string {
	return a.Settings.FilenameTemplate
}

func (a *AsuraChromedp) GetMaxConcurrency() MaxConcurrency {
	return a.Settings.MaxConcurrency
}

func (a *AsuraChromedp) GetPreferredLanguage() string {
	return a.Settings.Language
}
