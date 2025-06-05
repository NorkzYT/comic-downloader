package grabber

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NorkzYT/comic-downloader/internal/cloudflare"
	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/chromedp/chromedp"
)

// Zonatmo is a grabber implementation for zonatmo.com.
type Zonatmo struct {
	*Grabber
	title       string
	bypassOnce  sync.Once
	allocOnce   sync.Once
	allocCtx    context.Context
	allocCancel context.CancelFunc
}

// ZonatmoChapter represents a chapter on Zonatmo.
type ZonatmoChapter struct {
	Chapter
	URL string
}

// resolveChapterURL clicks the play button link to obtain the actual
// reading URL. Zonatmo returns a 404 if the link is accessed directly,
// so we must navigate via the series page and trigger the click event
// that performs the redirect.
func (z *Zonatmo) resolveChapterURL(btnURL string) (string, error) {
	// ensure Cloudflare bypass has been triggered
	var cfErr error
	z.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(z.URL, 1000)
	})
	if cfErr != nil {
		return "", fmt.Errorf("cloudflare bypass failed: %w", cfErr)
	}

	z.initAllocator()

	tCtx, tabCancel := chromedp.NewContext(z.allocCtx)
	defer tabCancel()
	ctx, cancel := context.WithTimeout(tCtx, 60*time.Second)
	defer cancel()

	selector := fmt.Sprintf("a[href='%s']", btnURL)
	jsClick := fmt.Sprintf(`var a=document.querySelector(%q); if(a) a.click();`, selector)
	var final string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(z.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(jsClick, nil),
		chromedp.WaitVisible("div.img-container img", chromedp.ByQuery),
		chromedp.Location(&final),
	); err != nil {
		return "", fmt.Errorf("chromedp resolve chapter url failed: %w", err)
	}

	return final, nil
}

// initAllocator sets up a shared ChromeDP allocator (browser instance) once.
func (z *Zonatmo) initAllocator() {
	z.allocOnce.Do(func() {
		remoteDebug := os.Getenv("REMOTE_DEBUG_URL")
		if remoteDebug == "" {
			remoteDebug = "http://localhost:6082"
		}
		allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), remoteDebug)
		z.allocCtx = allocCtx
		z.allocCancel = allocCancel
	})
}

// Test checks if the current URL belongs to zonatmo.com.
func (z *Zonatmo) Test() (bool, error) {
	logger.Debug("Zonatmo.Test: Checking URL: %s", z.URL)
	return strings.Contains(z.URL, "zonatmo.com"), nil
}

func (z *Zonatmo) UsesBrowser() bool { return true }

// FetchTitle retrieves (and caches) the comic title.
func (z *Zonatmo) FetchTitle() (string, error) {
	if z.title != "" {
		return z.title, nil
	}

	// Bypass Cloudflare once
	var cfErr error
	z.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(z.URL, 1000)
	})
	if cfErr != nil {
		return "", fmt.Errorf("cloudflare bypass failed: %w", cfErr)
	}

	z.initAllocator()

	// New tab context with timeout
	tCtx, tabCancel := chromedp.NewContext(z.allocCtx)
	defer tabCancel()
	ctx, cancel := context.WithTimeout(tCtx, 30*time.Second)
	defer cancel()

	var title string
	jsTitle := `document.querySelector("h2.element-subtitle")?.innerText || ""`
	if err := chromedp.Run(ctx,
		chromedp.Navigate(z.URL),
		chromedp.WaitVisible("h2.element-subtitle", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(jsTitle, &title),
	); err != nil {
		return "", fmt.Errorf("chromedp title extraction failed: %w", err)
	}

	z.title = strings.TrimSpace(title)
	logger.Debug("Zonatmo.FetchTitle → %s", z.title)
	return z.title, nil
}

// FetchChapters retrieves the list of chapters.
func (z *Zonatmo) FetchChapters() (Filterables, []error) {
	var cfErr error
	z.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(z.URL, 1000)
	})
	if cfErr != nil {
		return nil, []error{fmt.Errorf("cloudflare bypass failed: %w", cfErr)}
	}

	z.initAllocator()

	tCtx, tabCancel := chromedp.NewContext(z.allocCtx)
	defer tabCancel()
	ctx, cancel := context.WithTimeout(tCtx, 30*time.Second)
	defer cancel()

	var rawJSON string
	jsChapters := `(
        function(){
            const chapters = [];
            document.querySelectorAll('#chapters li.upload-link').forEach(li=>{
                const header = li.querySelector('a.btn-collapse');
                if(!header) return;
                const titleText = header.innerText.trim();
                let num = parseFloat(li.getAttribute('data-mal-sync-episode')) || 0;
                const m = titleText.match(/Cap\xED?tulo\s*([\d.]+)/i);
                if(m) num = parseFloat(m[1]);
                const link = li.querySelector('a[href*="/view_uploads/"]');
                const href = link ? link.href : '';
                chapters.push({title:titleText, number:num, url:href});
            });
            return JSON.stringify(chapters);
        }
    )();`

	if err := chromedp.Run(ctx,
		chromedp.Navigate(z.URL),
		chromedp.WaitVisible("div.card.chapters", chromedp.ByQuery),
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
		out = append(out, &ZonatmoChapter{
			Chapter: Chapter{Title: c.Title, Number: c.Number},
			URL:     c.URL,
		})
	}
	logger.Debug("Zonatmo.FetchChapters → %d chapters", len(out))
	return out, nil
}

// FetchChapter downloads one chapter by using FastAPI to batch-save images.
func (z *Zonatmo) FetchChapter(f Filterable) (*Chapter, error) {
	sc, ok := f.(*ZonatmoChapter)
	if !ok {
		return nil, fmt.Errorf("invalid chapter type")
	}

	realURL := sc.URL
	if strings.Contains(realURL, "/view_uploads/") {
		var err error
		realURL, err = z.resolveChapterURL(sc.URL)
		if err != nil {
			return nil, err
		}
	}

	jsImg := `(
        function(){
            const srcs = [];
            document.querySelectorAll('div.img-container img').forEach(img=>{
                const s = img.getAttribute('data-src') || img.getAttribute('src');
                if(s && s.trim()) srcs.push(s.trim());
            });
            return JSON.stringify(srcs);
        }
    )();`

	z.initAllocator()
	tCtx, tabCancel := chromedp.NewContext(z.allocCtx)
	defer tabCancel()
	ctx, cancel := context.WithTimeout(tCtx, 60*time.Second)
	defer cancel()

	var raw string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(realURL),
		chromedp.WaitVisible("div.img-container img", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(jsImg, &raw),
	); err != nil {
		return nil, fmt.Errorf("chromedp images extraction failed: %w", err)
	}

	var images []string
	if err := json.Unmarshal([]byte(raw), &images); err != nil {
		return nil, fmt.Errorf("invalid images JSON: %w", err)
	}

	ch := &Chapter{
		Title:    sc.Title,
		Number:   sc.Number,
		Language: "es",
	}
	for i, imgURL := range images {
		ch.Pages = append(ch.Pages, Page{Number: int64(i + 1), URL: imgURL})
	}
	ch.PagesCount = int64(len(ch.Pages))

	return ch, nil
}

// BaseUrl returns the site's origin.
func (z *Zonatmo) BaseUrl() string {
	u, _ := url.Parse(z.URL)
	return u.Scheme + "://" + u.Host
}