package grabber

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// AsuraChromedp implements the Site interface for asuracomic.net using chromedp.
// It uses a remote browser (e.g. Browserless) to scrape both the series page and chapter pages.
type AsuraChromedp struct {
	*Grabber
}

// Test verifies if the URL is from asuracomic.net.
func (a *AsuraChromedp) Test() (bool, error) {
	return strings.Contains(a.URL, "asuracomic.net"), nil
}

// newRemoteContext creates a Chromedp context that connects to a remote Browserless instance.
// It checks the DEVTOOLS_WS_URL environment variable; if not set, it falls back to a default endpoint.
func newRemoteContext() (context.Context, context.CancelFunc, error) {
	// You may replace this default with your Browserless endpoint and token.
	devtoolsWsURL := "ws://localhost:8454?token=6R0W53R135510"
	log.Printf("Connecting to Browserless at: %s\n", devtoolsWsURL)

	// Create a parent context with a timeout to avoid hanging indefinitely.
	parentCtx, cancelParent := context.WithTimeout(context.Background(), 30*time.Second)

	// Create the remote allocator with the parent context.
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(parentCtx, devtoolsWsURL, chromedp.NoModifyURL)

	// Create a new chromedp context.
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	// Combine all cancel functions.
	cancel := func() {
		cancelCtx()
		cancelAlloc()
		cancelParent()
	}

	return ctx, cancel, nil
}

// FetchTitle navigates to the series URL and extracts the comic title.
// It first tries to get the text of the <span> element with classes "text-xl font-bold".
// If that fails, it falls back to evaluating document.title.
func (a *AsuraChromedp) FetchTitle() (string, error) {
	ctx, cancel, err := newRemoteContext()
	if err != nil {
		return "", fmt.Errorf("failed to create remote context: %w", err)
	}
	defer cancel()

	var title string
	// Use the correct selector for asuracomic.net's comic title.
	err = chromedp.Run(ctx,
		chromedp.Navigate(a.URL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Text("div.text-center.sm\\:text-left span.text-xl.font-bold", &title, chromedp.NodeVisible, chromedp.ByQuery),
	)
	if err != nil {
		return "", fmt.Errorf("error fetching title: %w", err)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		// Fallback to document.title if the <span> is not available.
		err = chromedp.Run(ctx,
			chromedp.Evaluate("document.title", &title),
		)
		if err != nil {
			return "", fmt.Errorf("error evaluating document.title: %w", err)
		}
	}
	return title, nil
}

// ChapterData is used to unmarshal the JSON output from the series page.
// It holds the chapter title, its numeric value, and its URL.
type ChapterData struct {
	Title  string  `json:"title"`
	Number float64 `json:"number"`
	URL    string  `json:"url"`
}

// AsuraChapter implements the Filterable interface for a chapter from asuracomic.net.
type AsuraChapter struct {
	Chapter
	URL string
}

// FetchChapters navigates to the series page and uses a JavaScript snippet to extract all chapter links.
func (a *AsuraChromedp) FetchChapters() (Filterables, []error) {
	ctx, cancel, err := newRemoteContext()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to create remote context: %w", err)}
	}
	defer cancel()

	// Updated JavaScript snippet to extract a clean chapter title (e.g. "Chapter 1")
	js := `(function(){
		var chapters = [];
		var links = document.querySelectorAll("div.overflow-y-auto a");
		for(var i = 0; i < links.length; i++){
			var rawTitle = links[i].textContent.trim();
			// Use a regex to extract only "Chapter <number>" from the full text.
			var match = rawTitle.match(/(Chapter\s*\d+(?:\.\d+)?)/i);
			var title = match ? match[0] : rawTitle;
			var href = links[i].getAttribute("href");
			var num = 0;
			if(match){
				// Remove any non-digit/dot characters before parsing the number.
				num = parseFloat(match[0].replace(/[^0-9.]/g, ""));
			}
			// Fix relative URL if needed.
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

	var chaptersJSON string
	err = chromedp.Run(ctx,
		chromedp.Navigate(a.URL),
		chromedp.WaitVisible("div.overflow-y-auto", chromedp.ByQuery),
		// Wait extra seconds to ensure that the chapter list is fully loaded.
		chromedp.Sleep(5*time.Second),
		chromedp.Evaluate(js, &chaptersJSON),
	)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to evaluate chapter list: %w", err)}
	}

	var chaps []ChapterData
	err = json.Unmarshal([]byte(chaptersJSON), &chaps)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to unmarshal chapter data: %w", err)}
	}

	var filterables Filterables
	for _, c := range chaps {
		// Skip chapters with an empty URL.
		if c.URL == "" {
			continue
		}
		ac := &AsuraChapter{
			Chapter: Chapter{
				Title:  c.Title, // This will now be the simplified title like "Chapter 1"
				Number: c.Number,
			},
			URL: c.URL,
		}
		filterables = append(filterables, ac)
	}
	return filterables, nil
}

// FetchChapter navigates to the chapter URL (from the AsuraChapter),
// waits for the page content to load, and then uses a JavaScript snippet
// to extract all image URLs from a container with class "w-full mx-auto center".
// It returns a Chapter with its Pages slice populated.
func (a *AsuraChromedp) FetchChapter(f Filterable) (*Chapter, error) {
	ac, ok := f.(*AsuraChapter)
	if !ok {
		return nil, fmt.Errorf("invalid chapter type")
	}
	ctx, cancel, err := newRemoteContext()
	if err != nil {
		return nil, fmt.Errorf("failed to create remote context: %w", err)
	}
	defer cancel()

	// Navigate to the chapter page and wait for its content to load.
	var chapterHTML string
	err = chromedp.Run(ctx,
		chromedp.Navigate(ac.URL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(5*time.Second), // Allow extra time for images to load
		chromedp.OuterHTML("html", &chapterHTML, chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chapter page: %w", err)
	}

	// Execute JavaScript to extract image URLs from the designated container.
	var imageSrcs []string
	jsImages := `Array.from(document.querySelectorAll("div.w-full.mx-auto.center img")).map(img => img.src)`
	err = chromedp.Run(ctx,
		chromedp.Evaluate(jsImages, &imageSrcs),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate image extraction script: %w", err)
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

	// Return the Chapter populated with its title, chapter number, total pages, and pages.
	chapter := &Chapter{
		Title:      ac.Title,
		Number:     ac.Number,
		PagesCount: int64(len(pages)),
		Pages:      pages,
		Language:   "en", // Default language; adjust if needed.
	}
	return chapter, nil
}

// BaseUrl returns the base URL for asuracomic.net derived from the chapter URL.
func (a *AsuraChromedp) BaseUrl() string {
	u, err := url.Parse(a.URL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// GetFilenameTemplate returns the filename template from settings.
func (a *AsuraChromedp) GetFilenameTemplate() string {
	return a.Settings.FilenameTemplate
}

// GetMaxConcurrency returns the max concurrency settings for this site.
func (a *AsuraChromedp) GetMaxConcurrency() MaxConcurrency {
	return a.Settings.MaxConcurrency
}

// GetPreferredLanguage returns the preferred language setting.
func (a *AsuraChromedp) GetPreferredLanguage() string {
	return a.Settings.Language
}
