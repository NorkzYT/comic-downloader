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

// Mangabuddy is a grabber implementation for www.mangabuddy.com.
type Mangabuddy struct {
	*Grabber
	title       string
	bypassOnce  sync.Once
	allocOnce   sync.Once
	allocCtx    context.Context
	allocCancel context.CancelFunc
}

// MangabuddyChapter represents a single chapter on Mangabuddy.
type MangabuddyChapter struct {
	Chapter
	URL string
}

// initAllocator sets up a shared ChromeDP allocator (browser instance) once.
func (m *Mangabuddy) initAllocator() {
	m.allocOnce.Do(func() {
		remoteDebug := os.Getenv("REMOTE_DEBUG_URL")
		if remoteDebug == "" {
			remoteDebug = "http://localhost:6082"
		}
		allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), remoteDebug)
		m.allocCtx = allocCtx
		m.allocCancel = allocCancel
	})
}

// Test checks if the current URL is for mangabuddy.com.
func (m *Mangabuddy) Test() (bool, error) {
	logger.Debug("Mangabuddy.Test: Checking URL: %s", m.URL)
	return strings.Contains(m.URL, "mangabuddy.com"), nil
}

// UsesBrowser returns true since Mangabuddy pages require a headless browser.
func (m *Mangabuddy) UsesBrowser() bool {
	return true
}

// FetchTitle retrieves (and caches) the comic title.
func (m *Mangabuddy) FetchTitle() (string, error) {
	if m.title != "" {
		return m.title, nil
	}

	// 1) Bypass Cloudflare once.
	var cfErr error
	m.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(m.URL, 1000)
	})
	if cfErr != nil {
		return "", fmt.Errorf("cloudflare bypass failed: %w", cfErr)
	}

	// 2) Initialize shared allocator.
	m.initAllocator()

	// 3) Create a new tab context.
	tCtx, tabCancel := chromedp.NewContext(m.allocCtx)
	defer tabCancel()

	// 4) Add timeout.
	ctx, cancel := context.WithTimeout(tCtx, 30*time.Second)
	defer cancel()

	// 5) Extract title via JS.
	var title string
	jsTitle := `
		(document.querySelector(
		  "body > div.layout > div.main-container.book-details > div > div.row.no-gutters > div.col-70.container__left > div.book-info > div.detail > div.name.box > h1"
		)?.innerText || "").trim();
	`
	if err := chromedp.Run(ctx,
		chromedp.Navigate(m.URL),
		chromedp.WaitVisible("div.name.box h1", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(jsTitle, &title),
	); err != nil {
		return "", fmt.Errorf("chromedp title extraction failed: %w", err)
	}

	m.title = title
	logger.Debug("Mangabuddy.FetchTitle → %s", m.title)
	return m.title, nil
}

// FetchChapters retrieves the list of chapters.
func (m *Mangabuddy) FetchChapters() (Filterables, []error) {
	// 1) Bypass Cloudflare once.
	var cfErr error
	m.bypassOnce.Do(func() {
		cfErr = cloudflare.TriggerCloudflare(m.URL, 1000)
	})
	if cfErr != nil {
		return nil, []error{fmt.Errorf("cloudflare bypass failed: %w", cfErr)}
	}

	// 2) Initialize shared allocator.
	m.initAllocator()

	// 3) New tab context.
	tCtx, tabCancel := chromedp.NewContext(m.allocCtx)
	defer tabCancel()

	// 4) Add timeout.
	ctx, cancel := context.WithTimeout(tCtx, 30*time.Second)
	defer cancel()

	// 5) Extract chapters via JS.
	var rawJSON string
	jsChapters := `
		(function(){
			const chapters = [];
			document.querySelectorAll("#chapter-list li").forEach(li => {
				const a = li.querySelector("a");
				if (!a) return;
				const titleElem = a.querySelector("strong.chapter-title");
				const titleText = titleElem ? titleElem.innerText.trim() : "";
				const num = parseFloat(titleText.replace(/Chapter\s*/i, "")) || 0;
				chapters.push({ title: titleText, number: num, url: a.href });
			});
			return JSON.stringify(chapters);
		})();
	`
	if err := chromedp.Run(ctx,
		chromedp.Navigate(m.URL),
		chromedp.WaitVisible("#chapter-list", chromedp.ByID),
		chromedp.Sleep(800*time.Millisecond),
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
		out = append(out, &MangabuddyChapter{
			Chapter: Chapter{Title: c.Title, Number: c.Number},
			URL:     c.URL,
		})
	}
	logger.Debug("Mangabuddy.FetchChapters → %d chapters", len(out))
	return out, nil
}

// FetchChapter downloads one chapter by using FastAPI to batch-save images.
func (m *Mangabuddy) FetchChapter(f Filterable) (*Chapter, error) {
	mbc, ok := f.(*MangabuddyChapter)
	if !ok {
		return nil, fmt.Errorf("invalid chapter type")
	}

	// 1) Derive series slug from title.
	fullTitle, err := m.FetchTitle()
	if err != nil {
		return nil, fmt.Errorf("could not fetch title: %w", err)
	}
	slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(fullTitle), " ", "-"))

	// 2) JavaScript extractor for images.
	jsImg := `
		(function(){
			const srcs = [];
			document.querySelectorAll("#chapter-images img").forEach(img => {
				const s = img.getAttribute("src") || img.getAttribute("data-src");
				if (s && s.trim()) srcs.push(s.trim());
			});
			return JSON.stringify(srcs);
		})();
	`

	// 3) Ask Tenshi (FastAPI) to save whole chapter images.
	if err := cloudflare.SaveChapter(mbc.URL, jsImg, slug); err != nil {
		logger.Error("SaveChapter failed: %v", err)
	}

	// 4) Fetch back the filenames.
	fnames, err := cloudflare.GetSavedImages(mbc.URL, slug)
	if err != nil {
		return nil, fmt.Errorf("GetSavedImages failed: %w", err)
	}

	// 5) Build our Chapter model.
	ch := &Chapter{
		Title:      mbc.Title,
		Number:     mbc.Number,
		PagesCount: int64(len(fnames)),
		Language:   "en",
	}
	for idx, fn := range fnames {
		chapFolder := path.Base(mbc.URL)
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
func (m *Mangabuddy) BaseUrl() string {
	u, _ := url.Parse(m.URL)
	return u.Scheme + "://" + u.Host
}
