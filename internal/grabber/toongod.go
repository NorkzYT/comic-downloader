package grabber

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/NorkzYT/comic-downloader/internal/cloudflare"
	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/chromedp/chromedp"
)

// Toongod is a grabber implementation for www.toongod.org.
type Toongod struct {
	*Grabber
	title       string
	bypassOnce  sync.Once
	allocOnce   sync.Once
	allocCtx    context.Context
	allocCancel context.CancelFunc
}

// ToongodChapter represents a chapter from the Toongod website.
type ToongodChapter struct {
	Chapter
	URL string
}

// initAllocator sets up a shared ChromeDP allocator (browser instance) once.
func (t *Toongod) initAllocator() {
	t.allocOnce.Do(func() {
		remoteDebug := os.Getenv("REMOTE_DEBUG_URL")
		if remoteDebug == "" {
			remoteDebug = "http://localhost:6082"
		}
		allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), remoteDebug)
		t.allocCtx = allocCtx
		t.allocCancel = allocCancel
	})
}

// cleanupAllocator should be called at shutdown to close the browser.
func (t *Toongod) cleanupAllocator() {
	if t.allocCancel != nil {
		t.allocCancel()
	}
}

// Test checks if the current URL is hosted on "toongod.org".
func (t *Toongod) Test() (bool, error) {
	logger.Debug("Toongod.Test: Checking URL: %s", t.URL)
	return strings.Contains(t.URL, "toongod.org"), nil
}

// UsesBrowser returns true since Toongod requires a headless browser.
func (t *Toongod) UsesBrowser() bool {
	return true
}

// FetchTitle retrieves (and caches) the comic title.
func (t *Toongod) FetchTitle() (string, error) {
	if t.title != "" {
		return t.title, nil
	}

	// 1) Bypass Cloudflare once.
	var cfErr error
	t.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(t.URL, 1000)
	})
	if cfErr != nil {
		return "", fmt.Errorf("cloudflare bypass failed: %w", cfErr)
	}

	// 2) Initialize shared allocator.
	t.initAllocator()

	// 3) Create a new tab context for this operation.
	tCtx, tabCancel := chromedp.NewContext(t.allocCtx)
	defer tabCancel()

	// 4) Add timeout.
	ctx, cancel := context.WithTimeout(tCtx, 30*time.Second)
	defer cancel()

	// 5) Extract title via JS.
	var title string
	jsTitle := `document.querySelector("div.post-title h1")?.innerText||""`
	if err := chromedp.Run(ctx,
		chromedp.Navigate(t.URL),
		chromedp.WaitVisible("div.post-title h1", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(jsTitle, &title),
	); err != nil {
		return "", fmt.Errorf("chromedp title extraction failed: %w", err)
	}

	t.title = strings.TrimSpace(title)
	logger.Debug("Toongod.FetchTitle → %s", t.title)
	return t.title, nil
}

// FetchChapters retrieves the list of chapters.
func (t *Toongod) FetchChapters() (Filterables, []error) {
	// 1) Bypass Cloudflare once.
	var cfErr error
	t.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(t.URL, 1000)
	})
	if cfErr != nil {
		return nil, []error{fmt.Errorf("cloudflare bypass failed: %w", cfErr)}
	}

	// 2) Initialize shared allocator.
	t.initAllocator()

	// 3) New tab context.
	tCtx, tabCancel := chromedp.NewContext(t.allocCtx)
	defer tabCancel()

	// 4) Timeout for this op.
	ctx, cancel := context.WithTimeout(tCtx, 30*time.Second)
	defer cancel()

	// 5) Extract chapters via JS.
	var rawJSON string
	jsChapters := `(function(){
        var chapters = [];
        var items = document.querySelectorAll("ul.main.version-chap.no-volumn.active li.wp-manga-chapter");
        for(var i = 0; i < items.length; i++){
            var link = items[i].querySelector("a");
            if(!link) continue;
            var titleText = link.textContent.trim();
            var num = parseFloat(titleText.replace(/Chapter\s*/i, "")) || 0;
            var href = link.href;
            chapters.push({title: titleText, number: num, url: href});
        }
        return JSON.stringify(chapters);
    })();`

	if err := chromedp.Run(ctx,
		chromedp.Navigate(t.URL),
		chromedp.WaitVisible("ul.main.version-chap.no-volumn.active", chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(jsChapters, &rawJSON),
	); err != nil {
		return nil, []error{fmt.Errorf("chromedp chapters extraction failed: %w", err)}
	}

	// 6) Unmarshal and wrap.
	var list []struct {
		Title  string  `json:"title"`
		Number float64 `json:"number"`
		URL    string  `json:"url"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &list); err != nil {
		return nil, []error{fmt.Errorf("invalid chapters JSON: %w", err)}
	}

	var out Filterables
	for _, c := range list {
		out = append(out, &ToongodChapter{
			Chapter: Chapter{Title: c.Title, Number: c.Number},
			URL:     c.URL,
		})
	}
	logger.Debug("Toongod.FetchChapters → %d chapters", len(out))
	return out, nil
}

// FetchChapter downloads one chapter by using FastAPI to batch-save images.
func (t *Toongod) FetchChapter(f Filterable) (*Chapter, error) {
	tc, ok := f.(*ToongodChapter)
	if !ok {
		return nil, fmt.Errorf("invalid chapter type")
	}

	// 1) Derive the series slug from the title
	fullTitle, err := t.FetchTitle()
	if err != nil {
		return nil, fmt.Errorf("could not fetch title: %w", err)
	}
	// e.g. "The Knight King Who Returned with a God" → "the-knight-king-who-returned-with-a-god"
	slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(fullTitle), " ", "-"))

	// 2) JavaScript extractor
	jsImg := `(function(){
        var imgs = document.querySelectorAll("div.reading-content img.wp-manga-chapter-img");
        var srcs = [];
        for(var i=0;i<imgs.length;i++){
            var src = imgs[i].getAttribute("src") || imgs[i].getAttribute("data-src");
            if(src && src.trim()) srcs.push(src.trim());
        }
        return JSON.stringify(srcs);
    })();`

	// 3) Ask Tenshi (FastAPI) to save whole chapter into /tenshi/data/{slug}/{chapter‑folder}
	if err := cloudflare.SaveChapter(tc.URL, jsImg, slug); err != nil {
		logger.Error("SaveChapter failed: %v", err)
	}

	// 4) Fetch back the filenames
	fnames, err := cloudflare.GetSavedImages(tc.URL, slug)
	if err != nil {
		return nil, fmt.Errorf("GetSavedImages failed: %w", err)
	}

	// 5) Build our Chapter model with the FastAPI URLs (including slug)
	ch := &Chapter{
		Title:      tc.Title,
		Number:     tc.Number,
		PagesCount: int64(len(fnames)),
		Language:   "en",
	}
	for idx, fn := range fnames {
		chapFolder := path.Base(tc.URL)
		getURL := fmt.Sprintf(
			"%s/get_image?chapter=%s&filename=%s&slug=%s",
			cloudflare.FASTAPIBaseURL,
			url.PathEscape(chapFolder),
			url.QueryEscape(fn),
			url.QueryEscape(slug),
		)
		ch.Pages = append(ch.Pages, Page{
			Number: int64(idx + 1),
			URL:    getURL,
		})
	}

	return ch, nil
}

// BaseUrl returns the site’s origin.
func (t *Toongod) BaseUrl() string {
	u, _ := url.Parse(t.URL)
	return u.Scheme + "://" + u.Host
}
