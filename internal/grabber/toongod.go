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
	title      string
	bypassOnce sync.Once
}

// ToongodChapter represents a chapter from the Toongod website.
type ToongodChapter struct {
	Chapter
	URL string
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

	// 1) Bypass Cloudflare once (share cookies across domain).
	var cfErr error
	t.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(t.URL, 5000)
	})
	if cfErr != nil {
		return "", fmt.Errorf("cloudflare bypass failed: %w", cfErr)
	}

	// 2) Headless extract via existing remote-debug session.
	remoteDebugURL := os.Getenv("REMOTE_DEBUG_URL")
	if remoteDebugURL == "" {
		remoteDebugURL = "http://localhost:6082"
	}
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), remoteDebugURL)
	defer allocCancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var title string
	jsTitle := `document.querySelector("div.post-title h1") ? document.querySelector("div.post-title h1").innerText : "";`
	err := chromedp.Run(ctx,
		chromedp.Navigate(t.URL),
		chromedp.WaitVisible("div.post-title h1", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(jsTitle, &title),
	)
	if err != nil {
		return "", fmt.Errorf("chromedp title extraction failed: %w", err)
	}

	t.title = strings.TrimSpace(title)
	logger.Debug("Toongod.FetchTitle → %s", t.title)
	return t.title, nil
}

// FetchChapters retrieves the list of chapters.
func (t *Toongod) FetchChapters() (Filterables, []error) {
	// 1) Ensure Cloudflare bypass done only once.
	var cfErr error
	t.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(t.URL, 5000)
	})
	if cfErr != nil {
		return nil, []error{fmt.Errorf("cloudflare bypass failed: %w", cfErr)}
	}

	// 2) Headless extract
	remoteDebugURL := os.Getenv("REMOTE_DEBUG_URL")
	if remoteDebugURL == "" {
		remoteDebugURL = "http://localhost:6082"
	}
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), remoteDebugURL)
	defer allocCancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

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
		chromedp.Sleep(2*time.Second),
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
		out = append(out, &ToongodChapter{
			Chapter: Chapter{Title: c.Title, Number: c.Number},
			URL:     c.URL,
		})
	}
	logger.Debug("Toongod.FetchChapters → %d chapters", len(out))
	return out, nil
}

// FetchChapter downloads one chapter by retrieving raw image URLs
// and pushing them into FastAPI's /save_image, then collecting the saved filenames.
func (t *Toongod) FetchChapter(f Filterable) (*Chapter, error) {
	tc, ok := f.(*ToongodChapter)
	if !ok {
		return nil, fmt.Errorf("invalid chapter type")
	}
	logger.Debug("Toongod.FetchChapter: %s", tc.URL)

	// 1) (Bypass already done once; skip per-chapter trigger to avoid loops.)

	// 2) Extract raw image URLs headlessly
	remoteDebugURL := os.Getenv("REMOTE_DEBUG_URL")
	if remoteDebugURL == "" {
		remoteDebugURL = "http://localhost:6082"
	}
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), remoteDebugURL)
	defer allocCancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var imagesJSON string
	jsImg := `(function(){
		var imgs = document.querySelectorAll("div.reading-content img.wp-manga-chapter-img");
		var srcs = [];
		for(var i = 0; i < imgs.length; i++){
			var src = imgs[i].getAttribute("src") || imgs[i].getAttribute("data-src");
			if(src && src.trim() !== ""){
				srcs.push(src.trim());
			}
		}
		return JSON.stringify(srcs);
	})();`

	if err := chromedp.Run(ctx,
		chromedp.Navigate(tc.URL),
		chromedp.WaitVisible("div.reading-content", chromedp.ByQuery),
		chromedp.Evaluate(jsImg, &imagesJSON),
	); err != nil {
		return nil, fmt.Errorf("chromedp images extraction failed: %w", err)
	}

	var rawURLs []string
	if err := json.Unmarshal([]byte(imagesJSON), &rawURLs); err != nil {
		return nil, fmt.Errorf("invalid images JSON: %w", err)
	}

	// 3) Submit each URL to FastAPI
	for i, img := range rawURLs {
		logger.Debug("Toongod.SaveImage [%d/%d]: %s", i+1, len(rawURLs), img)
		if err := cloudflare.SaveImage(tc.URL, img); err != nil {
			logger.Error("SaveImage failed: %v", err)
		}
		// slight pause to avoid overwhelming the API
		time.Sleep(100 * time.Millisecond)
	}

	// 4) Retrieve saved filenames
	fnames, err := cloudflare.GetSavedImages(tc.URL)
	if err != nil {
		return nil, fmt.Errorf("GetSavedImages failed: %w", err)
	}

	// 5) Build Chapter.Pages with FastAPI URLs
	ch := &Chapter{
		Title:      tc.Title,
		Number:     tc.Number,
		PagesCount: int64(len(fnames)),
		Language:   "en",
	}
	for idx, fn := range fnames {
		chap := path.Base(tc.URL)
		getURL := fmt.Sprintf("%s/get_image?chapter=%s&filename=%s",
			cloudflare.FASTAPIBaseURL,
			url.PathEscape(chap),
			url.QueryEscape(fn),
		)
		ch.Pages = append(ch.Pages, Page{Number: int64(idx + 1), URL: getURL})
	}
	logger.Debug("Toongod.FetchChapter → %d pages via FastAPI", len(ch.Pages))
	return ch, nil
}

// BaseUrl returns the site’s origin.
func (t *Toongod) BaseUrl() string {
	u, _ := url.Parse(t.URL)
	return u.Scheme + "://" + u.Host
}
