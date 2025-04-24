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

// Kappabeast is a grabber implementation for www.kappabeast.com.
type Kappabeast struct {
	*Grabber
	title       string
	bypassOnce  sync.Once
	allocOnce   sync.Once
	allocCtx    context.Context
	allocCancel context.CancelFunc
}

// KappabeastChapter represents a chapter from the Kappabeast website.
type KappabeastChapter struct {
	Chapter
	URL string
}

// initAllocator sets up a shared ChromeDP allocator (browser instance) once.
func (k *Kappabeast) initAllocator() {
	k.allocOnce.Do(func() {
		remoteDebug := os.Getenv("REMOTE_DEBUG_URL")
		if remoteDebug == "" {
			remoteDebug = "http://localhost:6082"
		}
		allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), remoteDebug)
		k.allocCtx = allocCtx
		k.allocCancel = allocCancel
	})
}

// cleanupAllocator should be called at shutdown to close the browser.
func (k *Kappabeast) cleanupAllocator() {
	if k.allocCancel != nil {
		k.allocCancel()
	}
}

// Test checks if the current URL is hosted on "kappabeast.com".
func (k *Kappabeast) Test() (bool, error) {
	logger.Debug("Kappabeast.Test: Checking URL: %s", k.URL)
	return strings.Contains(k.URL, "kappabeast.com"), nil
}

// UsesBrowser returns true since Kappabeast requires a headless browser.
func (k *Kappabeast) UsesBrowser() bool {
	return true
}

// FetchTitle retrieves (and caches) the comic title.
func (k *Kappabeast) FetchTitle() (string, error) {
	if k.title != "" {
		return k.title, nil
	}

	// 1) Bypass Cloudflare once.
	var cfErr error
	k.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(k.URL, 1000)
	})
	if cfErr != nil {
		return "", fmt.Errorf("cloudflare bypass failed: %w", cfErr)
	}

	// 2) Initialize shared allocator.
	k.initAllocator()

	// 3) Create a new tab context.
	tCtx, cancel := chromedp.NewContext(k.allocCtx)
	defer cancel()

	// 4) Extract title via JS.
	var title string
	jsTitle := `document.querySelector("div.infox h1.entry-title")?.innerText || ""`
	if err := chromedp.Run(tCtx,
		chromedp.Navigate(k.URL),
		chromedp.WaitVisible("div.infox h1.entry-title", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.Evaluate(jsTitle, &title),
	); err != nil {
		return "", fmt.Errorf("chromedp title extraction failed: %w", err)
	}

	k.title = strings.TrimSpace(title)
	logger.Debug("Kappabeast.FetchTitle → %s", k.title)
	return k.title, nil
}

// FetchChapters retrieves the list of chapters.
func (k *Kappabeast) FetchChapters() (Filterables, []error) {
	// 1) Bypass Cloudflare once.
	var cfErr error
	k.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(k.URL, 1000)
	})
	if cfErr != nil {
		return nil, []error{fmt.Errorf("cloudflare bypass failed: %w", cfErr)}
	}

	// 2) Initialize shared allocator.
	k.initAllocator()

	// 3) New tab context.
	tCtx, cancel := chromedp.NewContext(k.allocCtx)
	defer cancel()

	// 4) Extract chapters via JS.
	var rawJSON string
	jsChapters := `(function(){
		var chapters = [];
		var items = document.querySelectorAll("div.eplister#chapterlist ul li");
		for(var i=0;i<items.length;i++){
			var a = items[i].querySelector("div.chbox div.eph-num a");
			if(!a) continue;
			var titleText = a.querySelector("span.chapternum").innerText.trim();
			var num = parseFloat(titleText.replace(/Chapter\s*/i, "")) || 0;
			var href = a.href;
			chapters.push({title: titleText, number: num, url: href});
		}
		return JSON.stringify(chapters);
	})();`

	if err := chromedp.Run(tCtx,
		chromedp.Navigate(k.URL),
		chromedp.WaitVisible("div.eplister#chapterlist", chromedp.ByQuery),
		chromedp.Sleep(800*time.Millisecond),
		chromedp.Evaluate(jsChapters, &rawJSON),
	); err != nil {
		return nil, []error{fmt.Errorf("chromedp chapters extraction failed: %w", err)}
	}

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
		out = append(out, &KappabeastChapter{
			Chapter: Chapter{Title: c.Title, Number: c.Number},
			URL:     c.URL,
		})
	}
	logger.Debug("Kappabeast.FetchChapters → %d chapters", len(out))
	return out, nil
}

// FetchChapter downloads one chapter by using FastAPI to batch-save images.
func (k *Kappabeast) FetchChapter(f Filterable) (*Chapter, error) {
	chap, ok := f.(*KappabeastChapter)
	if !ok {
		return nil, fmt.Errorf("invalid chapter type")
	}

	// 1) Derive the series slug from the title.
	fullTitle, err := k.FetchTitle()
	if err != nil {
		return nil, fmt.Errorf("could not fetch title: %w", err)
	}
	slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(fullTitle), " ", "-"))

	// 2) JavaScript extractor for images.
	jsImg := `(function(){
		var imgs = document.querySelectorAll("div#readerarea img.ts-main-image");
		var srcs = [];
		for(var i=0;i<imgs.length;i++){
			var s = imgs[i].getAttribute("src");
			if(s && s.trim()) srcs.push(s.trim());
		}
		return JSON.stringify(srcs);
	})();`

	// 3) Ask FastAPI to save the chapter images.
	if err := cloudflare.SaveChapter(chap.URL, jsImg, slug); err != nil {
		logger.Error("SaveChapter failed: %v", err)
	}

	// 4) Fetch back the filenames.
	fnames, err := cloudflare.GetSavedImages(chap.URL, slug)
	if err != nil {
		return nil, fmt.Errorf("GetSavedImages failed: %w", err)
	}

	// 5) Build our Chapter model.
	c := &Chapter{
		Title:      chap.Title,
		Number:     chap.Number,
		PagesCount: int64(len(fnames)),
		Language:   "en",
	}
	for idx, fn := range fnames {
		folder := path.Base(strings.TrimRight(chap.URL, "/"))
		getURL := fmt.Sprintf("%s/get_image?chapter=%s&filename=%s&slug=%s",
			cloudflare.FASTAPIBaseURL,
			url.PathEscape(folder),
			url.QueryEscape(fn),
			url.QueryEscape(slug),
		)
		c.Pages = append(c.Pages, Page{Number: int64(idx + 1), URL: getURL})
	}

	return c, nil
}

// BaseUrl returns the site’s origin.
func (k *Kappabeast) BaseUrl() string {
	u, _ := url.Parse(k.URL)
	return u.Scheme + "://" + u.Host
}
